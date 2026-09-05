package models

import "testing"

// The parser proposes a difficulty and the turn resolves against it directly.
// That is a defensible division -- setting difficulty is a DM's judgement and
// the AI is the DM here -- but it must land on the table's rungs. A DC 17 is
// not a 5e number; it is a model splitting the difference.
func TestSuggestedDifficultySnapsToTheStandardRungs(t *testing.T) {
	cases := map[int]int{
		5: 5, 10: 10, 15: 15, 20: 20, 25: 25, 30: 30,

		7: 5, 8: 10, 12: 10, 13: 15, 17: 15, 18: 20,
		22: 20, 23: 25, 27: 25, 28: 30,

		// Outside the table, clamped to its ends.
		1: 5, 4: 5, 45: 30, 100: 30,
	}
	for given, want := range cases {
		if got := SnapToDifficulty(given); got != want {
			t.Errorf("SnapToDifficulty(%d) = %d, want %d", given, got, want)
		}
	}

	// Zero means "no difficulty proposed" and must survive as zero, or every
	// action would acquire a DC of 5.
	if got := SnapToDifficulty(0); got != 0 {
		t.Errorf("SnapToDifficulty(0) = %d, want 0 to mean unset", got)
	}
	if got := SnapToDifficulty(-3); got != 0 {
		t.Errorf("SnapToDifficulty(-3) = %d, want 0", got)
	}
}

// Every rung must survive a round trip through its own label, or the two
// tables have drifted apart.
func TestEveryDifficultyRungRoundTrips(t *testing.T) {
	for label, dc := range DifficultyClasses {
		if got := SnapToDifficulty(dc); got != dc {
			t.Errorf("%s (DC %d) snapped to %d", label, dc, got)
		}
		if got := DifficultyLabel(dc); got != label {
			t.Errorf("DC %d labels as %q, want %q", dc, got, label)
		}
	}
}

// Normalise is where a parsed intent is made safe, so the snapping belongs
// there rather than at each call site that might forget.
func TestNormaliseSnapsTheSuggestedDC(t *testing.T) {
	i := Intent{Action: IntentSkillCheck, Skill: SkillAthletics, SuggestedDC: 17}
	i.Normalise()
	if i.SuggestedDC != 15 {
		t.Errorf("SuggestedDC = %d, want it snapped to 15", i.SuggestedDC)
	}

	unset := Intent{Action: IntentNarrative}
	unset.Normalise()
	if unset.SuggestedDC != 0 {
		t.Errorf("an unset DC became %d", unset.SuggestedDC)
	}
}
