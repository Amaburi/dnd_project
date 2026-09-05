package models

import (
	"fmt"
	"strings"
	"time"
)

// Location is a place in the world and, more importantly, the things in it that
// a player can do something to.
//
// Before this, the only place in the game was SessionLocation.CurrentLocation,
// a bare string. So the DM would narrate a locked chest in the corner and
// nothing recorded that the chest existed; the next turn a player said "I open
// the chest" and there was no chest to open. It is the same failure NPCs and
// plot threads had -- the DM mentions something and nothing remembers it.
type Location struct {
	ID         string `json:"id,omitempty" bson:"_id,omitempty"`
	LocationID string `json:"location_id" bson:"location_id"`
	CampaignID string `json:"campaign_id" bson:"campaign_id"`

	Name        string `json:"name" bson:"name"`
	Description string `json:"description,omitempty" bson:"description,omitempty"`

	// Ambience is what the place sounds and smells like: the material a
	// narrator needs so every room does not read as "a room".
	Ambience string `json:"ambience,omitempty" bson:"ambience,omitempty"`

	Lighting Lighting `json:"lighting,omitempty" bson:"lighting,omitempty"`

	Interactables []Interactable `json:"interactables,omitempty" bson:"interactables,omitempty"`
	Exits         []Exit         `json:"exits,omitempty" bson:"exits,omitempty"`

	// NPCIDs are who is usually here. Monsters live in encounters.
	NPCIDs []string `json:"npc_ids,omitempty" bson:"npc_ids,omitempty"`

	Visited   bool      `json:"visited" bson:"visited"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

// Lighting is how well a place can be seen, which 5e makes a rules question
// rather than a mood one.
type Lighting string

const (
	LightingBright Lighting = "bright"
	LightingDim    Lighting = "dim"
	LightingDark   Lighting = "dark"
)

// Lightings lists the levels from brightest to darkest.
var Lightings = []Lighting{LightingBright, LightingDim, LightingDark}

// Valid reports whether l is recognised. Empty means bright.
func (l Lighting) Valid() bool {
	if l == "" {
		return true
	}
	for _, known := range Lightings {
		if l == known {
			return true
		}
	}
	return false
}

// PerceptionMode is the roll mode for noticing something by sight.
//
// Dim light is lightly obscured, which is disadvantage on Wisdom (Perception)
// checks that rely on sight. Darkness is heavily obscured and blinds outright,
// so it returns disadvantage here but BlindsSight is the question that matters.
func (l Lighting) PerceptionMode() RollMode {
	switch l {
	case LightingDim, LightingDark:
		return RollDisadvantage
	default:
		return RollNormal
	}
}

// BlindsSight reports whether a creature simply cannot see.
//
// Darkness is heavily obscured: a creature trying to see into it has the
// blinded condition, and a blinded creature automatically fails any check that
// requires sight rather than rolling one at disadvantage.
func (l Lighting) BlindsSight() bool { return l == LightingDark }

// SeenWithDarkvision is how bright this looks to a creature with darkvision.
//
// Darkvision moves the world one step brighter within its range: darkness
// becomes dim, dim becomes bright. Beyond the range it does nothing. (RAW also
// says colour is lost in that darkness, which is a narration detail rather than
// a roll, so it is left to the prose.)
func (l Lighting) SeenWithDarkvision(rangeFeet, distanceFeet int) Lighting {
	if rangeFeet <= 0 || distanceFeet > rangeFeet {
		return l
	}
	switch l {
	case LightingDark:
		return LightingDim
	case LightingDim:
		return LightingBright
	default:
		return l
	}
}

// InteractionKind is something a player can do to a thing.
//
// A closed list so the parser chooses from what the object actually supports,
// the same way it chooses from the weapons a character is carrying. "I vibe
// with the tapestry" is not an action the world has.
type InteractionKind string

const (
	InteractExamine InteractionKind = "examine"
	InteractSearch  InteractionKind = "search"
	InteractOpen    InteractionKind = "open"
	InteractClose   InteractionKind = "close"
	InteractUnlock  InteractionKind = "unlock"
	InteractMove    InteractionKind = "move"
	InteractClimb   InteractionKind = "climb"
	InteractPull    InteractionKind = "pull"
	InteractPush    InteractionKind = "push"
	InteractRead    InteractionKind = "read"
	InteractTake    InteractionKind = "take"
	InteractBreak   InteractionKind = "break"
	InteractListen  InteractionKind = "listen"
	InteractTouch   InteractionKind = "touch"
)

// InteractionKinds lists everything a player may do to a thing.
var InteractionKinds = []InteractionKind{
	InteractExamine, InteractSearch, InteractOpen, InteractClose, InteractUnlock,
	InteractMove, InteractClimb, InteractPull, InteractPush, InteractRead,
	InteractTake, InteractBreak, InteractListen, InteractTouch,
}

// Valid reports whether k is recognised.
func (k InteractionKind) Valid() bool {
	for _, known := range InteractionKinds {
		if k == known {
			return true
		}
	}
	return false
}

// InteractableState is the condition a thing is in.
type InteractableState string

const (
	StateIntact   InteractableState = "intact"
	StateLocked   InteractableState = "locked"
	StateUnlocked InteractableState = "unlocked"
	StateOpen     InteractableState = "open"
	StateSearched InteractableState = "searched"
	StateBroken   InteractableState = "broken"
)

// InteractableStates lists every recognised state. Empty means intact.
var InteractableStates = []InteractableState{
	StateIntact, StateLocked, StateUnlocked, StateOpen, StateSearched, StateBroken,
}

// Valid reports whether s is recognised.
func (s InteractableState) Valid() bool {
	if s == "" {
		return true
	}
	for _, known := range InteractableStates {
		if s == known {
			return true
		}
	}
	return false
}

// Interactable is a thing in a place that a player can do something to.
type Interactable struct {
	InteractableID string `json:"interactable_id,omitempty" bson:"interactable_id,omitempty"`
	Name           string `json:"name" bson:"name"`
	Description    string `json:"description,omitempty" bson:"description,omitempty"`

	Interactions []InteractionKind `json:"interactions,omitempty" bson:"interactions,omitempty"`
	State        InteractableState `json:"state,omitempty" bson:"state,omitempty"`

	// Hidden means the party has not noticed this yet.
	//
	// A hidden thing is kept out of the narration prompt entirely rather than
	// sent with an instruction not to mention it. Telling a model "there is a
	// secret door, do not mention it" is how secret doors get mentioned.
	Hidden        bool  `json:"hidden,omitempty" bson:"hidden,omitempty"`
	DiscoverDC    int   `json:"discover_dc,omitempty" bson:"discover_dc,omitempty"`
	DiscoverSkill Skill `json:"discover_skill,omitempty" bson:"discover_skill,omitempty"`

	UnlockDC    int   `json:"unlock_dc,omitempty" bson:"unlock_dc,omitempty"`
	UnlockSkill Skill `json:"unlock_skill,omitempty" bson:"unlock_skill,omitempty"`

	// Reveals is what a successful search turns up, and Contents what taking it
	// yields. Both are the DM's until earned, like Hidden.
	Reveals  string   `json:"reveals,omitempty" bson:"reveals,omitempty"`
	Contents []string `json:"contents,omitempty" bson:"contents,omitempty"`
}

// Allows reports whether this thing supports an interaction.
//
// Examining is always allowed: anything you can see, you can look at. Anything
// else must be listed, so a thing with an empty list can be looked at and no
// more rather than silently permitting everything.
func (i *Interactable) Allows(kind InteractionKind) bool {
	if kind == InteractExamine {
		return true
	}
	// A thing that has a lock can be picked, whether or not anyone remembered
	// to write "unlock" on it. Requiring both is bookkeeping that gets
	// forgotten, and the forgetting looks like a rule.
	if kind == InteractUnlock && (i.State == StateLocked || i.UnlockDC > 0) {
		return true
	}
	for _, allowed := range i.Interactions {
		if allowed == kind {
			return true
		}
	}
	return false
}

// CanOpen reports whether this can be opened as it stands, and why not.
func (i *Interactable) CanOpen() (bool, string) {
	switch i.State {
	case StateLocked:
		return false, fmt.Sprintf("the %s is locked", i.Name)
	case StateBroken:
		return false, fmt.Sprintf("the %s is broken", i.Name)
	case StateOpen:
		return false, fmt.Sprintf("the %s is already open", i.Name)
	}
	return true, ""
}

// Unlock tries a roll against the lock, and reports whether it gave.
func (i *Interactable) Unlock(total int) bool {
	if i.State != StateLocked {
		return true
	}
	if total < i.UnlockDC {
		return false
	}
	i.State = StateUnlocked
	return true
}

// Exit is a way out of a place.
type Exit struct {
	Direction    string `json:"direction" bson:"direction"`
	Description  string `json:"description,omitempty" bson:"description,omitempty"`
	ToLocationID string `json:"to_location_id,omitempty" bson:"to_location_id,omitempty"`

	Locked bool `json:"locked,omitempty" bson:"locked,omitempty"`

	// Hidden is a secret door, and follows the same rule as a hidden thing:
	// it does not reach the prompt until it is found.
	Hidden     bool `json:"hidden,omitempty" bson:"hidden,omitempty"`
	DiscoverDC int  `json:"discover_dc,omitempty" bson:"discover_dc,omitempty"`
}

// VisibleInteractables are the things the party can see.
func (l *Location) VisibleInteractables() []Interactable {
	out := make([]Interactable, 0, len(l.Interactables))
	for _, i := range l.Interactables {
		if !i.Hidden {
			out = append(out, i)
		}
	}
	return out
}

// InteractableNames is the closed list offered to the parser.
func (l *Location) InteractableNames() []string {
	visible := l.VisibleInteractables()
	names := make([]string, 0, len(visible))
	for _, i := range visible {
		names = append(names, i.Name)
	}
	return names
}

// VisibleExits are the ways out the party knows about.
func (l *Location) VisibleExits() []Exit {
	out := make([]Exit, 0, len(l.Exits))
	for _, e := range l.Exits {
		if !e.Hidden {
			out = append(out, e)
		}
	}
	return out
}

// ExitNames is the closed list of ways out, for the parser.
func (l *Location) ExitNames() []string {
	visible := l.VisibleExits()
	names := make([]string, 0, len(visible))
	for _, e := range visible {
		names = append(names, e.Direction)
	}
	return names
}

// Interactable finds a visible thing by the name a player would say.
//
// Hidden things are not findable by name either: the party does not know it is
// there, so neither does the parser, and a lucky guess is not a discovery.
func (l *Location) Interactable(name string) (*Interactable, bool) {
	wanted := strings.ToLower(strings.TrimSpace(name))
	for i := range l.Interactables {
		if l.Interactables[i].Hidden {
			continue
		}
		if strings.ToLower(strings.TrimSpace(l.Interactables[i].Name)) == wanted {
			return &l.Interactables[i], true
		}
	}
	return nil, false
}

// Discover reveals everything a search of this quality turns up.
//
// The engine decides and the narrator is told afterwards, which is the same
// division as everywhere else: a check total is compared with a DC, and what
// clears it is revealed. Nothing here is a judgement call.
//
// A thing that names a skill is only found by that skill; one that names none
// is found by any search.
func (l *Location) Discover(skill Skill, total int) []Interactable {
	var found []Interactable
	for i := range l.Interactables {
		item := &l.Interactables[i]
		if !item.Hidden {
			continue
		}
		if item.DiscoverSkill != "" && item.DiscoverSkill != skill {
			continue
		}
		if total < item.DiscoverDC {
			continue
		}
		item.Hidden = false
		found = append(found, *item)
	}
	return found
}

// DiscoverExits reveals secret doors a search of this quality turns up.
func (l *Location) DiscoverExits(total int) []Exit {
	var found []Exit
	for i := range l.Exits {
		exit := &l.Exits[i]
		if exit.Hidden && total >= exit.DiscoverDC {
			exit.Hidden = false
			found = append(found, *exit)
		}
	}
	return found
}

// SceneBlock renders the place for a prompt.
//
// Only what the party can see. Nothing hidden appears here at all -- not the
// name, not a hint, not a count. That is the entire safety property: a model
// cannot leak what it was never told.
func (l *Location) SceneBlock() string {
	var b strings.Builder

	fmt.Fprintf(&b, "Location: %s", l.Name)
	if description := strings.TrimSpace(l.Description); description != "" {
		fmt.Fprintf(&b, ". %s", description)
	}
	if ambience := strings.TrimSpace(l.Ambience); ambience != "" {
		fmt.Fprintf(&b, " %s", ambience)
	}

	lighting := l.Lighting
	if lighting == "" {
		lighting = LightingBright
	}
	fmt.Fprintf(&b, "\nLight: %s.", lighting)
	if lighting == LightingDim {
		b.WriteString(" Sight-based Perception is at disadvantage here.")
	}
	if lighting == LightingDark {
		b.WriteString(" Anyone without darkvision cannot see at all.")
	}

	if visible := l.VisibleInteractables(); len(visible) > 0 {
		b.WriteString("\nThings here the party can interact with:")
		for _, i := range visible {
			fmt.Fprintf(&b, "\n- %s", i.Name)
			if description := strings.TrimSpace(i.Description); description != "" {
				fmt.Fprintf(&b, ": %s", description)
			}
			if i.State != "" && i.State != StateIntact {
				fmt.Fprintf(&b, " (%s)", i.State)
			}
		}
	}

	if exits := l.VisibleExits(); len(exits) > 0 {
		b.WriteString("\nWays out:")
		for _, e := range exits {
			fmt.Fprintf(&b, "\n- %s", e.Direction)
			if description := strings.TrimSpace(e.Description); description != "" {
				fmt.Fprintf(&b, ": %s", description)
			}
			if e.Locked {
				b.WriteString(" (locked)")
			}
		}
	}
	return b.String()
}

// Validate reports a location that cannot mean anything.
func (l *Location) Validate() error {
	var problems []string

	if strings.TrimSpace(l.Name) == "" {
		return Invalid("a location needs a name")
	}
	if strings.TrimSpace(l.CampaignID) == "" {
		problems = append(problems, "campaign_id is required")
	}
	if !l.Lighting.Valid() {
		problems = append(problems, fmt.Sprintf("unknown lighting %q", l.Lighting))
	}

	for i, item := range l.Interactables {
		label := item.Name
		if strings.TrimSpace(label) == "" {
			problems = append(problems, fmt.Sprintf("interactable %d has no name", i+1))
			continue
		}
		if !item.State.Valid() {
			problems = append(problems, fmt.Sprintf("%s has unknown state %q", label, item.State))
		}
		for _, kind := range item.Interactions {
			if !kind.Valid() {
				problems = append(problems, fmt.Sprintf("%s allows unknown interaction %q", label, kind))
			}
		}
		if item.DiscoverSkill != "" && !item.DiscoverSkill.Valid() {
			problems = append(problems, fmt.Sprintf("%s is found with unknown skill %q", label, item.DiscoverSkill))
		}
		if item.UnlockSkill != "" && !item.UnlockSkill.Valid() {
			problems = append(problems, fmt.Sprintf("%s is opened with unknown skill %q", label, item.UnlockSkill))
		}
	}

	for i, exit := range l.Exits {
		if strings.TrimSpace(exit.Direction) == "" {
			problems = append(problems, fmt.Sprintf("exit %d has no direction", i+1))
		}
	}

	if len(problems) > 0 {
		return Invalid("location %s: %s", l.Name, strings.Join(problems, "; "))
	}
	return nil
}
