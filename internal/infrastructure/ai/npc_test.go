package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

func toblen() *models.NPC {
	return &models.NPC{
		NPCID: "npc1", CampaignID: "camp1",
		Name: "Toblen Stonehill", Role: "innkeeper", Race: "human",
		Location: "the Stonehill Inn", Status: models.NPCAlive,
		Personality: "anxious, talkative", Voice: "quick, hushed",
		Knowledge: []string{"the road south is watched by Redbrands"},
	}
}

// The gap this closes: every dialogue call used to invent the NPC from loose
// strings the caller supplied, so the same innkeeper was a different person in
// every scene and had never met the party before.
func TestNPCDialogueCarriesTheNPCsMemory(t *testing.T) {
	npc := toblen()
	npc.Meet()
	npc.Remember(models.NPCMemory{
		Summary: "Thistle paid for a round for the whole common room",
		Actor:   "Thistle", Outcome: models.OutcomeGenerous,
	})

	service, stub := NewStubService("\"Back again! Sit, sit.\"")
	_, err := service.GenerateNPCDialogue(context.Background(), &NPCDialogueRequest{
		NPC: npc, SpeakerName: "Thistle", PlayerMessage: "What's the news?",
	})
	if err != nil {
		t.Fatalf("GenerateNPCDialogue: %v", err)
	}

	prompt := stub.LastPrompt()
	for _, want := range []string{
		"Toblen Stonehill", "anxious, talkative",
		"Thistle paid for a round", "friendly",
		"Redbrands",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt is missing %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "{{") {
		t.Errorf("the prompt has unsubstituted placeholders:\n%s", prompt)
	}
}

// A first meeting has to say so, or the model invents a shared history.
func TestAFirstMeetingSaysSo(t *testing.T) {
	service, stub := NewStubService("\"Can I help you?\"")
	_, err := service.GenerateNPCDialogue(context.Background(), &NPCDialogueRequest{
		NPC: toblen(), SpeakerName: "Thistle", PlayerMessage: "Hello.",
	})
	if err != nil {
		t.Fatalf("GenerateNPCDialogue: %v", err)
	}
	if !strings.Contains(stub.LastPrompt(), "never met") {
		t.Errorf("a first meeting was not flagged:\n%s", stub.LastPrompt())
	}
}

// An NPC who has been wronged must sound like it. The attitude is the part the
// model is told, because it is the part the rules decided.
func TestAHostileNPCIsDescribedAsHostile(t *testing.T) {
	npc := toblen()
	npc.Meet()
	npc.Remember(models.NPCMemory{
		Summary: "the party burned down the stable", Outcome: models.OutcomeAttacked,
	})

	service, stub := NewStubService("\"Get out.\"")
	if _, err := service.GenerateNPCDialogue(context.Background(), &NPCDialogueRequest{
		NPC: npc, SpeakerName: "Thistle", PlayerMessage: "Any rooms?",
	}); err != nil {
		t.Fatalf("GenerateNPCDialogue: %v", err)
	}
	if !strings.Contains(stub.LastPrompt(), string(models.AttitudeHostile)) {
		t.Errorf("the prompt does not say the NPC is hostile:\n%s", stub.LastPrompt())
	}
}

// Dialogue without an NPC is the old bug in a new form.
func TestDialogueRequiresAnNPC(t *testing.T) {
	service, stub := NewStubService("unused")
	if _, err := service.GenerateNPCDialogue(context.Background(), &NPCDialogueRequest{
		SpeakerName: "Thistle", PlayerMessage: "Hello.",
	}); err == nil {
		t.Error("dialogue without an NPC should be refused")
	}
	if len(stub.Requests) != 0 {
		t.Error("the provider was called with no NPC to speak as")
	}
}

// The NPC must not narrate or decide: the same boundary every other prompt has.
func TestNPCPromptForbidsNarrating(t *testing.T) {
	service, stub := NewStubService("\"Hm.\"")
	if _, err := service.GenerateNPCDialogue(context.Background(), &NPCDialogueRequest{
		NPC: toblen(), SpeakerName: "Thistle", PlayerMessage: "Hello.",
	}); err != nil {
		t.Fatalf("GenerateNPCDialogue: %v", err)
	}

	system := strings.ToLower(stub.Requests[0].Messages[0].Content)
	for _, want := range []string{"do not describe the scene", "do not resolve"} {
		if !strings.Contains(system, want) {
			t.Errorf("the system prompt does not say %q", want)
		}
	}
}
