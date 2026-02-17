package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/dnd-campaign/manager/internal/infrastructure/ai"
	"github.com/dnd-campaign/manager/internal/infrastructure/config"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Check if API key is set
	if cfg.DeepSeek.APIKey == "" {
		log.Fatal("DEEPSEEK_API_KEY environment variable not set")
	}

	// Create AI service
	aiService, err := ai.NewService(ai.ClientConfig{
		APIKey:     cfg.DeepSeek.APIKey,
		BaseURL:    cfg.DeepSeek.BaseURL,
		Model:      cfg.DeepSeek.Model,
		Timeout:    cfg.DeepSeek.Timeout,
		MaxRetries: cfg.DeepSeek.MaxRetries,
	})
	if err != nil {
		log.Fatalf("Failed to create AI service: %v", err)
	}
	defer aiService.Close()

	ctx := context.Background()

	// Example 1: Generate Narrative
	fmt.Println("=== Example 1: Generate Narrative ===")
	narrativeResp, err := aiService.GenerateNarrative(ctx, &ai.NarrativeRequest{
		PlayerInput:    "I want to search the ancient temple for hidden passages",
		Location:       "Ancient Temple of the Moon",
		PartyStatus:    "Full health, cautious after recent trap",
		RecentEvents:   "Defeated stone guardians, found mysterious inscription",
		DMStyle:        "Narrative-focused",
		NarrativeVoice: "Third-person omniscient",
		HumorLevel:     "Light",
		DetailLevel:    "Descriptive",
	})
	if err != nil {
		log.Fatalf("Failed to generate narrative: %v", err)
	}
	fmt.Printf("\nNarrative:\n%s\n\n", narrativeResp.Narrative)
	fmt.Printf("Tokens: %d, Cost: $%.4f, Time: %v\n\n",
		narrativeResp.TokensUsed, narrativeResp.Cost, narrativeResp.ProcessingTime)

	// Example 2: Generate NPC Dialogue
	fmt.Println("=== Example 2: Generate NPC Dialogue ===")
	dialogueResp, err := aiService.GenerateNPCDialogue(ctx, &ai.NPCDialogueRequest{
		NPCName:           "Grimnar Ironforge",
		NPCRace:           "Dwarf",
		NPCClass:          "Blacksmith",
		PersonalityTraits: "Gruff exterior but kind-hearted, takes pride in his work",
		Background:        "Former adventurer who settled down to run the town smithy",
		Motivations:       "Craft legendary weapons, protect the town",
		SpeechPattern:     "Gruff, uses dwarven accent, speaks plainly",
		EmotionalState:    "Busy but friendly",
		Knowledge:         "Expert in weapons, armor, and metalworking",
		Relationship:      "Friendly acquaintance, respects adventurers",
		SpeakerName:       "Adventurer",
		PlayerMessage:     "Can you repair my sword? It was damaged fighting the stone guardians.",
		Context:           "In the smithy, morning, sound of hammer on anvil",
	})
	if err != nil {
		log.Fatalf("Failed to generate dialogue: %v", err)
	}
	fmt.Printf("\nGrimnar says:\n%s\n\n", dialogueResp.Dialogue)
	fmt.Printf("Tokens: %d, Cost: $%.4f, Time: %v\n\n",
		dialogueResp.TokensUsed, dialogueResp.Cost, dialogueResp.ProcessingTime)

	// Example 3: Interpret Dice Roll
	fmt.Println("=== Example 3: Interpret Dice Roll ===")
	diceResp, err := aiService.InterpretDiceRoll(ctx, &ai.DiceInterpretationRequest{
		RollType:      "ability_check",
		CharacterName: "Elara the Rogue",
		Skill:         "Stealth",
		Roll:          18,
		Modifier:      5,
		Total:         23,
		DC:            15,
		Outcome:       "success",
		Context:       "Sneaking past guards in the castle courtyard at night",
	})
	if err != nil {
		log.Fatalf("Failed to interpret dice roll: %v", err)
	}
	fmt.Printf("\nDice Interpretation:\n%s\n\n", diceResp.Interpretation)
	fmt.Printf("Tokens: %d, Cost: $%.4f, Time: %v\n\n",
		diceResp.TokensUsed, diceResp.Cost, diceResp.ProcessingTime)

	// Example 4: Streaming Narrative
	fmt.Println("=== Example 4: Streaming Narrative ===")
	fmt.Print("Streaming response: ")

	textChan, errChan := aiService.StreamNarrative(ctx, &ai.NarrativeRequest{
		PlayerInput:    "I charge at the dragon with my sword raised!",
		Location:       "Dragon's Lair - volcanic cavern",
		PartyStatus:    "Ready for combat, buffed with protection spells",
		RecentEvents:   "Dragon just woke up and roared",
		DMStyle:        "Dramatic and intense",
		NarrativeVoice: "Third-person",
		HumorLevel:     "None",
		DetailLevel:    "Descriptive",
	})

	for {
		select {
		case text, ok := <-textChan:
			if !ok {
				fmt.Println("\n\nStreaming complete!")
				return
			}
			fmt.Print(text)
		case err := <-errChan:
			if err != nil {
				log.Fatalf("\nStreaming error: %v", err)
			}
		}
	}
}

// Helper function to check if running in example mode
func init() {
	if len(os.Args) > 1 && os.Args[1] == "example" {
		fmt.Println("Running AI Usage Examples")
		fmt.Println("Make sure DEEPSEEK_API_KEY is set in your environment")
		fmt.Println()
	}
}
