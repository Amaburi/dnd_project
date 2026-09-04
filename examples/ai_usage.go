// Command ai_usage demonstrates one full turn: parse the player's sentence,
// let the rules engine decide what happened, then have the model narrate that
// verdict.
//
// It runs offline against a stub client by default, so it costs nothing. Pass
// -live to send the same prompts to the configured provider.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/dnd-campaign/manager/internal/domain/dice"
	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/dnd-campaign/manager/internal/domain/rules"
	"github.com/dnd-campaign/manager/internal/infrastructure/ai"
	"github.com/dnd-campaign/manager/internal/infrastructure/config"
)

func main() {
	live := flag.Bool("live", false, "call the configured provider instead of the offline stub")
	input := flag.String("input", "I slash at the goblin with my rapier", "what the player said")
	flag.Parse()

	hero := buildCharacter()
	goblin := findGoblin()
	target := goblin.ToCombatant("g1")

	// A fixed seed so the demo tells the same story every run.
	engine := rules.NewEngine(dice.NewSeeded(1337))

	service, stub := newService(*live)
	ctx := context.Background()

	fmt.Printf("%s (AC %d)  vs  %s (AC %d, %d hp)\n\n",
		hero.Name, hero.ArmorClass(), target.Name, target.ArmorClass, target.HitPoints.Maximum)
	fmt.Printf("Player: %q\n\n", *input)

	// --- 1. Parse -----------------------------------------------------------
	options := models.ActionOptionsFor(hero, []string{target.Name})
	intent, err := service.ExtractIntent(ctx, &ai.IntentRequest{
		PlayerInput: *input,
		Options:     options,
		Situation:   "in combat, round 1",
	})
	if err != nil {
		log.Fatalf("intent extraction failed: %v", err)
	}

	fmt.Println("1. PARSED INTENT")
	fmt.Printf("   action=%s target=%q weapon=%q confidence=%s\n",
		intent.Intent.Action, intent.Intent.Target, intent.Intent.Weapon, intent.Intent.Confidence)
	if intent.Repaired {
		fmt.Printf("   repaired: %s\n", intent.RepairNote)
	}
	if intent.Intent.Action == models.IntentUnclear {
		fmt.Printf("\n   Ask the player: %s\n", intent.Intent.Clarification)
		return
	}

	// --- 2. Resolve ---------------------------------------------------------
	weapon := findWeapon(hero, intent.Intent.Weapon)
	result, err := engine.WeaponAttack(hero, weapon, &target, models.RollNormal)
	if err != nil {
		log.Fatalf("resolution failed: %v", err)
	}

	fmt.Println("\n2. ENGINE VERDICT (authoritative)")
	fmt.Printf("   %s\n", result.Summary())

	// --- 3. Narrate ---------------------------------------------------------
	narration, err := service.NarrateAction(ctx, &ai.NarrationRequest{
		Facts:   result.Facts(),
		Context: "a damp cellar lit by one guttering torch",
		Style:   ai.NarrationStyle{NarrativeVoice: "third person, present tense", CombatTone: "grim and quick"},
	})
	if err != nil {
		log.Fatalf("narration failed: %v", err)
	}

	fmt.Println("\n3. NARRATION")
	fmt.Printf("   %s\n", strings.TrimSpace(narration.Text))
	fmt.Printf("\n   (%d tokens, $%.5f)\n", narration.TokensUsed, narration.Cost)

	if stub != nil {
		fmt.Printf("\n   Offline run: %d prompts assembled, none sent. Use -live to send them.\n",
			stub.CallCount())
	}
}

// newService returns either a live service or an offline stub.
func newService(live bool) (*ai.Service, *ai.StubClient) {
	if !live {
		// The stub answers the intent call with JSON and the narration call
		// with prose, in that order.
		return ai.NewStubService(
			`{"action":"attack","target":"Goblin","weapon":"Rapier","confidence":"high",
			  "rationale":"the player names a weapon and a creature in play"}`,
			"The rapier slides past the goblin's guard and punches through leather. "+
				"It folds around the blade with a wet grunt and does not get up.",
		)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	service, err := ai.NewService(ai.ClientConfig{
		Provider:   cfg.AI.Provider,
		APIKey:     cfg.AI.APIKey,
		BaseURL:    cfg.AI.BaseURL,
		Model:      cfg.AI.Model,
		Timeout:    cfg.AI.Timeout,
		MaxRetries: cfg.AI.MaxRetries,
		Pricing: ai.Pricing{
			PromptUSDPerMillion:     cfg.AI.Pricing.PromptUSDPerMillion,
			CompletionUSDPerMillion: cfg.AI.Pricing.CompletionUSDPerMillion,
		},
	})
	if err != nil {
		log.Fatalf("failed to create AI service: %v", err)
	}
	return service, nil
}

func buildCharacter() *models.Character {
	rapier := models.InventoryItem{
		ItemID: "w1", Key: "rapier", Name: "Rapier", Kind: models.ItemWeapon, Weight: 2,
		Equipped: true,
		Weapon: &models.WeaponProperties{
			Category: models.WeaponMartial, DamageDice: "1d8",
			DamageType: models.DamagePiercing,
			Properties: []models.WeaponProperty{models.PropertyFinesse},
		},
	}

	c := &models.Character{
		Name: "Thistle", Type: models.CharacterPlayer,
		BasicInfo: models.BasicInfo{
			Race: models.RaceHalfling, Subrace: "lightfoot",
			Background: models.BackgroundCriminal,
			Classes:    []models.ClassLevel{{Class: models.ClassRogue, Subclass: "thief", Level: 5}},
		},
		AbilityScores: models.AbilityScores{
			Strength: 10, Dexterity: 18, Constitution: 14,
			Intelligence: 12, Wisdom: 13, Charisma: 11,
		},
		Skills: models.SkillProficiencies{
			models.SkillStealth:    models.ProficiencyExpertise,
			models.SkillAcrobatics: models.ProficiencyProficient,
		},
		Proficiencies: models.Proficiencies{
			Weapons: []string{models.ProfSimpleWeapons, "rapier", "shortsword"},
		},
		Inventory: []models.InventoryItem{rapier},
		Equipment: models.Equipment{Weapons: []models.InventoryItem{rapier}},
		CombatStats: models.CombatStats{
			HitPoints: models.HitPoints{Current: 33, Maximum: 33},
		},
	}
	c.ApplyClassDefaults()
	return c
}

func findGoblin() models.Monster {
	for _, m := range models.SRDMonsters() {
		if m.MonsterID == "srd_goblin" {
			return m
		}
	}
	log.Fatal("goblin missing from the SRD catalogue")
	return models.Monster{}
}

// findWeapon resolves the name the parser returned to an item the character is
// actually carrying, falling back to the first equipped weapon.
func findWeapon(c *models.Character, name string) models.InventoryItem {
	for _, w := range c.Equipment.Weapons {
		if strings.EqualFold(w.Name, name) {
			return w
		}
	}
	if len(c.Equipment.Weapons) > 0 {
		return c.Equipment.Weapons[0]
	}
	log.Fatal("character is carrying no weapon")
	return models.InventoryItem{}
}
