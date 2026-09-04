package dice

import (
	"testing"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

// A seeded roller is reproducible but not *controllable*: a test that needs a
// hit has to hope the seed obliges, and the ones that did not were papered
// over with t.Skip. A scripted roller lets a test state the dice outright.
func TestScriptedRollerReturnsTheGivenFaces(t *testing.T) {
	r := NewScripted(20, 1, 7)

	if got := r.D20(0, models.RollNormal).Natural; got != 20 {
		t.Errorf("first d20 = %d, want 20", got)
	}
	if got := r.D20(0, models.RollNormal).Natural; got != 1 {
		t.Errorf("second d20 = %d, want 1", got)
	}
	if got := r.D20(0, models.RollNormal).Natural; got != 7 {
		t.Errorf("third d20 = %d, want 7", got)
	}
}

// Once the script runs out the last face repeats, so a test only has to state
// the rolls it cares about rather than every die a call will make.
func TestScriptedRollerRepeatsTheLastFace(t *testing.T) {
	r := NewScripted(20, 4)

	r.D20(0, models.RollNormal)
	for i := 0; i < 5; i++ {
		if got := r.D20(0, models.RollNormal).Natural; got != 4 {
			t.Fatalf("roll %d after the script = %d, want the last face 4", i, got)
		}
	}
}

// A face larger than the die is clamped rather than producing an impossible
// result: NewScripted(20) on a d8 means "as high as it goes".
func TestScriptedRollerClampsToTheDie(t *testing.T) {
	r := NewScripted(20)

	result, err := r.Roll("2d6")
	if err != nil {
		t.Fatalf("Roll: %v", err)
	}
	for _, roll := range result.Rolls {
		if roll != 6 {
			t.Errorf("a d6 rolled %d, want it clamped to 6", roll)
		}
	}

	low := NewScripted(0, -3)
	if got, _ := low.Roll("1d8"); got.Rolls[0] != 1 {
		t.Errorf("a face below 1 gave %d, want it clamped to 1", got.Rolls[0])
	}
}

// The point of the whole thing: an attack whose outcome the test decides.
func TestScriptedRollerMakesOutcomesCertain(t *testing.T) {
	hit := NewScripted(20)
	if got := hit.D20(0, models.RollNormal); !got.IsNatural20() {
		t.Errorf("scripted 20 rolled %d", got.Natural)
	}

	miss := NewScripted(1)
	if got := miss.D20(0, models.RollNormal); !got.IsNatural1() {
		t.Errorf("scripted 1 rolled %d", got.Natural)
	}
}

// Advantage still draws two dice, so a script feeds both.
func TestScriptedRollerFeedsBothDiceUnderAdvantage(t *testing.T) {
	r := NewScripted(5, 18)

	result := r.D20(0, models.RollAdvantage)
	if len(result.Rolls) != 2 {
		t.Fatalf("rolled %d dice, want 2", len(result.Rolls))
	}
	if result.Rolls[0] != 5 || result.Rolls[1] != 18 {
		t.Errorf("dice = %v, want [5 18]", result.Rolls)
	}
	if result.Natural != 18 {
		t.Errorf("kept %d, want the higher 18", result.Natural)
	}
}

func TestScriptedRollerRequiresAScript(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewScripted with no faces should panic rather than roll silently")
		}
	}()
	NewScripted()
}
