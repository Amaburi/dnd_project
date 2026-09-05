package models

import (
	"strings"
	"testing"
)

func cellar() *Location {
	return &Location{
		LocationID: "loc1", CampaignID: "camp1",
		Name: "The wine cellar", Description: "Low vaults, racks of dusty bottles.",
		Lighting: LightingDim,
		Interactables: []Interactable{
			{InteractableID: "f1", Name: "wine rack", Description: "Bottles, most of them empty.",
				Interactions: []InteractionKind{InteractSearch, InteractMove}},
			{InteractableID: "f2", Name: "iron-bound chest", Description: "Squat and padlocked.",
				Interactions: []InteractionKind{InteractOpen, InteractSearch},
				State:        StateLocked, UnlockDC: 15, UnlockSkill: SkillSleightOfHand},
			{InteractableID: "f3", Name: "loose flagstone", Description: "A hollow sound underfoot.",
				Interactions: []InteractionKind{InteractSearch, InteractMove},
				Hidden:       true, DiscoverDC: 14, DiscoverSkill: SkillPerception},
		},
		Exits: []Exit{
			{Direction: "north", Description: "Stairs up to the taproom", ToLocationID: "loc2"},
			{Direction: "behind the rack", Description: "A crawlspace", Hidden: true, DiscoverDC: 18},
		},
	}
}

// Dim light is lightly obscured: disadvantage on sight-based Perception.
// Darkness is heavily obscured, which blinds -- and a blinded creature does not
// get a check at all.
func TestLightingFollowsTheObscurementRules(t *testing.T) {
	if got := LightingBright.PerceptionMode(); got != RollNormal {
		t.Errorf("bright light gives %s", got)
	}
	if got := LightingDim.PerceptionMode(); got != RollDisadvantage {
		t.Errorf("dim light gives %s, want disadvantage", got)
	}
	if !LightingDark.BlindsSight() {
		t.Error("darkness should blind sight")
	}
	if LightingDim.BlindsSight() || LightingBright.BlindsSight() {
		t.Error("only darkness blinds")
	}
}

// Darkvision moves the world one step brighter, within its range.
func TestDarkvisionUpgradesLightingByOneStep(t *testing.T) {
	cases := []struct {
		light      Lighting
		darkvision int
		distance   int
		want       Lighting
	}{
		{LightingDark, 60, 30, LightingDim},
		{LightingDim, 60, 30, LightingBright},
		{LightingBright, 60, 30, LightingBright},

		// Beyond its range darkvision does nothing.
		{LightingDark, 60, 90, LightingDark},
		// And a creature without it sees what everyone else sees.
		{LightingDark, 0, 10, LightingDark},
	}
	for _, tc := range cases {
		if got := tc.light.SeenWithDarkvision(tc.darkvision, tc.distance); got != tc.want {
			t.Errorf("%s at %dft with %dft darkvision = %s, want %s",
				tc.light, tc.distance, tc.darkvision, got, tc.want)
		}
	}
}

// Hidden things are the DM's, not the party's. This is the list the prompt is
// allowed to describe.
func TestOnlyVisibleInteractablesAreOffered(t *testing.T) {
	loc := cellar()

	visible := loc.VisibleInteractables()
	if len(visible) != 2 {
		t.Fatalf("offered %d features, want the 2 that are not hidden", len(visible))
	}
	for _, f := range visible {
		if f.Hidden {
			t.Errorf("%q is hidden and was offered", f.Name)
		}
	}

	names := loc.InteractableNames()
	if len(names) != 2 || names[0] != "wine rack" {
		t.Errorf("feature names = %v", names)
	}

	exits := loc.VisibleExits()
	if len(exits) != 1 || exits[0].Direction != "north" {
		t.Errorf("exits = %+v, want only the visible one", exits)
	}
}

// A hidden feature must not reach the prompt at all. Telling a model "there is
// a secret door, do not mention it" is how secret doors get mentioned.
func TestTheSceneBlockNeverNamesSomethingHidden(t *testing.T) {
	block := cellar().SceneBlock()

	for _, want := range []string{"wine cellar", "wine rack", "iron-bound chest", "north"} {
		if !strings.Contains(block, want) {
			t.Errorf("the block is missing %q:\n%s", want, block)
		}
	}
	for _, secret := range []string{"flagstone", "crawlspace", "hidden"} {
		if strings.Contains(strings.ToLower(block), secret) {
			t.Errorf("the block leaks the hidden %q:\n%s", secret, block)
		}
	}
	// Lighting is part of the scene and part of the rules.
	if !strings.Contains(block, string(LightingDim)) {
		t.Errorf("the block does not say how bright it is:\n%s", block)
	}
}

// Searching reveals what the roll actually beat, and nothing else. The engine
// decides; the narrator is told afterwards.
func TestSearchingRevealsWhatTheRollBeats(t *testing.T) {
	loc := cellar()

	// A 12 is under the flagstone's DC of 14.
	found := loc.Discover(SkillPerception, 12)
	if len(found) != 0 {
		t.Errorf("a roll of 12 found %v", found)
	}
	if !loc.Interactables[2].Hidden {
		t.Error("the flagstone stopped being hidden on a failed search")
	}

	found = loc.Discover(SkillPerception, 15)
	if len(found) != 1 || found[0].Name != "loose flagstone" {
		t.Fatalf("a roll of 15 found %+v, want the flagstone", found)
	}
	if loc.Interactables[2].Hidden {
		t.Error("the flagstone is still hidden after being found")
	}

	// Finding it twice is not a second discovery.
	if again := loc.Discover(SkillPerception, 20); len(again) != 0 {
		t.Errorf("searching again re-found %v", again)
	}
}

// The skill matters: a flagstone noticed by looking is not found by listening.
func TestDiscoveryHonoursTheSkill(t *testing.T) {
	loc := cellar()
	if found := loc.Discover(SkillInsight, 20); len(found) != 0 {
		t.Errorf("an Insight check found %v", found)
	}
	// A feature with no skill named is found by any search.
	loc.Interactables[2].DiscoverSkill = ""
	if found := loc.Discover(SkillInvestigation, 20); len(found) != 1 {
		t.Errorf("a feature with no named skill was not found: %v", found)
	}
}

// A hidden exit is a secret door and follows the same rule.
func TestHiddenExitsAreDiscoveredToo(t *testing.T) {
	loc := cellar()
	if exits := loc.DiscoverExits(20); len(exits) != 1 || exits[0].Direction != "behind the rack" {
		t.Fatalf("found %+v", exits)
	}
	if loc.Exits[1].Hidden {
		t.Error("the crawlspace is still hidden after being found")
	}
}

// You cannot do to a thing what the thing does not do.
func TestInteractionsAreCheckedAgainstTheFeature(t *testing.T) {
	loc := cellar()
	rack, ok := loc.Interactable("wine rack")
	if !ok {
		t.Fatal("the wine rack is missing")
	}

	if !rack.Allows(InteractSearch) {
		t.Error("the rack should be searchable")
	}
	if rack.Allows(InteractOpen) {
		t.Error("the rack is not something that opens")
	}
	// Having a lock is enough to make it pickable: the chest lists only "open"
	// and "search", and requiring it to also list "unlock" would be
	// bookkeeping that gets forgotten, where the forgetting looks like a rule.
	chest, _ := loc.Interactable("iron-bound chest")
	if !chest.Allows(InteractUnlock) {
		t.Error("a locked chest cannot be picked")
	}
	if rack.Allows(InteractUnlock) {
		t.Error("a wine rack with no lock was pickable")
	}

	// A feature that lists nothing can be examined and no more, rather than
	// silently allowing everything.
	bare := Interactable{Name: "a scorch mark"}
	if bare.Allows(InteractOpen) {
		t.Error("a feature with no interactions allowed one")
	}
	if !bare.Allows(InteractExamine) {
		t.Error("anything can be looked at")
	}
}

// Lookup is by name because that is what a player says.
func TestInteractableLookupIsForgivingAboutCase(t *testing.T) {
	loc := cellar()
	for _, name := range []string{"iron-bound chest", "IRON-BOUND CHEST", "  Iron-Bound Chest "} {
		if _, ok := loc.Interactable(name); !ok {
			t.Errorf("Interactable(%q) found nothing", name)
		}
	}
	// A hidden feature is not findable by name either: the party does not know
	// it is there, so neither does the parser.
	if _, ok := loc.Interactable("loose flagstone"); ok {
		t.Error("a hidden feature was found by name")
	}
}

// A locked chest opens on a good enough roll and stays open.
func TestUnlockingAnInteractable(t *testing.T) {
	loc := cellar()
	chest, _ := loc.Interactable("iron-bound chest")

	if ok, reason := chest.CanOpen(); ok {
		t.Error("a locked chest opened without a check")
	} else if !strings.Contains(reason, "locked") {
		t.Errorf("reason = %q", reason)
	}

	if opened := chest.Unlock(10); opened {
		t.Error("a roll of 10 beat a DC of 15")
	}
	if opened := chest.Unlock(16); !opened {
		t.Error("a roll of 16 failed a DC of 15")
	}
	if chest.State != StateUnlocked {
		t.Errorf("state = %q, want unlocked", chest.State)
	}
	if ok, _ := chest.CanOpen(); !ok {
		t.Error("an unlocked chest still refuses to open")
	}
}

func TestLocationValidation(t *testing.T) {
	if err := cellar().Validate(); err != nil {
		t.Fatalf("a well-formed location was rejected: %v", err)
	}
	cases := map[string]func(*Location){
		"no name":                 func(l *Location) { l.Name = "" },
		"no campaign":             func(l *Location) { l.CampaignID = "" },
		"an invented lighting":    func(l *Location) { l.Lighting = "gloomy-ish" },
		"a feature with no name":  func(l *Location) { l.Interactables[0].Name = "" },
		"an invented interaction": func(l *Location) { l.Interactables[0].Interactions = []InteractionKind{"vibe with"} },
		"an invented state":       func(l *Location) { l.Interactables[1].State = "ajar-ish" },
		"an exit to nowhere":      func(l *Location) { l.Exits[0].Direction = "" },
	}
	for name, mutate := range cases {
		loc := cellar()
		mutate(loc)
		if err := loc.Validate(); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}
