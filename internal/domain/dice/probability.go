package dice

import (
	"fmt"
	"math"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

// MaxDistributionWork bounds an exact distribution.
//
// Distribute convolves one die at a time, so the work grows with the number of
// reachable totals times the die size. Enumerating 100d1000 is not a slow
// request, it is a hung one, and a dice calculator that stalls the API is worse
// than one that says no. Everything a table actually rolls -- 20d20, 8d12,
// 10d100 -- is far inside this.
const MaxDistributionWork = 5000

// Outcome is one total an expression can produce, and how likely it is.
//
// The cumulative fields are carried alongside because they are what gets asked
// at the table: not "how likely is exactly 14" but "how likely is 14 or more".
type Outcome struct {
	Total       int     `json:"total"`
	Probability float64 `json:"probability"`
	AtLeast     float64 `json:"at_least"`
	AtMost      float64 `json:"at_most"`
}

// Distribution is the exact probability of every total an expression can roll.
//
// It is computed, not sampled: a Monte Carlo estimate of 3d6 would be wrong in
// the fourth decimal place for no reason, since the real answer is a short
// convolution away.
type Distribution struct {
	Expression Expression `json:"expression"`
	Min        int        `json:"min"`
	Max        int        `json:"max"`
	Mean       float64    `json:"mean"`
	StdDev     float64    `json:"std_dev"`
	Outcomes   []Outcome  `json:"outcomes"`
}

// AtLeast is the probability of rolling the given total or better.
func (d Distribution) AtLeast(total int) float64 {
	switch {
	case total <= d.Min:
		return 1
	case total > d.Max:
		return 0
	}
	for _, o := range d.Outcomes {
		if o.Total == total {
			return o.AtLeast
		}
	}
	return 0
}

// AtMost is the probability of rolling the given total or worse.
func (d Distribution) AtMost(total int) float64 {
	switch {
	case total >= d.Max:
		return 1
	case total < d.Min:
		return 0
	}
	for _, o := range d.Outcomes {
		if o.Total == total {
			return o.AtMost
		}
	}
	return 0
}

// ExpectedValue is the true mean of the expression.
//
// Average rounds down because that is the number a statblock prints; this does
// not, because rounding here would compound through an expected-damage sum.
func (e Expression) ExpectedValue() float64 {
	return float64(e.Count)*(float64(e.Sides)+1)/2 + float64(e.Modifier)
}

// Variance is the spread of the expression, for which only the dice count:
// a flat modifier moves the curve without widening it.
func (e Expression) Variance() float64 {
	if e.Count == 0 || e.Sides < 2 {
		return 0
	}
	return float64(e.Count) * (float64(e.Sides)*float64(e.Sides) - 1) / 12
}

// Distribute computes the exact probability of every total.
func Distribute(e Expression) (Distribution, error) {
	switch {
	case e.Count < 0:
		return Distribution{}, fmt.Errorf("dice count %d is negative", e.Count)
	case e.Count > 0 && e.Sides < 2:
		return Distribution{}, fmt.Errorf("a d%d cannot be rolled", e.Sides)
	case e.Count*e.Sides > MaxDistributionWork:
		return Distribution{}, fmt.Errorf(
			"%s has too many combinations to enumerate exactly (limit is %d dice-faces)",
			e, MaxDistributionWork)
	}

	// probabilities[i] is the chance the dice so far sum to base+i, where base
	// is one per die rolled: the lowest sum reachable.
	probabilities := []float64{1}
	perFace := 0.0
	if e.Sides > 0 {
		perFace = 1 / float64(e.Sides)
	}
	for i := 0; i < e.Count; i++ {
		next := make([]float64, len(probabilities)+e.Sides-1)
		for sum, p := range probabilities {
			if p == 0 {
				continue
			}
			for face := 1; face <= e.Sides; face++ {
				next[sum+face-1] += p * perFace
			}
		}
		probabilities = next
	}

	d := Distribution{
		Expression: e,
		Min:        e.Min(),
		Max:        e.Max(),
		Mean:       e.ExpectedValue(),
		StdDev:     math.Sqrt(e.Variance()),
		Outcomes:   make([]Outcome, len(probabilities)),
	}

	// One pass up accumulates AtMost, one pass down accumulates AtLeast.
	// Summing from each end rather than subtracting from one keeps the two
	// tails from disagreeing by an accumulated epsilon.
	var running float64
	for i, p := range probabilities {
		running += p
		d.Outcomes[i] = Outcome{
			Total:       e.Count + i + e.Modifier,
			Probability: p,
			AtMost:      running,
		}
	}
	running = 0
	for i := len(probabilities) - 1; i >= 0; i-- {
		running += probabilities[i]
		d.Outcomes[i].AtLeast = running
	}
	return d, nil
}

// faceProbabilities is the chance of each natural d20 face being the one kept.
//
// Advantage and disadvantage are not a modifier on the total: they reshape
// which face survives. Deriving both attack and check odds from this one table
// is what keeps the two endpoints from disagreeing about the same d20.
func faceProbabilities(mode models.RollMode) [D20Sides + 1]float64 {
	var p [D20Sides + 1]float64
	for face := 1; face <= D20Sides; face++ {
		switch mode {
		case models.RollAdvantage:
			// P(max of two = f) = (f^2 - (f-1)^2) / 400.
			p[face] = float64(2*face-1) / 400
		case models.RollDisadvantage:
			// P(min of two = f) = ((21-f)^2 - (20-f)^2) / 400.
			p[face] = float64(41-2*face) / 400
		default:
			p[face] = 1.0 / D20Sides
		}
	}
	return p
}

// CheckOdds is the chance of passing a d20 check against a difficulty class.
type CheckOdds struct {
	DC       int             `json:"dc"`
	Modifier int             `json:"modifier"`
	Mode     models.RollMode `json:"mode"`

	// NeedsNatural is the lowest die face that succeeds, clamped to 1-21.
	// A 1 means the modifier alone carries it; a 21 means no face can.
	NeedsNatural int     `json:"needs_natural"`
	Success      float64 `json:"success"`
	Failure      float64 `json:"failure"`
}

// OddsOfCheck computes the chance of meeting a DC.
//
// Ability checks have no automatic success or failure in RAW -- that is a house
// rule -- so a large enough modifier genuinely makes the roll a formality, and
// ResolveCheck agrees.
func OddsOfCheck(dc, modifier int, mode models.RollMode) CheckOdds {
	needed := dc - modifier
	switch {
	case needed < 1:
		needed = 1
	case needed > D20Sides:
		needed = D20Sides + 1
	}

	odds := CheckOdds{DC: dc, Modifier: modifier, Mode: mode, NeedsNatural: needed}
	faces := faceProbabilities(mode)
	for face := needed; face <= D20Sides; face++ {
		odds.Success += faces[face]
	}
	odds.Failure = 1 - odds.Success
	return odds
}

// AttackOdds is the chance of connecting with an attack, and what it costs the
// target on average.
type AttackOdds struct {
	TargetAC  int             `json:"target_ac"`
	Modifier  int             `json:"modifier"`
	CritRange int             `json:"crit_range"`
	Mode      models.RollMode `json:"mode"`
	Damage    string          `json:"damage"`

	Hit         float64 `json:"hit"`
	OrdinaryHit float64 `json:"ordinary_hit"`
	Critical    float64 `json:"critical"`
	Miss        float64 `json:"miss"`
	Fumble      float64 `json:"fumble"`

	AverageHit      float64 `json:"average_hit"`
	AverageCritical float64 `json:"average_critical"`

	// ExpectedDamage is per attack, counting misses as zero -- the number that
	// answers how long a fight lasts.
	ExpectedDamage float64 `json:"expected_damage"`
}

// OddsOfAttack computes attack chances for a normal roll.
func OddsOfAttack(targetAC, modifier, critRange int, damage string) (AttackOdds, error) {
	return OddsOfAttackWithMode(targetAC, modifier, critRange, damage, models.RollNormal)
}

// OddsOfAttackWithMode computes attack chances, honouring advantage.
//
// Attacks differ from checks at both ends of the die: a natural 1 always misses
// and a face in the critical range always hits, whatever the modifiers say.
func OddsOfAttackWithMode(targetAC, modifier, critRange int, damage string, mode models.RollMode) (AttackOdds, error) {
	if critRange < 2 || critRange > D20Sides {
		return AttackOdds{}, fmt.Errorf("crit range %d is outside 2-%d", critRange, D20Sides)
	}
	e, err := Parse(damage)
	if err != nil {
		return AttackOdds{}, err
	}

	odds := AttackOdds{
		TargetAC: targetAC, Modifier: modifier, CritRange: critRange,
		Mode: mode, Damage: damage,
		AverageHit:      e.ExpectedValue(),
		AverageCritical: e.Doubled().ExpectedValue(),
	}

	faces := faceProbabilities(mode)
	for face := 1; face <= D20Sides; face++ {
		switch {
		case face == 1:
			// A natural 1 misses even a target it would otherwise beat, and
			// it is checked first so it can never be read as a critical.
			odds.Fumble += faces[face]
		case face >= critRange:
			odds.Critical += faces[face]
		case face+modifier >= targetAC:
			odds.OrdinaryHit += faces[face]
		}
	}

	odds.Hit = odds.OrdinaryHit + odds.Critical
	odds.Miss = 1 - odds.Hit
	odds.ExpectedDamage = odds.OrdinaryHit*odds.AverageHit + odds.Critical*odds.AverageCritical
	return odds, nil
}
