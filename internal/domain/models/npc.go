package models

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// NPC is a person in the world who remembers the party.
//
// It is deliberately not a Character. A Character must be a legal 5e sheet --
// ValidateSheet demands a race, a class and a subclass taken at the right level
// -- and a tavern keeper is none of those things. What an NPC needs instead is
// the thing neither a Character nor a Monster has: a memory of the party and a
// disposition towards them.
//
// An NPC who needs to fight links to a Monster statblock through StatblockID
// rather than carrying combat rules of its own.
type NPC struct {
	ID         string `json:"id,omitempty" bson:"_id,omitempty"`
	NPCID      string `json:"npc_id" bson:"npc_id"`
	CampaignID string `json:"campaign_id" bson:"campaign_id"`

	Name     string `json:"name" bson:"name"`
	Role     string `json:"role,omitempty" bson:"role,omitempty"`
	Race     string `json:"race,omitempty" bson:"race,omitempty"`
	Location string `json:"location,omitempty" bson:"location,omitempty"`

	// How they come across. These feed the dialogue prompt directly, which is
	// what stops the same NPC sounding like a different person every scene.
	Appearance  string   `json:"appearance,omitempty" bson:"appearance,omitempty"`
	Personality string   `json:"personality,omitempty" bson:"personality,omitempty"`
	Voice       string   `json:"voice,omitempty" bson:"voice,omitempty"`
	Mannerisms  string   `json:"mannerisms,omitempty" bson:"mannerisms,omitempty"`
	Motivations string   `json:"motivations,omitempty" bson:"motivations,omitempty"`
	Knowledge   []string `json:"knowledge,omitempty" bson:"knowledge,omitempty"`

	// Disposition towards the party, MinDisposition to MaxDisposition.
	//
	// One number for the whole party rather than one per character: 5e's own
	// social rules treat an NPC as having a single attitude towards the people
	// in front of them. Who did what is kept on each memory instead, so the
	// dialogue can still say she will not look at Thistle.
	Disposition int `json:"disposition" bson:"disposition"`

	Memories []NPCMemory `json:"memories,omitempty" bson:"memories,omitempty"`

	Status      NPCStatus `json:"status" bson:"status"`
	StatblockID string    `json:"statblock_id,omitempty" bson:"statblock_id,omitempty"`

	FirstMet time.Time `json:"first_met,omitempty" bson:"first_met,omitempty"`
	LastSeen time.Time `json:"last_seen,omitempty" bson:"last_seen,omitempty"`
	TimesMet int       `json:"times_met" bson:"times_met"`

	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

// The bounds of disposition. It is a scale, not an accumulator: no amount of
// free ale buys more than devotion.
const (
	MinDisposition = -100
	MaxDisposition = 100
)

// NPCStatus is where this person is in the world.
type NPCStatus string

const (
	NPCAlive   NPCStatus = "alive"
	NPCDead    NPCStatus = "dead"
	NPCMissing NPCStatus = "missing"
)

// NPCStatuses lists every recognised status.
var NPCStatuses = []NPCStatus{NPCAlive, NPCDead, NPCMissing}

// Valid reports whether s is a recognised status. The empty status is alive,
// so an NPC created without one is not immediately a corpse.
func (s NPCStatus) Valid() bool {
	if s == "" {
		return true
	}
	for _, known := range NPCStatuses {
		if s == known {
			return true
		}
	}
	return false
}

// NPCAttitude is 5e's social-interaction attitude.
//
// Three, and only three: the rules give Hostile, Indifferent and Friendly, and
// inventing a fourth would put a number in the DM's mouth that no table uses.
type NPCAttitude string

const (
	AttitudeHostile     NPCAttitude = "hostile"
	AttitudeIndifferent NPCAttitude = "indifferent"
	AttitudeFriendly    NPCAttitude = "friendly"
)

// The disposition at which an attitude changes.
const (
	HostileBelow  = -25
	FriendlyAbove = 25
)

// AttitudeFor maps a disposition onto an attitude.
func AttitudeFor(disposition int) NPCAttitude {
	switch {
	case disposition <= HostileBelow:
		return AttitudeHostile
	case disposition >= FriendlyAbove:
		return AttitudeFriendly
	default:
		return AttitudeIndifferent
	}
}

// Attitude is how this NPC currently regards the party.
func (n *NPC) Attitude() NPCAttitude { return AttitudeFor(n.Disposition) }

// SocialCheckModifier shifts the difficulty of persuading this NPC.
//
// The DMG resolves social interaction with a table of absolute DCs per attitude
// and per how much the request asks of the creature. This is deliberately not
// that table: it is a shift applied to whatever DC the DM set, because the
// table's exact numbers are not reproduced here and a rules table that is
// nearly right is worse than one that is plainly a simplification.
//
// What is certain, and what this encodes, is the direction and the ordering: a
// friendly creature is easier to sway than an indifferent one, which is easier
// than a hostile one.
func (a NPCAttitude) SocialCheckModifier() int {
	switch a {
	case AttitudeFriendly:
		return -5
	case AttitudeHostile:
		return 5
	default:
		return 0
	}
}

// InteractionOutcome is a recognised thing the party did to an NPC.
//
// It is a closed list for the same reason AdvantageReason is: disposition is a
// mechanical value, and "the innkeeper now likes you forty percent more" is not
// something a language model should be able to assert. The model classifies
// what happened; this table decides what it costs.
type InteractionOutcome string

const (
	OutcomeNone InteractionOutcome = ""

	OutcomeHelped      InteractionOutcome = "helped"
	OutcomeKeptPromise InteractionOutcome = "kept_promise"
	OutcomeGenerous    InteractionOutcome = "paid_generously"
	OutcomeSavedLife   InteractionOutcome = "saved_life"
	OutcomeDefended    InteractionOutcome = "defended_them"

	OutcomeInsulted     InteractionOutcome = "insulted"
	OutcomeThreatened   InteractionOutcome = "threatened"
	OutcomeStoleFrom    InteractionOutcome = "stole_from"
	OutcomeBrokePromise InteractionOutcome = "broke_promise"
	OutcomeAttacked     InteractionOutcome = "attacked"
	OutcomeKilledAlly   InteractionOutcome = "killed_someone_they_loved"
)

// InteractionOutcomes lists every recognised outcome.
var InteractionOutcomes = []InteractionOutcome{
	OutcomeNone,
	OutcomeHelped, OutcomeKeptPromise, OutcomeGenerous, OutcomeSavedLife, OutcomeDefended,
	OutcomeInsulted, OutcomeThreatened, OutcomeStoleFrom, OutcomeBrokePromise,
	OutcomeAttacked, OutcomeKilledAlly,
}

// interactionImpact is what each outcome moves disposition by.
var interactionImpact = map[InteractionOutcome]int{
	OutcomeHelped:      10,
	OutcomeKeptPromise: 15,
	OutcomeGenerous:    5,
	OutcomeDefended:    20,
	OutcomeSavedLife:   30,

	OutcomeInsulted:     -10,
	OutcomeThreatened:   -15,
	OutcomeStoleFrom:    -20,
	OutcomeBrokePromise: -25,
	OutcomeAttacked:     -40,
	OutcomeKilledAlly:   -60,
}

// Valid reports whether o is a recognised outcome.
func (o InteractionOutcome) Valid() bool {
	for _, known := range InteractionOutcomes {
		if o == known {
			return true
		}
	}
	return false
}

// Impact is what this outcome moves disposition by. An unrecognised outcome
// moves nothing, rather than breaking the interaction: a model that invents one
// should change no numbers.
func (o InteractionOutcome) Impact() int { return interactionImpact[o] }

// SignificantImpact is the size of memory an NPC never forgets.
const SignificantImpact = 20

// Memory limits. An NPC met fifty times must not carry fifty memories into a
// prompt -- the token budget is finite -- but must not forget the one that
// mattered either.
const (
	MaxNPCMemories         = 12
	MaxSignificantMemories = 6
)

// NPCMemory is one thing this NPC remembers the party doing.
type NPCMemory struct {
	Summary string             `json:"summary" bson:"summary"`
	Actor   string             `json:"actor,omitempty" bson:"actor,omitempty"`
	Outcome InteractionOutcome `json:"outcome,omitempty" bson:"outcome,omitempty"`

	// Impact is the disposition change this caused, recorded so the number can
	// be audited later rather than only its effect being visible.
	Impact int `json:"impact" bson:"impact"`

	SessionID string    `json:"session_id,omitempty" bson:"session_id,omitempty"`
	At        time.Time `json:"at" bson:"at"`
}

// Significant reports whether this is a memory the NPC never forgets.
func (m NPCMemory) Significant() bool {
	return m.Impact >= SignificantImpact || m.Impact <= -SignificantImpact
}

// Meet records that the party has spoken with this NPC.
func (n *NPC) Meet() {
	now := time.Now().UTC()
	if n.FirstMet.IsZero() {
		n.FirstMet = now
	}
	n.LastSeen = now
	n.TimesMet++
}

// Remember records an interaction and moves disposition by what it was worth.
//
// The caller supplies the outcome; the impact comes from the table, so a
// summary and an outcome cannot disagree about what something cost.
func (n *NPC) Remember(memory NPCMemory) {
	memory.Impact = memory.Outcome.Impact()
	if memory.At.IsZero() {
		memory.At = time.Now().UTC()
	}

	n.Disposition = clampDisposition(n.Disposition + memory.Impact)
	n.Memories = append(n.Memories, memory)
	n.pruneMemories()
}

func clampDisposition(value int) int {
	switch {
	case value > MaxDisposition:
		return MaxDisposition
	case value < MinDisposition:
		return MinDisposition
	default:
		return value
	}
}

// pruneMemories keeps the list short without losing what matters.
//
// Recency alone would let an NPC forget that the party killed their brother
// after a dozen room rentals. Significance alone would let a decades-old grudge
// crowd out this morning's conversation. So both are kept: the most significant
// memories, and the most recent ones, in the order they happened.
func (n *NPC) pruneMemories() {
	if len(n.Memories) <= MaxNPCMemories {
		return
	}

	keep := make(map[int]bool, MaxNPCMemories)

	// The heaviest memories first, ties broken towards the older one, because
	// the first betrayal is the one that defines a relationship.
	significant := make([]int, 0, len(n.Memories))
	for i, m := range n.Memories {
		if m.Significant() {
			significant = append(significant, i)
		}
	}
	sort.SliceStable(significant, func(a, b int) bool {
		return abs(n.Memories[significant[a]].Impact) > abs(n.Memories[significant[b]].Impact)
	})
	for i, index := range significant {
		if i >= MaxSignificantMemories {
			break
		}
		keep[index] = true
	}

	// Then the most recent, until the list is full.
	for i := len(n.Memories) - 1; i >= 0 && len(keep) < MaxNPCMemories; i-- {
		keep[i] = true
	}

	kept := make([]NPCMemory, 0, len(keep))
	for i, m := range n.Memories {
		if keep[i] {
			kept = append(kept, m)
		}
	}
	n.Memories = kept
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// CanConverse reports whether this NPC can be spoken to, and why not.
func (n *NPC) CanConverse() (bool, string) {
	switch n.Status {
	case NPCDead:
		return false, fmt.Sprintf("%s is dead", n.Name)
	case NPCMissing:
		return false, fmt.Sprintf("%s is nowhere to be found", n.Name)
	}
	return true, ""
}

// Validate reports an NPC that cannot mean anything.
//
// It is the NPC counterpart of Monster.Validate and ValidateSheet.
func (n *NPC) Validate() error {
	var problems []string

	if strings.TrimSpace(n.Name) == "" {
		return Invalid("npc name is required")
	}
	if strings.TrimSpace(n.CampaignID) == "" {
		problems = append(problems, "campaign_id is required")
	}
	if n.Disposition < MinDisposition || n.Disposition > MaxDisposition {
		problems = append(problems, fmt.Sprintf(
			"disposition %d is outside %d to %d", n.Disposition, MinDisposition, MaxDisposition))
	}
	if !n.Status.Valid() {
		problems = append(problems, fmt.Sprintf("unknown status %q", n.Status))
	}
	if n.TimesMet < 0 {
		problems = append(problems, "times_met is negative")
	}

	for i, m := range n.Memories {
		if strings.TrimSpace(m.Summary) == "" {
			problems = append(problems, fmt.Sprintf("memory %d has no summary", i+1))
		}
		if !m.Outcome.Valid() {
			problems = append(problems, fmt.Sprintf("memory %d has unknown outcome %q", i+1, m.Outcome))
		}
	}

	if len(problems) > 0 {
		return Invalid("npc %s: %s", n.Name, strings.Join(problems, "; "))
	}
	return nil
}

// MemoryBlock renders what this NPC knows about the party, for the prompt.
//
// Never empty: a blank value in a prompt reads as an invitation to invent a
// history, which is the precise failure this whole type exists to prevent.
func (n *NPC) MemoryBlock() string {
	var b strings.Builder

	if n.TimesMet == 0 {
		b.WriteString("You have never met these people before. ")
		b.WriteString("You know nothing about them beyond what you can see.")
		return b.String()
	}

	fmt.Fprintf(&b, "You have met these people %s. Your attitude towards them is %s.",
		timesText(n.TimesMet), n.Attitude())

	if len(n.Memories) > 0 {
		b.WriteString("\n\nWhat you remember, oldest first:")
		for _, m := range n.Memories {
			b.WriteString("\n- ")
			if actor := strings.TrimSpace(m.Actor); actor != "" {
				fmt.Fprintf(&b, "%s: ", actor)
			}
			b.WriteString(strings.TrimSpace(m.Summary))
		}
	}
	return b.String()
}

func timesText(n int) string {
	switch n {
	case 1:
		return "once before"
	case 2:
		return "twice before"
	default:
		return fmt.Sprintf("%d times before", n)
	}
}
