package models

import (
	"strings"
	"testing"
	"time"
)

func innkeeper() *NPC {
	return &NPC{
		NPCID: "npc1", CampaignID: "camp1",
		Name: "Toblen Stonehill", Role: "innkeeper",
		Location: "the Stonehill Inn",
	}
}

// The three attitudes are 5e's, and everything social hangs off which one an
// NPC is in.
func TestAttitudeBands(t *testing.T) {
	cases := map[int]NPCAttitude{
		-100: AttitudeHostile, -50: AttitudeHostile, -25: AttitudeHostile,
		-24: AttitudeIndifferent, 0: AttitudeIndifferent, 24: AttitudeIndifferent,
		25: AttitudeFriendly, 60: AttitudeFriendly, 100: AttitudeFriendly,
	}
	for disposition, want := range cases {
		if got := AttitudeFor(disposition); got != want {
			t.Errorf("AttitudeFor(%d) = %q, want %q", disposition, got, want)
		}
	}

	// A fresh NPC has met nobody and owes them nothing.
	if got := innkeeper().Attitude(); got != AttitudeIndifferent {
		t.Errorf("a new NPC is %q, want indifferent", got)
	}
}

// A friendlier NPC is easier to sway. The direction is the part that must not
// be backwards.
func TestAttitudeShiftsSocialDifficulty(t *testing.T) {
	friendly := AttitudeFriendly.SocialCheckModifier()
	indifferent := AttitudeIndifferent.SocialCheckModifier()
	hostile := AttitudeHostile.SocialCheckModifier()

	if !(friendly < indifferent && indifferent < hostile) {
		t.Errorf("modifiers do not order friendly<indifferent<hostile: %d, %d, %d",
			friendly, indifferent, hostile)
	}
	if indifferent != 0 {
		t.Errorf("an indifferent NPC shifts the DC by %d, want 0", indifferent)
	}
}

// Disposition moves by fixed amounts from a closed list, never by a number the
// narrator picked: "the innkeeper now likes you 40% more" is not something a
// language model should be able to assert.
func TestInteractionOutcomesMoveDispositionByFixedAmounts(t *testing.T) {
	if len(InteractionOutcomes) == 0 {
		t.Fatal("there are no recognised outcomes")
	}

	for _, outcome := range InteractionOutcomes {
		if !outcome.Valid() {
			t.Errorf("%q is listed but not valid", outcome)
		}
	}
	if OutcomeNone.Impact() != 0 {
		t.Errorf("OutcomeNone shifts disposition by %d", OutcomeNone.Impact())
	}

	// Direction is the thing to get right.
	for _, good := range []InteractionOutcome{OutcomeHelped, OutcomeKeptPromise, OutcomeSavedLife} {
		if good.Impact() <= 0 {
			t.Errorf("%q has impact %d, want positive", good, good.Impact())
		}
	}
	for _, bad := range []InteractionOutcome{OutcomeInsulted, OutcomeThreatened, OutcomeAttacked, OutcomeBrokePromise} {
		if bad.Impact() >= 0 {
			t.Errorf("%q has impact %d, want negative", bad, bad.Impact())
		}
	}

	// Attacking someone should cost more than being rude to them.
	if OutcomeAttacked.Impact() >= OutcomeInsulted.Impact() {
		t.Error("an attack is no worse than an insult")
	}

	// An invented outcome moves nothing rather than breaking the interaction.
	invented := InteractionOutcome("gave_them_a_really_meaningful_look")
	if invented.Valid() || invented.Impact() != 0 {
		t.Errorf("an invented outcome was accepted with impact %d", invented.Impact())
	}
}

func TestRememberingAnInteractionMovesTheNeedle(t *testing.T) {
	npc := innkeeper()
	before := npc.Disposition

	npc.Remember(NPCMemory{
		Summary: "Thistle paid for a round for the whole common room",
		Actor:   "Thistle", Outcome: OutcomeGenerous,
	})

	if npc.Disposition <= before {
		t.Errorf("disposition is %d, was %d", npc.Disposition, before)
	}
	if len(npc.Memories) != 1 {
		t.Fatalf("recorded %d memories, want one", len(npc.Memories))
	}
	held := npc.Memories[0]
	if held.Impact != OutcomeGenerous.Impact() {
		t.Errorf("impact recorded as %d, want %d", held.Impact, OutcomeGenerous.Impact())
	}
	if held.At.IsZero() {
		t.Error("the memory has no timestamp")
	}
	// Who did it matters: an NPC can like the bard and distrust the paladin.
	if held.Actor != "Thistle" {
		t.Errorf("actor = %q", held.Actor)
	}
}

// Disposition is a scale, not an accumulator: no amount of ale buys more than
// devotion, and no amount of rudeness digs below hatred.
func TestDispositionIsClamped(t *testing.T) {
	adored := innkeeper()
	for i := 0; i < 50; i++ {
		adored.Remember(NPCMemory{Summary: "helped again", Outcome: OutcomeSavedLife})
	}
	if adored.Disposition != MaxDisposition {
		t.Errorf("disposition = %d, want it clamped to %d", adored.Disposition, MaxDisposition)
	}

	loathed := innkeeper()
	for i := 0; i < 50; i++ {
		loathed.Remember(NPCMemory{Summary: "attacked again", Outcome: OutcomeAttacked})
	}
	if loathed.Disposition != MinDisposition {
		t.Errorf("disposition = %d, want it clamped to %d", loathed.Disposition, MinDisposition)
	}
}

// An NPC met fifty times must not carry fifty memories into a prompt, and must
// not forget the one that mattered either.
func TestMemoriesArePrunedButTheImportantOnesSurvive(t *testing.T) {
	npc := innkeeper()

	// The thing they will never forget, first, so recency cannot save it.
	npc.Remember(NPCMemory{Summary: "the party killed my brother", Outcome: OutcomeAttacked})

	for i := 0; i < 60; i++ {
		npc.Remember(NPCMemory{Summary: "sold them a room for the night", Outcome: OutcomeNone})
	}

	if len(npc.Memories) > MaxNPCMemories {
		t.Errorf("holding %d memories, want at most %d", len(npc.Memories), MaxNPCMemories)
	}

	var remembered bool
	for _, m := range npc.Memories {
		if strings.Contains(m.Summary, "killed my brother") {
			remembered = true
		}
	}
	if !remembered {
		t.Error("the NPC forgot that the party killed their brother")
	}

	// The most recent trivia is still there: it is the middle that goes.
	newest := npc.Memories[len(npc.Memories)-1]
	if !strings.Contains(newest.Summary, "sold them a room") {
		t.Errorf("the newest memory is %q", newest.Summary)
	}
}

// Meeting someone is itself a fact worth keeping.
func TestMeetingIsRecorded(t *testing.T) {
	npc := innkeeper()
	if npc.TimesMet != 0 || !npc.FirstMet.IsZero() {
		t.Fatal("a new NPC has already met the party")
	}

	npc.Meet()
	if npc.TimesMet != 1 {
		t.Errorf("TimesMet = %d, want 1", npc.TimesMet)
	}
	first := npc.FirstMet
	if first.IsZero() || first.Location() != time.UTC {
		t.Errorf("FirstMet = %v, want a UTC timestamp", first)
	}

	npc.Meet()
	if npc.TimesMet != 2 {
		t.Errorf("TimesMet = %d, want 2", npc.TimesMet)
	}
	if !npc.FirstMet.Equal(first) {
		t.Error("the second meeting overwrote the first")
	}
	if !npc.LastSeen.After(first) && !npc.LastSeen.Equal(first) {
		t.Error("LastSeen did not advance")
	}
}

// The dead do not chat.
func TestADeadNPCCannotBeTalkedTo(t *testing.T) {
	npc := innkeeper()
	if ok, _ := npc.CanConverse(); !ok {
		t.Error("a living NPC refused to talk")
	}

	npc.Status = NPCDead
	ok, reason := npc.CanConverse()
	if ok {
		t.Error("a dead NPC held a conversation")
	}
	if !strings.Contains(strings.ToLower(reason), "dead") {
		t.Errorf("reason = %q", reason)
	}
}

// Validate is the NPC counterpart of Monster.Validate and ValidateSheet.
func TestNPCValidation(t *testing.T) {
	if err := innkeeper().Validate(); err != nil {
		t.Fatalf("a well-formed NPC was rejected: %v", err)
	}

	cases := map[string]func(*NPC){
		"no name":               func(n *NPC) { n.Name = "" },
		"no campaign":           func(n *NPC) { n.CampaignID = "" },
		"disposition too high":  func(n *NPC) { n.Disposition = 500 },
		"disposition too low":   func(n *NPC) { n.Disposition = -500 },
		"an invented status":    func(n *NPC) { n.Status = "sleepy" },
		"an invented attitude":  func(n *NPC) { n.Memories = []NPCMemory{{Summary: "x", Outcome: "shrugged"}} },
		"a memory with no text": func(n *NPC) { n.Memories = []NPCMemory{{Outcome: OutcomeHelped}} },
	}
	for name, mutate := range cases {
		npc := innkeeper()
		mutate(npc)
		if err := npc.Validate(); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// The prompt block is what the model actually sees, so it has to read as
// sentences and never as an empty field inviting invention.
func TestMemoryBlockReadsAsSentences(t *testing.T) {
	fresh := innkeeper().MemoryBlock()
	if strings.TrimSpace(fresh) == "" {
		t.Fatal("a new NPC produced an empty memory block")
	}
	if !strings.Contains(fresh, "never met") {
		t.Errorf("a first meeting does not say so: %q", fresh)
	}

	npc := innkeeper()
	npc.Meet()
	npc.Remember(NPCMemory{
		Summary: "Thistle paid for a round", Actor: "Thistle", Outcome: OutcomeGenerous,
	})

	block := npc.MemoryBlock()
	for _, want := range []string{"Thistle paid for a round", string(npc.Attitude())} {
		if !strings.Contains(block, want) {
			t.Errorf("the block is missing %q:\n%s", want, block)
		}
	}
	if strings.Contains(block, "never met") {
		t.Error("an NPC who has met the party says they have not")
	}
}
