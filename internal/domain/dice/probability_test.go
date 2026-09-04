package dice

import (
	"math"
	"strconv"
	"testing"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

func closeTo(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %.10f, want %.10f", label, got, want)
	}
}

func distribute(t *testing.T, expression string) Distribution {
	t.Helper()
	e, err := Parse(expression)
	if err != nil {
		t.Fatalf("Parse(%q): %v", expression, err)
	}
	d, err := Distribute(e)
	if err != nil {
		t.Fatalf("Distribute(%q): %v", expression, err)
	}
	return d
}

// A single die is flat, which is the base case every convolution builds on.
func TestDistributionOfOneDieIsUniform(t *testing.T) {
	d := distribute(t, "1d6")

	if len(d.Outcomes) != 6 {
		t.Fatalf("1d6 has %d outcomes, want 6", len(d.Outcomes))
	}
	for i, o := range d.Outcomes {
		if o.Total != i+1 {
			t.Errorf("outcome %d is total %d, want %d", i, o.Total, i+1)
		}
		closeTo(t, "P(1d6)", o.Probability, 1.0/6.0)
	}
	closeTo(t, "mean", d.Mean, 3.5)
}

// 2d6 is the textbook triangle: 7 is six times likelier than 2.
func TestDistributionOfTwoDiceIsTriangular(t *testing.T) {
	d := distribute(t, "2d6")

	want := map[int]float64{
		2: 1.0 / 36, 3: 2.0 / 36, 4: 3.0 / 36, 5: 4.0 / 36, 6: 5.0 / 36,
		7: 6.0 / 36, 8: 5.0 / 36, 9: 4.0 / 36, 10: 3.0 / 36, 11: 2.0 / 36, 12: 1.0 / 36,
	}
	if len(d.Outcomes) != len(want) {
		t.Fatalf("2d6 has %d outcomes, want %d", len(d.Outcomes), len(want))
	}
	for _, o := range d.Outcomes {
		closeTo(t, "P(2d6 total)", o.Probability, want[o.Total])
	}
	closeTo(t, "mean", d.Mean, 7)
	closeTo(t, "stddev", d.StdDev, math.Sqrt(2*(36-1)/12.0))
}

// The modifier shifts the whole curve and changes nothing about its shape.
func TestModifierShiftsTheDistribution(t *testing.T) {
	plain, shifted := distribute(t, "2d6"), distribute(t, "2d6+3")

	if shifted.Min != 5 || shifted.Max != 15 {
		t.Errorf("2d6+3 spans %d-%d, want 5-15", shifted.Min, shifted.Max)
	}
	closeTo(t, "mean", shifted.Mean, 10)
	closeTo(t, "stddev", shifted.StdDev, plain.StdDev)

	for i := range plain.Outcomes {
		if shifted.Outcomes[i].Total != plain.Outcomes[i].Total+3 {
			t.Fatalf("outcome %d is %d, want %d", i, shifted.Outcomes[i].Total, plain.Outcomes[i].Total+3)
		}
		closeTo(t, "P shifted", shifted.Outcomes[i].Probability, plain.Outcomes[i].Probability)
	}
}

// AtLeast is what a player actually asks: "what are my chances of beating 15?"
func TestCumulativeProbabilities(t *testing.T) {
	d := distribute(t, "3d6")

	var sum float64
	for _, o := range d.Outcomes {
		sum += o.Probability
	}
	closeTo(t, "total probability", sum, 1)

	first, last := d.Outcomes[0], d.Outcomes[len(d.Outcomes)-1]
	closeTo(t, "AtLeast(min)", first.AtLeast, 1)
	closeTo(t, "AtMost(max)", last.AtMost, 1)
	closeTo(t, "AtLeast(max)", last.AtLeast, 1.0/216)
	closeTo(t, "AtMost(min)", first.AtMost, 1.0/216)

	previous := 1.0
	for _, o := range d.Outcomes {
		if o.AtLeast > previous+1e-12 {
			t.Fatalf("AtLeast rose at total %d: %.6f after %.6f", o.Total, o.AtLeast, previous)
		}
		previous = o.AtLeast
	}

	// 3d6 is symmetric about 10.5, so half of 216 outcomes beat 10, and the
	// 27 ways to roll exactly 10 push it to 135/216.
	closeTo(t, "P(3d6 >= 10)", d.AtLeast(10), 135.0/216)
	closeTo(t, "P(3d6 >= 11)", d.AtLeast(11), 0.5)
	closeTo(t, "P(3d6 >= 3)", d.AtLeast(3), 1)
	closeTo(t, "P(3d6 >= 19)", d.AtLeast(19), 0)
}

// Parse turns a bare number into a constant, and a constant has one outcome.
func TestDistributionOfAConstant(t *testing.T) {
	d := distribute(t, "4")

	if len(d.Outcomes) != 1 || d.Outcomes[0].Total != 4 {
		t.Fatalf("constant distribution = %+v, want a single total of 4", d.Outcomes)
	}
	closeTo(t, "P(4)", d.Outcomes[0].Probability, 1)
	closeTo(t, "stddev", d.StdDev, 0)
}

// The convolution is exact, not sampled, so it has to refuse work it cannot do
// quickly rather than hang a request.
func TestDistributionRefusesAnImpracticalExpression(t *testing.T) {
	if _, err := Distribute(Expression{Count: 100, Sides: 1000}); err == nil {
		t.Error("100d1000 should be refused as too large to enumerate")
	}
	if _, err := Distribute(Expression{Count: 20, Sides: 20}); err != nil {
		t.Errorf("20d20 is well within reach: %v", err)
	}
}

// A d20 check is the single most common question at the table, and advantage
// changes the answer more than most players expect.
func TestCheckOddsAcrossModes(t *testing.T) {
	// DC 15 with +5 needs a natural 10 or better: eleven faces in twenty.
	normal := OddsOfCheck(15, 5, models.RollNormal)
	if normal.NeedsNatural != 10 {
		t.Errorf("needs natural %d, want 10", normal.NeedsNatural)
	}
	closeTo(t, "P(normal)", normal.Success, 11.0/20)
	closeTo(t, "P(fail)", normal.Failure, 9.0/20)

	// Advantage is the chance of not failing twice.
	closeTo(t, "P(advantage)", OddsOfCheck(15, 5, models.RollAdvantage).Success, 1-math.Pow(9.0/20, 2))
	closeTo(t, "P(disadvantage)", OddsOfCheck(15, 5, models.RollDisadvantage).Success, math.Pow(11.0/20, 2))
}

// An ability check has no automatic success or failure in RAW, so a large
// enough modifier really does make the roll a formality.
func TestCheckOddsAtTheExtremes(t *testing.T) {
	certain := OddsOfCheck(5, 20, models.RollNormal)
	closeTo(t, "P(certain)", certain.Success, 1)
	if certain.NeedsNatural > 1 {
		t.Errorf("needs natural %d, want 1 or less", certain.NeedsNatural)
	}

	hopeless := OddsOfCheck(40, 0, models.RollNormal)
	closeTo(t, "P(hopeless)", hopeless.Success, 0)
	if hopeless.NeedsNatural <= 20 {
		t.Errorf("needs natural %d, want an unreachable face", hopeless.NeedsNatural)
	}
}

func attackOdds(t *testing.T, ac, modifier, critRange int, damage string) AttackOdds {
	t.Helper()
	odds, err := OddsOfAttack(ac, modifier, critRange, damage)
	if err != nil {
		t.Fatalf("OddsOfAttack: %v", err)
	}
	return odds
}

// Attacks differ from checks in both directions: a natural 20 always hits and
// a natural 1 always misses, whatever the numbers say.
func TestAttackOddsHonourTheNaturalFaces(t *testing.T) {
	// AC 40 is unreachable, so only the automatic hit connects.
	only20 := attackOdds(t, 40, 0, 20, "1d8+3")
	closeTo(t, "P(hit)", only20.Hit, 1.0/20)
	closeTo(t, "P(crit)", only20.Critical, 1.0/20)
	closeTo(t, "P(ordinary hit)", only20.OrdinaryHit, 0)

	// AC 1 is beaten by everything, but a natural 1 still misses.
	always := attackOdds(t, 1, 0, 20, "1d8+3")
	closeTo(t, "P(hit)", always.Hit, 19.0/20)
	closeTo(t, "P(fumble)", always.Fumble, 1.0/20)
	closeTo(t, "P(miss)", always.Miss, 1.0/20)
}

// A Champion's widened crit range is the whole of that archetype, so the
// calculator has to show it.
func TestAttackOddsWidenedCritRange(t *testing.T) {
	ordinary := attackOdds(t, 15, 5, 20, "1d8+3")
	closeTo(t, "P(crit)", ordinary.Critical, 1.0/20)

	champion := attackOdds(t, 15, 5, 19, "1d8+3")
	closeTo(t, "P(crit)", champion.Critical, 2.0/20)
	if champion.ExpectedDamage <= ordinary.ExpectedDamage {
		t.Errorf("a wider crit range did not raise expected damage: %.4f vs %.4f",
			champion.ExpectedDamage, ordinary.ExpectedDamage)
	}

	// Hit chance is unchanged: a 19 already hit AC 15, it now hits harder.
	closeTo(t, "P(hit)", champion.Hit, ordinary.Hit)
}

// Expected damage is what makes the endpoint worth calling: it answers "how
// long does this fight last", which is the question behind encounter balance.
func TestExpectedDamageDoublesOnlyTheDice(t *testing.T) {
	// Only a natural 20 lands, so expected damage is one twentieth of a
	// critical: 2d8 averages 9, plus the modifier once, is 12.
	odds := attackOdds(t, 40, 0, 20, "1d8+3")
	closeTo(t, "expected damage", odds.ExpectedDamage, 12.0/20)

	// Everything but a natural 1 lands, one twentieth of it critically.
	all := attackOdds(t, 1, 0, 20, "1d8+3")
	closeTo(t, "expected damage", all.ExpectedDamage, (18*7.5+12)/20)
}

func TestAttackOddsRejectBadDamage(t *testing.T) {
	if _, err := OddsOfAttack(15, 5, 20, "sword"); err == nil {
		t.Error("an unparseable damage expression should be an error")
	}
	if _, err := OddsOfAttack(15, 5, 25, "1d8"); err == nil {
		t.Error("a crit range above 20 can never trigger and should be rejected")
	}
}

// Advantage on an attack has to use the same face weights as a check, or the
// two endpoints would disagree about the same d20.
func TestAttackOddsUnderAdvantageMatchTheCheck(t *testing.T) {
	odds, err := OddsOfAttackWithMode(15, 5, 20, "1d8+3", models.RollAdvantage)
	if err != nil {
		t.Fatalf("OddsOfAttackWithMode: %v", err)
	}
	closeTo(t, "P(hit)", odds.Hit, OddsOfCheck(15, 5, models.RollAdvantage).Success)
	closeTo(t, "P(crit)", odds.Critical, 1-math.Pow(19.0/20, 2))
}

// The convolution is clever enough to be wrong quietly, so it is checked
// against the dumbest possible implementation: enumerate every combination.
func TestDistributionMatchesBruteForce(t *testing.T) {
	for _, e := range []Expression{
		{Count: 1, Sides: 20},
		{Count: 2, Sides: 6},
		{Count: 3, Sides: 6, Modifier: 2},
		{Count: 4, Sides: 4, Modifier: -1},
		{Count: 2, Sides: 10, Modifier: 5},
	} {
		counts := map[int]int{}
		total := 0
		var enumerate func(dice, sum int)
		enumerate = func(dice, sum int) {
			if dice == 0 {
				counts[sum+e.Modifier]++
				total++
				return
			}
			for face := 1; face <= e.Sides; face++ {
				enumerate(dice-1, sum+face)
			}
		}
		enumerate(e.Count, 0)

		d, err := Distribute(e)
		if err != nil {
			t.Fatalf("Distribute(%s): %v", e, err)
		}
		if len(d.Outcomes) != len(counts) {
			t.Errorf("%s has %d outcomes, brute force found %d", e, len(d.Outcomes), len(counts))
		}
		for _, o := range d.Outcomes {
			closeTo(t, e.String()+" P("+strconv.Itoa(o.Total)+")", o.Probability, float64(counts[o.Total])/float64(total))
		}

		// The closed-form mean has to agree with the enumerated one too.
		var mean float64
		for value, n := range counts {
			mean += float64(value) * float64(n) / float64(total)
		}
		closeTo(t, e.String()+" mean", d.Mean, mean)
	}
}
