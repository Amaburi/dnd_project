package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

func testOptions() models.ActionOptions {
	return models.ActionOptions{
		Actor:   "Thistle",
		Skills:  models.Skills,
		Weapons: []string{"Rapier", "Shortbow"},
		Spells:  []string{"Mage Hand"},
		Items:   []string{"Thieves' Tools", "Healing Potion"},
		Targets: []string{"Goblin", "Goblin Boss"},
	}
}

func extract(t *testing.T, reply string, opts models.ActionOptions, input string) *IntentResponse {
	t.Helper()
	service, _ := NewStubService(reply)

	out, err := service.ExtractIntent(context.Background(), &IntentRequest{
		PlayerInput: input, Options: opts, Situation: "in combat",
	})
	if err != nil {
		t.Fatalf("ExtractIntent: %v", err)
	}
	return out
}

func TestExtractIntentParsesAnAttack(t *testing.T) {
	out := extract(t, `{"action":"attack","target":"Goblin","weapon":"Rapier","confidence":"high",
		"rationale":"names a weapon and a creature in play"}`,
		testOptions(), "I stab the goblin with my rapier")

	if out.Repaired {
		t.Fatalf("a valid intent was repaired: %s", out.RepairNote)
	}
	if out.Intent.Action != models.IntentAttack {
		t.Errorf("action = %s, want attack", out.Intent.Action)
	}
	if out.Intent.Target != "Goblin" || out.Intent.Weapon != "Rapier" {
		t.Errorf("target/weapon = %q/%q", out.Intent.Target, out.Intent.Weapon)
	}
	if out.Intent.Actor != "Thistle" {
		t.Errorf("actor = %q, want the option list's actor", out.Intent.Actor)
	}
	if out.Intent.RawInput == "" {
		t.Error("the original sentence was not kept")
	}
	if out.TokensUsed == 0 || out.Cost == 0 {
		t.Error("usage and cost were not reported")
	}
}

// Models are inconsistent about casing and spacing; rejecting "Sleight of Hand"
// would be pedantry rather than safety.
func TestExtractIntentNormalisesLooseFormatting(t *testing.T) {
	out := extract(t, `{"action":"Skill_Check","skill":"Sleight of Hand","confidence":"HIGH","suggested_dc":15}`,
		testOptions(), "I pick his pocket")

	if out.Intent.Action != models.IntentSkillCheck {
		t.Errorf("action = %q, want skill_check", out.Intent.Action)
	}
	if out.Intent.Skill != models.SkillSleightOfHand {
		t.Errorf("skill = %q, want sleight_of_hand", out.Intent.Skill)
	}
	if out.Intent.Confidence != models.ConfidenceHigh {
		t.Errorf("confidence = %q, want high", out.Intent.Confidence)
	}
	// The ability is inferred from the skill when the model omits it.
	if out.Intent.Ability != models.AbilityDexterity {
		t.Errorf("ability = %q, want dexterity inferred from the skill", out.Intent.Ability)
	}
}

// A model naming a weapon the character is not carrying is a hallucination.
// Turning it into a question beats resolving a fiction.
func TestExtractIntentRepairsAnImpossibleAction(t *testing.T) {
	out := extract(t, `{"action":"attack","target":"Goblin","weapon":"Greatsword","confidence":"high"}`,
		testOptions(), "I cleave the goblin with my greatsword")

	if !out.Repaired {
		t.Fatal("an attack with a weapon the character lacks should be repaired")
	}
	if out.Intent.Action != models.IntentUnclear {
		t.Errorf("action = %s, want unclear", out.Intent.Action)
	}
	if out.Intent.Clarification == "" {
		t.Error("a repaired intent must carry a question for the player")
	}
	if !strings.Contains(out.RepairNote, "Greatsword") {
		t.Errorf("repair note does not name the problem: %q", out.RepairNote)
	}
}

func TestExtractIntentRepairsAnUnknownTarget(t *testing.T) {
	out := extract(t, `{"action":"attack","target":"Dragon","weapon":"Rapier","confidence":"medium"}`,
		testOptions(), "I attack the dragon")

	if !out.Repaired || out.Intent.Action != models.IntentUnclear {
		t.Fatalf("attacking a creature not in play should be repaired, got %+v", out.Intent)
	}
}

func TestExtractIntentAcceptsNarrativeActions(t *testing.T) {
	out := extract(t, `{"action":"narrative","confidence":"high","rationale":"nothing is at stake"}`,
		testOptions(), "I look around the room")

	if out.Repaired {
		t.Fatalf("a narrative intent was repaired: %s", out.RepairNote)
	}
	if out.Intent.Action.NeedsResolution() {
		t.Error("a narrative action should not need the rules engine")
	}
}

func TestExtractIntentKeepsUnclearAsAQuestion(t *testing.T) {
	out := extract(t, `{"action":"unclear","confidence":"low",
		"clarification":"Do you mean the goblin or the goblin boss?"}`,
		testOptions(), "I attack it")

	if out.Repaired {
		t.Fatalf("a well-formed unclear intent should pass through: %s", out.RepairNote)
	}
	if !strings.Contains(out.Intent.Clarification, "goblin boss") {
		t.Errorf("clarification = %q", out.Intent.Clarification)
	}
}

// Models wrap JSON in fences often enough that refusing one would fail for a
// reason unrelated to the answer's quality.
func TestParseIntentUnwrapsMarkdownAndProse(t *testing.T) {
	cases := []string{
		"```json\n{\"action\":\"attack\",\"target\":\"Goblin\",\"confidence\":\"high\"}\n```",
		"Here is the parse:\n{\"action\":\"attack\",\"target\":\"Goblin\",\"confidence\":\"high\"}\nHope that helps!",
		"  {\"action\":\"attack\",\"target\":\"Goblin\",\"confidence\":\"high\"}  ",
	}

	for _, reply := range cases {
		intent, err := ParseIntent(reply)
		if err != nil {
			t.Errorf("ParseIntent(%.30q): %v", reply, err)
			continue
		}
		if intent.Action != models.IntentAttack || intent.Target != "Goblin" {
			t.Errorf("ParseIntent(%.30q) = %+v", reply, intent)
		}
	}
}

// A brace inside a string must not end the object early.
func TestParseIntentHandlesBracesInsideStrings(t *testing.T) {
	intent, err := ParseIntent(`{"action":"unclear","confidence":"low","clarification":"Did you mean {the door} or the chest?"}`)
	if err != nil {
		t.Fatalf("ParseIntent: %v", err)
	}
	if !strings.Contains(intent.Clarification, "{the door}") {
		t.Errorf("clarification = %q, want the braces preserved", intent.Clarification)
	}
}

func TestParseIntentRejectsGarbage(t *testing.T) {
	for _, reply := range []string{
		"",
		"I think the player wants to attack.",
		`{"action":"teleport","confidence":"high"}`,
		`{"action":"attack",`,
	} {
		if _, err := ParseIntent(reply); err == nil {
			t.Errorf("ParseIntent(%q) should have failed", reply)
		}
	}
}

// The parse call must be deterministic, or the same sentence resolves
// differently between runs.
func TestIntentExtractionRunsAtZeroTemperatureInJSONMode(t *testing.T) {
	service, stub := NewStubService(`{"action":"narrative","confidence":"high"}`)

	if _, err := service.ExtractIntent(context.Background(), &IntentRequest{
		PlayerInput: "I look around", Options: testOptions(),
	}); err != nil {
		t.Fatalf("ExtractIntent: %v", err)
	}

	req := stub.LastRequest()
	if req.Temperature == nil {
		t.Fatal("temperature was not set on the parse call")
	}
	if *req.Temperature != 0 {
		t.Errorf("temperature = %v, want 0", *req.Temperature)
	}
	if req.ResponseFormat == nil || req.ResponseFormat.Type != "json_object" {
		t.Errorf("response format = %+v, want json_object", req.ResponseFormat)
	}
}

// The parser must be shown the closed lists, or it has nothing to choose from
// and will invent names.
func TestIntentPromptCarriesTheAvailableOptions(t *testing.T) {
	service, stub := NewStubService(`{"action":"narrative","confidence":"high"}`)

	if _, err := service.ExtractIntent(context.Background(), &IntentRequest{
		PlayerInput: "I look around", Options: testOptions(), Situation: "exploring the crypt",
	}); err != nil {
		t.Fatalf("ExtractIntent: %v", err)
	}

	prompt := stub.LastPrompt()
	for _, want := range []string{"Rapier", "Shortbow", "Mage Hand", "Goblin Boss", "sleight_of_hand", "exploring the crypt"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt does not offer %q", want)
		}
	}
	// And it must be told it is a parser, not a DM.
	if !strings.Contains(prompt, "parser, not a Dungeon Master") {
		t.Error("the parse prompt does not tell the model it is not the DM")
	}
}

func TestExtractIntentRejectsEmptyInput(t *testing.T) {
	service, _ := NewStubService(`{"action":"narrative","confidence":"high"}`)

	if _, err := service.ExtractIntent(context.Background(), &IntentRequest{
		PlayerInput: "   ", Options: testOptions(),
	}); err == nil {
		t.Error("an empty sentence should be rejected before a call is made")
	}
}

func TestActionOptionsFromACharacter(t *testing.T) {
	rapier := models.InventoryItem{
		Name: "Rapier", Kind: models.ItemWeapon,
		Weapon: &models.WeaponProperties{Category: models.WeaponMartial, DamageDice: "1d8"},
	}
	rope := models.InventoryItem{Name: "Rope", Kind: models.ItemGear}

	c := &models.Character{
		Name:      "Thistle",
		Inventory: []models.InventoryItem{rapier, rope},
		Equipment: models.Equipment{Weapons: []models.InventoryItem{rapier}},
		Spells: models.Spells{
			Cantrips: []string{"Mage Hand"},
			Known:    []models.Spell{{Name: "Disguise Self", Level: 1}},
		},
	}

	opts := models.ActionOptionsFor(c, []string{"Goblin"})

	if opts.Actor != "Thistle" {
		t.Errorf("actor = %q", opts.Actor)
	}
	if len(opts.Weapons) != 1 || opts.Weapons[0] != "Rapier" {
		t.Errorf("weapons = %v, want just the rapier", opts.Weapons)
	}
	if len(opts.Items) != 1 || opts.Items[0] != "Rope" {
		t.Errorf("items = %v, want the rope only (weapons are listed separately)", opts.Items)
	}
	if len(opts.Spells) != 2 {
		t.Errorf("spells = %v, want the cantrip and the known spell", opts.Spells)
	}
	if len(opts.Skills) != 18 {
		t.Errorf("skills = %d, want all 18 (proficiency only changes the modifier)", len(opts.Skills))
	}

	prompt := opts.Prompt()
	for _, want := range []string{"Thistle", "Goblin", "Rapier", "Rope", "Mage Hand"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("options prompt is missing %q:\n%s", want, prompt)
		}
	}
}

func TestDifficultyLabels(t *testing.T) {
	cases := map[int]string{5: "very_easy", 10: "easy", 15: "medium", 20: "hard", 25: "very_hard", 30: "nearly_impossible"}
	for dc, want := range cases {
		if got := models.DifficultyLabel(dc); got != want {
			t.Errorf("DifficultyLabel(%d) = %q, want %q", dc, got, want)
		}
	}
	// The table's rungs and the labels agree.
	for label, dc := range models.DifficultyClasses {
		if got := models.DifficultyLabel(dc); got != label {
			t.Errorf("DC %d labelled %q, want %q", dc, got, label)
		}
	}
}

func TestIntentNormaliseClampsSuggestedDC(t *testing.T) {
	for _, tc := range []struct{ in, want int }{{0, 0}, {3, 5}, {15, 15}, {99, 30}} {
		intent := models.Intent{Action: models.IntentSkillCheck, SuggestedDC: tc.in}
		intent.Normalise()
		if intent.SuggestedDC != tc.want {
			t.Errorf("SuggestedDC %d normalised to %d, want %d", tc.in, intent.SuggestedDC, tc.want)
		}
	}
}
