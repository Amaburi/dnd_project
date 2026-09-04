package dice

import (
	"strings"
	"testing"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

func TestParse(t *testing.T) {
	cases := []struct {
		text                   string
		count, sides, modifier int
	}{
		{"1d8+3", 1, 8, 3},
		{"2d6", 2, 6, 0},
		{"d20", 1, 20, 0},
		{"4d6-1", 4, 6, -1},
		{"1d8 + 3", 1, 8, 3},
		{"2D10+5", 2, 10, 5},
		{"8d10+40", 8, 10, 40},
	}

	for _, tc := range cases {
		got, err := Parse(tc.text)
		if err != nil {
			t.Errorf("Parse(%q): %v", tc.text, err)
			continue
		}
		if got.Count != tc.count || got.Sides != tc.sides || got.Modifier != tc.modifier {
			t.Errorf("Parse(%q) = %+v, want %dd%d%+d", tc.text, got, tc.count, tc.sides, tc.modifier)
		}
	}

	// A bare number is a constant, which unarmed strike damage uses.
	flat, err := Parse("1")
	if err != nil {
		t.Fatalf("Parse(\"1\"): %v", err)
	}
	if flat.Count != 0 || flat.Modifier != 1 {
		t.Errorf("Parse(\"1\") = %+v, want a flat 1", flat)
	}
}

func TestParseRejectsBadInput(t *testing.T) {
	for _, bad := range []string{"", "d", "2x6", "abc", "0d6", "d1", "999d6", "2d99999", "-3d6"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) should have failed", bad)
		}
	}
}

func TestExpressionBounds(t *testing.T) {
	e, _ := Parse("2d6+3")
	if e.Min() != 5 {
		t.Errorf("Min = %d, want 5", e.Min())
	}
	if e.Max() != 15 {
		t.Errorf("Max = %d, want 15", e.Max())
	}
	if e.Average() != 10 {
		t.Errorf("Average = %d, want 10", e.Average())
	}
	if got := e.String(); got != "2d6+3" {
		t.Errorf("String = %q, want 2d6+3", got)
	}

	// A critical doubles the dice but not the modifier.
	if got := e.Doubled(); got.Count != 4 || got.Modifier != 3 {
		t.Errorf("Doubled = %+v, want 4d6+3", got)
	}
}

// A seeded roller repeats exactly, which is what makes the rules testable
// rather than merely statistical.
func TestSeededRollerIsDeterministic(t *testing.T) {
	a, b := NewSeeded(42), NewSeeded(42)

	for i := 0; i < 50; i++ {
		ra, _ := a.Roll("3d8+2")
		rb, _ := b.Roll("3d8+2")
		if ra.Total != rb.Total {
			t.Fatalf("roll %d differs: %d vs %d", i, ra.Total, rb.Total)
		}
	}
}

func TestRollStaysWithinBounds(t *testing.T) {
	r := NewSeeded(1)
	e, _ := Parse("3d6+2")

	for i := 0; i < 500; i++ {
		result := r.RollExpression(e)
		if len(result.Rolls) != 3 {
			t.Fatalf("rolled %d dice, want 3", len(result.Rolls))
		}
		for _, roll := range result.Rolls {
			if roll < 1 || roll > 6 {
				t.Fatalf("a d6 came up %d", roll)
			}
		}
		if result.Total < e.Min() || result.Total > e.Max() {
			t.Fatalf("total %d outside %d-%d", result.Total, e.Min(), e.Max())
		}
	}
}

// Every face of the die should appear over enough rolls; a roller stuck on one
// value would still pass a bounds check.
func TestRollCoversEveryFace(t *testing.T) {
	r := NewSeeded(7)
	seen := map[int]bool{}
	for i := 0; i < 2000; i++ {
		result, _ := r.Roll("1d20")
		seen[result.Rolls[0]] = true
	}
	for face := 1; face <= 20; face++ {
		if !seen[face] {
			t.Errorf("a d20 never rolled %d in 2000 attempts", face)
		}
	}
}

func TestD20AdvantageKeepsTheHigher(t *testing.T) {
	r := NewSeeded(99)

	for i := 0; i < 300; i++ {
		result := r.D20(3, models.RollAdvantage)
		if len(result.Rolls) != 2 {
			t.Fatalf("advantage rolled %d dice, want 2", len(result.Rolls))
		}
		want := result.Rolls[0]
		if result.Rolls[1] > want {
			want = result.Rolls[1]
		}
		if result.Natural != want {
			t.Fatalf("advantage kept %d from %v, want %d", result.Natural, result.Rolls, want)
		}
		if result.Total != result.Natural+3 {
			t.Fatalf("total = %d, want natural %d plus 3", result.Total, result.Natural)
		}
	}
}

func TestD20DisadvantageKeepsTheLower(t *testing.T) {
	r := NewSeeded(123)

	for i := 0; i < 300; i++ {
		result := r.D20(0, models.RollDisadvantage)
		want := result.Rolls[0]
		if result.Rolls[1] < want {
			want = result.Rolls[1]
		}
		if result.Natural != want {
			t.Fatalf("disadvantage kept %d from %v, want %d", result.Natural, result.Rolls, want)
		}
	}
}

func TestD20NormalRollsOneDie(t *testing.T) {
	r := NewSeeded(5)
	result := r.D20(2, models.RollNormal)

	if len(result.Rolls) != 1 {
		t.Errorf("a normal roll used %d dice, want 1", len(result.Rolls))
	}
	if result.Natural != result.Rolls[0] {
		t.Errorf("natural = %d, want the only roll %d", result.Natural, result.Rolls[0])
	}
}

// Advantage should beat a normal roll on average; this is the one place a
// statistical check is the right test.
func TestAdvantageBeatsNormalOnAverage(t *testing.T) {
	r := NewSeeded(2024)
	const trials = 5000

	var normal, advantage int
	for i := 0; i < trials; i++ {
		normal += r.D20(0, models.RollNormal).Natural
		advantage += r.D20(0, models.RollAdvantage).Natural
	}

	normalAvg := float64(normal) / trials
	advAvg := float64(advantage) / trials

	// A d20 averages 10.5; with advantage it averages about 13.8.
	if normalAvg < 9.5 || normalAvg > 11.5 {
		t.Errorf("normal d20 averaged %.2f, want about 10.5", normalAvg)
	}
	if advAvg < 13 || advAvg > 14.5 {
		t.Errorf("advantage averaged %.2f, want about 13.8", advAvg)
	}
}

// A critical doubles the dice and adds the modifier once.
func TestRollDamageCritical(t *testing.T) {
	r := NewSeeded(31)

	for i := 0; i < 200; i++ {
		normal, err := r.RollDamage("2d6+4", false)
		if err != nil {
			t.Fatalf("RollDamage: %v", err)
		}
		if len(normal.Rolls) != 2 {
			t.Fatalf("normal damage rolled %d dice, want 2", len(normal.Rolls))
		}
		if normal.Total < 6 || normal.Total > 16 {
			t.Fatalf("normal total %d outside 6-16", normal.Total)
		}

		crit, _ := r.RollDamage("2d6+4", true)
		if len(crit.Rolls) != 4 {
			t.Fatalf("critical damage rolled %d dice, want 4", len(crit.Rolls))
		}
		// 4d6+4, not 2*(2d6+4): the modifier is added once.
		if crit.Total < 8 || crit.Total > 28 {
			t.Fatalf("critical total %d outside 8-28", crit.Total)
		}
	}
}

func TestRollDamageNeverGoesNegative(t *testing.T) {
	r := NewSeeded(3)
	result, err := r.RollDamage("1d4-10", false)
	if err != nil {
		t.Fatalf("RollDamage: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("total = %d, want 0 (damage never heals)", result.Total)
	}
}

func TestDeathSave(t *testing.T) {
	r := NewSeeded(11)

	// A natural 20 restores a hit point and clears the tally.
	saves := models.DeathSaves{Successes: 1, Failures: 2}
	for i := 0; i < 500; i++ {
		before := saves
		roll, regained := r.DeathSave(&saves)

		switch {
		case roll.IsNatural20():
			if !regained {
				t.Fatal("a natural 20 should restore a hit point")
			}
			if saves != (models.DeathSaves{}) {
				t.Fatalf("a natural 20 left saves at %+v, want cleared", saves)
			}
		case roll.IsNatural1():
			if saves.Failures != min(before.Failures+2, models.DeathSaveThreshold) {
				t.Fatalf("a natural 1 added %d failures, want 2", saves.Failures-before.Failures)
			}
		case roll.Total >= 10:
			if saves.Successes <= before.Successes && before.Successes < models.DeathSaveThreshold {
				t.Fatal("a 10 or higher should add a success")
			}
		default:
			if saves.Failures <= before.Failures && before.Failures < models.DeathSaveThreshold {
				t.Fatal("a 9 or lower should add a failure")
			}
		}

		if saves.Dead() || saves.Stabilised() || regained {
			saves = models.DeathSaves{}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestRollInitiativeRecordsTheTotal(t *testing.T) {
	r := NewSeeded(77)
	c := &models.Combatant{Name: "Thistle", InitiativeModifier: 4}

	roll := r.RollInitiative(c)
	if c.Initiative != roll.Total {
		t.Errorf("combatant initiative = %d, want the rolled %d", c.Initiative, roll.Total)
	}
	if roll.Total != roll.Natural+4 {
		t.Errorf("total = %d, want natural %d plus the +4 modifier", roll.Total, roll.Natural)
	}
}

func TestResultString(t *testing.T) {
	r := NewSeeded(8)
	result, _ := r.Roll("2d6+3")

	got := result.String()
	if got == "" {
		t.Fatal("String() returned nothing")
	}
	// It should name the expression and the total.
	if !strings.Contains(got, "2d6+3") || !strings.Contains(got, "=") {
		t.Errorf("String() = %q, want the expression and a total", got)
	}
}
