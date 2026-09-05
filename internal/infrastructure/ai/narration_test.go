package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

// A representative Facts map, the shape rules.AttackResult.Facts() produces.
func attackFacts() map[string]string {
	return map[string]string{
		"attacker": "Thistle", "target": "Goblin", "weapon": "Rapier",
		"roll_mode": "normal", "natural": "18", "all_rolls": "18",
		"attack_bonus": "+7", "attack_total": "25", "target_ac": "15",
		"outcome": "hit", "hit": "yes", "critical": "no",
		"damage_total": "9", "damage_type": "piercing",
		"damage_expression": "1d8+4", "damage_affinity": "normal",
		"target_status": "dead", "target_hp": "0/7",
		"fact_summary": "Thistle hits Goblin with Rapier for 9 piercing damage; Goblin dies",
	}
}

func checkFacts() map[string]string {
	return map[string]string{
		"check_kind": "skill_check", "actor": "Thistle",
		"ability": "dexterity", "skill": "stealth", "dc": "15",
		"roll_mode": "normal", "natural": "12", "all_rolls": "12",
		"modifier": "+10", "total": "22", "outcome": "success",
		"margin": "7", "was_close": "no", "automatic_failure": "no",
		"fact_summary": "Thistle succeeds on a DC 15 stealth skill check",
	}
}

// This is the contract with the rules engine: the prompt must carry the exact
// numbers the engine decided, so the model has nothing left to choose.
func TestNarrateActionCarriesEveryFactIntoThePrompt(t *testing.T) {
	service, stub := NewStubService("The rapier finds a gap in the goblin's guard.")

	resp, err := service.NarrateAction(context.Background(), &NarrationRequest{
		Facts:   attackFacts(),
		Context: "a damp cellar",
		Style:   NarrationStyle{NarrativeVoice: "third person", CombatTone: "grim"},
	})
	if err != nil {
		t.Fatalf("NarrateAction: %v", err)
	}
	if resp.Text == "" {
		t.Error("no narration returned")
	}

	prompt := stub.LastPrompt()
	for _, want := range []string{
		"Thistle", "Goblin", "Rapier", "9", "piercing", "0/7", "dead", "a damp cellar", "grim",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt is missing the fact %q", want)
		}
	}
	if strings.Contains(prompt, "{{") {
		t.Errorf("prompt still contains an unresolved placeholder:\n%s", prompt)
	}
}

// The narration contract is the paragraph that stops the model inventing
// outcomes. If it goes missing the whole separation collapses silently.
func TestNarrationPromptForbidsDeciding(t *testing.T) {
	service, stub := NewStubService("prose")

	for name, call := range map[string]func() error{
		"action": func() error {
			_, err := service.NarrateAction(context.Background(), &NarrationRequest{Facts: attackFacts()})
			return err
		},
		"check": func() error {
			_, err := service.NarrateCheck(context.Background(), &NarrationRequest{Facts: checkFacts()})
			return err
		},
	} {
		if err := call(); err != nil {
			t.Fatalf("%s narration: %v", name, err)
		}

		prompt := stub.LastPrompt()
		for _, want := range []string{
			"already been decided",
			"Never roll dice",
			"Never decide an outcome",
			"You describe; the\n  engine decides",
		} {
			if !strings.Contains(prompt, want) {
				t.Errorf("%s prompt is missing the instruction %q", name, want)
			}
		}
	}
}

// Narrating without facts would be the model making the outcome up, which is
// exactly what the split exists to prevent.
func TestNarrationRefusesWithoutFacts(t *testing.T) {
	service, stub := NewStubService("prose")

	if _, err := service.NarrateAction(context.Background(), &NarrationRequest{}); err == nil {
		t.Error("narrating with no facts should be refused")
	}
	if stub.CallCount() != 0 {
		t.Error("a request was sent despite having no facts")
	}
}

// Style values must never be able to overwrite an engine fact.
func TestFactsWinOverStyle(t *testing.T) {
	service, stub := NewStubService("prose")

	facts := attackFacts()
	facts["context"] = "FACT-CONTEXT" // a fact named like a style variable

	if _, err := service.NarrateAction(context.Background(), &NarrationRequest{
		Facts:   facts,
		Context: "STYLE-CONTEXT",
	}); err != nil {
		t.Fatalf("NarrateAction: %v", err)
	}

	prompt := stub.LastPrompt()
	if !strings.Contains(prompt, "FACT-CONTEXT") {
		t.Error("a style value overwrote an engine fact")
	}
}

func TestNarrateCheckUsesTheCheckTemplate(t *testing.T) {
	service, stub := NewStubService("Thistle melts into the shadows.")

	if _, err := service.NarrateCheck(context.Background(), &NarrationRequest{
		Facts: checkFacts(), Context: "a torchlit corridor",
	}); err != nil {
		t.Fatalf("NarrateCheck: %v", err)
	}

	prompt := stub.LastPrompt()
	for _, want := range []string{"stealth", "DC 15", "success", "margin of 7"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("check prompt is missing %q", want)
		}
	}
	// It must not be the combat template.
	if strings.Contains(prompt, "combat action") {
		t.Error("a check was narrated with the combat template")
	}
}

func TestNarrationStyleHasDefaults(t *testing.T) {
	service, stub := NewStubService("prose")

	if _, err := service.NarrateAction(context.Background(), &NarrationRequest{Facts: attackFacts()}); err != nil {
		t.Fatalf("NarrateAction: %v", err)
	}

	prompt := stub.LastPrompt()
	if strings.Contains(prompt, "{{") {
		t.Error("missing style values left placeholders in the prompt")
	}
	if !strings.Contains(prompt, "no additional context") {
		t.Error("an absent context should fall back to a placeholder, not an empty string")
	}
}

// Every template in defaultPrompts must be reachable from a Service method: an
// uncallable prompt drifts out of date unnoticed.
// castFacts is a complete spell fact set, matching rules.CastResult.Facts().
func castFacts() map[string]string {
	return map[string]string{
		"caster": "Alaric", "spell": "Magic Missile", "slot_level": "1",
		"target": "Goblin", "outcome": "hit", "projectiles": "3", "hits": "3",
		"damage_total": "10", "damage_type": "force", "damage_affinity": "normal",
		"healing": "0", "condition": "none",
		"save_ability": "none", "save_dc": "0", "save_total": "none", "save_automatic": "no",
		"target_hp": "0/7", "target_status": "dead",
		"fact_summary": "Alaric casts Magic Missile for 10 force damage; Goblin dies",
	}
}

func TestEveryTemplateHasACaller(t *testing.T) {
	// Each call gets its own reply, so the two JSON templates are answered
	// with JSON and the rest with prose.
	service, stub := NewStubService()
	ctx := context.Background()

	calls := map[string]func() error{
		"intent_extraction": func() error {
			stub.Replies = []string{`{"action":"narrative","confidence":"high"}`}
			stub.Requests = nil
			_, err := service.ExtractIntent(ctx, &IntentRequest{PlayerInput: "look", Options: testOptions()})
			return err
		},
		"enemy_tactics": func() error {
			stub.Replies = []string{`{"action":"Scimitar","target":"Thistle","rationale":"nearest"}`}
			stub.Requests = nil
			_, err := service.ChooseTactics(ctx, &TacticsRequest{
				Monster: tacticsMonster(),
				Self:    testCombatant("Goblin", "enemy", 7),
				Enemies: []models.Combatant{testCombatant("Thistle", "player", 30)},
				Round:   1,
			})
			return err
		},
		"action_narration": func() error {
			_, err := service.NarrateAction(ctx, &NarrationRequest{Facts: attackFacts()})
			return err
		},
		"spell_narration": func() error {
			_, err := service.NarrateCast(ctx, &NarrationRequest{Facts: castFacts()})
			return err
		},
		"check_narration": func() error {
			_, err := service.NarrateCheck(ctx, &NarrationRequest{Facts: checkFacts()})
			return err
		},
		"dm_base": func() error {
			_, err := service.GenerateNarrative(ctx, &NarrativeRequest{PlayerInput: "I search"})
			return err
		},
		"npc_dialogue": func() error {
			_, err := service.GenerateNPCDialogue(ctx, &NPCDialogueRequest{
				NPC:         &models.NPC{Name: "Garrick", CampaignID: "c", Status: models.NPCAlive},
				SpeakerName: "Thistle", PlayerMessage: "Hello.",
			})
			return err
		},
		"narrative_generation": func() error {
			_, err := service.DescribeScene(ctx, &SceneRequest{SceneDescription: "a crypt"})
			return err
		},
		"story_adaptation": func() error {
			_, err := service.AdaptStory(ctx, &StoryAdaptationRequest{PlayerChoice: "they spared him"})
			return err
		},
		"character_backstory": func() error {
			_, err := service.GenerateBackstory(ctx, &BackstoryRequest{CharacterName: "Thistle"})
			return err
		},
		"history_summary": func() error {
			_, err := service.SummarizeHistory(ctx, "", []string{"They set out."})
			return err
		},
		"story_review": func() error {
			stub.Replies = []string{`{"new_threads":[],"new_consequences":[]}`}
			stub.Requests = nil
			_, err := service.ReviewStory(ctx, &StoryReviewRequest{RecentEvents: []string{"They arrived."}})
			return err
		},
		"quest_generation": func() error {
			_, err := service.GenerateQuest(ctx, &QuestRequest{QuestType: "rescue"})
			return err
		},
	}

	for name := range defaultPrompts() {
		call, ok := calls[name]
		if !ok {
			t.Errorf("template %q has no Service method; it can never be used", name)
			continue
		}
		if _, isJSON := map[string]bool{"intent_extraction": true, "enemy_tactics": true}[name]; !isJSON {
			stub.Replies = []string{"prose"}
			stub.Requests = nil
		}
		if err := call(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// A prompt with an empty variable reads as an instruction to invent something,
// so every optional field falls back to a placeholder.
func TestOptionalFieldsFallBackRatherThanBlank(t *testing.T) {
	service, stub := NewStubService("prose")

	if _, err := service.GenerateQuest(context.Background(), &QuestRequest{}); err != nil {
		t.Fatalf("GenerateQuest with no fields: %v", err)
	}

	prompt := stub.LastPrompt()
	if strings.Contains(prompt, "{{") {
		t.Errorf("unresolved placeholder in a fully defaulted prompt:\n%s", prompt)
	}
	for _, blank := range []string{": \n", ":\n\n"} {
		if strings.Contains(prompt, blank) {
			t.Errorf("prompt contains an empty field, which reads as an invitation to invent:\n%s", prompt)
		}
	}
}

// The scene narrator must not be told to enforce rules -- that was the standing
// contradiction between the prompt and the architecture.
func TestSceneNarratorHasNoRulesAuthority(t *testing.T) {
	service, stub := NewStubService("prose")

	if _, err := service.GenerateNarrative(context.Background(), &NarrativeRequest{
		PlayerInput: "I search the room",
	}); err != nil {
		t.Fatalf("GenerateNarrative: %v", err)
	}

	prompt := stub.LastPrompt()
	for _, forbidden := range []string{"Enforce", "Game State Changes"} {
		if strings.Contains(prompt, forbidden) {
			t.Errorf("the narrator prompt still claims rules authority (%q)", forbidden)
		}
	}
	for _, want := range []string{"Never roll dice", "separate rules engine", "no authority over it"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the narrator prompt is missing %q", want)
		}
	}
}

// testCombatant builds a minimal combatant for tactics tests.
func testCombatant(name, kind string, hp int) models.Combatant {
	return models.Combatant{
		CombatantID: name, Name: name, Type: kind, ArmorClass: 14,
		HitPoints: models.HitPoints{Current: hp, Maximum: hp},
		Status:    models.CombatantActive,
	}
}

func tacticsMonster() *models.Monster {
	for _, m := range models.SRDMonsters() {
		if m.MonsterID == "srd_goblin" {
			copy := m
			return &copy
		}
	}
	panic("goblin missing")
}
