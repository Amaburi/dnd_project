package models

import (
	"fmt"
	"sort"
	"strings"
)

// IntentAction is what a player is trying to do, once their sentence has been
// parsed into something the rules engine can act on.
//
// This is a *proposal*, not a decision: the model that produces it only reads
// the sentence, and the engine decides whether the action is legal and what it
// costs. Keeping the two apart is what stops a persuasive player talking the
// narrator into free hit points.
type IntentAction string

const (
	IntentAttack      IntentAction = "attack"
	IntentSkillCheck  IntentAction = "skill_check"
	IntentSavingThrow IntentAction = "saving_throw"
	IntentCastSpell   IntentAction = "cast_spell"
	IntentUseItem     IntentAction = "use_item"
	IntentMove        IntentAction = "move"
	IntentTalk        IntentAction = "talk"

	// IntentInteract is doing something to a thing in the room -- searching a
	// desk, opening a chest, pulling a lever.
	IntentInteract IntentAction = "interact"

	// IntentNarrative covers anything with no mechanical consequence -- looking
	// around, describing a gesture. It needs narration, not resolution.
	IntentNarrative IntentAction = "narrative"

	// IntentUnclear means the sentence could not be read confidently. The
	// right response is a question, not a guess.
	IntentUnclear IntentAction = "unclear"
)

// IntentActions lists every action an intent may name.
var IntentActions = []IntentAction{
	IntentAttack, IntentSkillCheck, IntentSavingThrow, IntentCastSpell,
	IntentUseItem, IntentMove, IntentTalk, IntentInteract, IntentNarrative, IntentUnclear,
}

// Valid reports whether a is a recognised action.
func (a IntentAction) Valid() bool {
	for _, known := range IntentActions {
		if a == known {
			return true
		}
	}
	return false
}

// NeedsResolution reports whether the action requires the rules engine rather
// than only narration.
func (a IntentAction) NeedsResolution() bool {
	switch a {
	case IntentAttack, IntentSkillCheck, IntentSavingThrow, IntentCastSpell, IntentInteract:
		return true
	}
	return false
}

// Confidence is how sure the parser is that it read the sentence correctly.
type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

// Valid reports whether c is a recognised confidence level.
func (c Confidence) Valid() bool {
	return c == ConfidenceHigh || c == ConfidenceMedium || c == ConfidenceLow
}

// Intent is a parsed player action.
type Intent struct {
	Action IntentAction `json:"action"`
	Actor  string       `json:"actor,omitempty"`
	Target string       `json:"target,omitempty"`

	Skill   Skill   `json:"skill,omitempty"`
	Ability Ability `json:"ability,omitempty"`
	Weapon  string  `json:"weapon,omitempty"`
	Spell   string  `json:"spell,omitempty"`
	Item    string  `json:"item,omitempty"`

	// Advantage is a circumstance the parser read from the sentence that the
	// rules cannot derive from recorded state -- striking from hiding, an ally
	// helping. It is a closed list: free text here would turn "I attack with
	// tremendous advantage" into a mechanical claim.
	//
	// Everything derivable (target conditions, the attacker's own conditions,
	// exhaustion) is derived by the engine instead, and this only combines with
	// it -- advantage and disadvantage never stack, and one of each cancels.
	Advantage AdvantageReason `json:"advantage,omitempty"`

	// Interaction is what the player is doing to the thing named in Target,
	// from the closed list the object itself supports.
	Interaction InteractionKind `json:"interaction,omitempty"`

	// NPCOutcome is what the party did to the NPC they are talking to, from a
	// closed list. Disposition is a mechanical value, so "the innkeeper now
	// likes you more" is a classification the parser makes, never a number it
	// invents -- the table decides the number.
	NPCOutcome InteractionOutcome `json:"npc_outcome,omitempty"`

	// SlotLevel is the spell slot the caster is spending. Zero means the
	// spell's own level, never a free cast -- a levelled spell "at level 0"
	// would cost nothing, which is the one reading that must not be possible.
	SlotLevel int `json:"slot_level,omitempty"`

	// SuggestedDC is the difficulty the parser proposes, and the turn resolves
	// against it.
	//
	// That is a real division of labour rather than a leak: setting difficulty
	// is a Dungeon Master's judgement, and the model is the DM here. What it is
	// not allowed to do is decide the *outcome* -- the engine rolls, compares
	// and reports, and the narration describes what it found.
	//
	// Normalise snaps this to the table's rungs, because a DC 17 is not a 5e
	// number: it is a model splitting the difference between two real ones.
	SuggestedDC int `json:"suggested_dc,omitempty"`

	Confidence Confidence `json:"confidence"`

	// Clarification is the question to ask when the sentence cannot be read.
	Clarification string `json:"clarification,omitempty"`

	// Rationale is the parser's one-line explanation, kept for debugging a
	// misread rather than for showing to players.
	Rationale string `json:"rationale,omitempty"`

	// RawInput is the sentence this was parsed from.
	RawInput string `json:"raw_input,omitempty"`
}

// Standard 5e difficulty classes, offered to the parser so its suggestions
// land on the table's rungs instead of arbitrary numbers.
var DifficultyClasses = map[string]int{
	"very_easy":         5,
	"easy":              10,
	"medium":            15,
	"hard":              20,
	"very_hard":         25,
	"nearly_impossible": 30,
}

// DifficultyRungs are the standard difficulties in ascending order.
//
// They are the only values a check should ever resolve against: 5e has six
// rungs, and a DC between two of them is a number nobody chose.
var DifficultyRungs = []int{5, 10, 15, 20, 25, 30}

// SnapToDifficulty rounds a proposed difficulty onto the nearest rung.
//
// Zero is preserved because zero means "no difficulty proposed" -- snapping it
// to 5 would give every unremarkable action a check to pass. Ties round down,
// towards the player.
func SnapToDifficulty(dc int) int {
	if dc <= 0 {
		return 0
	}

	best := DifficultyRungs[0]
	bestDistance := -1
	for _, rung := range DifficultyRungs {
		distance := dc - rung
		if distance < 0 {
			distance = -distance
		}
		// Strictly closer, so an exact tie keeps the lower rung already held.
		if bestDistance < 0 || distance < bestDistance {
			best, bestDistance = rung, distance
		}
	}
	return best
}

// DifficultyLabel names the rung a DC sits on.
func DifficultyLabel(dc int) string {
	switch {
	case dc <= 5:
		return "very_easy"
	case dc <= 10:
		return "easy"
	case dc <= 15:
		return "medium"
	case dc <= 20:
		return "hard"
	case dc <= 25:
		return "very_hard"
	default:
		return "nearly_impossible"
	}
}

// ActionOptions is what a character can actually do right now.
//
// It is handed to the parser so the model chooses from a closed list instead
// of inventing a skill the character lacks or a weapon they are not holding.
// Constraining the choices is most of what makes the parse reliable.
type ActionOptions struct {
	Actor   string   `json:"actor"`
	Skills  []Skill  `json:"skills"`
	Weapons []string `json:"weapons"`
	Spells  []string `json:"spells"`
	Items   []string `json:"items"`
	Targets []string `json:"targets"`

	// NPCs are the people present who can be spoken to. Separate from Targets
	// because you attack a creature and you talk to a person, and a parser
	// offered one closed list for both will eventually confuse them.
	NPCs []string `json:"npcs"`

	// Interactables and Exits are what is in the room. Offered as closed lists
	// for the same reason weapons are: without them the parser invents
	// furniture, and nothing can act on furniture that does not exist.
	Interactables []string `json:"interactables"`
	Exits         []string `json:"exits"`
}

// ActionOptionsFor builds the option list from a character sheet and the
// creatures currently in play.
func ActionOptionsFor(c *Character, targets []string, npcs ...string) ActionOptions {
	opts := ActionOptions{
		Actor:   c.Name,
		Targets: append([]string(nil), targets...),
		NPCs:    append([]string(nil), npcs...),
	}

	// Every skill is available; proficiency only changes the modifier.
	opts.Skills = append(opts.Skills, Skills...)

	for _, w := range c.Equipment.Weapons {
		opts.Weapons = addUnique(opts.Weapons, w.Name)
	}
	for _, item := range c.Inventory {
		if item.Weapon != nil {
			opts.Weapons = addUnique(opts.Weapons, item.Name)
			continue
		}
		opts.Items = addUnique(opts.Items, item.Name)
	}

	opts.Spells = addUnique(opts.Spells, c.Spells.Cantrips...)
	for _, s := range c.Spells.Known {
		opts.Spells = addUnique(opts.Spells, s.Name)
	}

	sort.Strings(opts.Weapons)
	sort.Strings(opts.Items)
	sort.Strings(opts.Spells)
	return opts
}

// SkillNames renders the skill list for a prompt.
func (o ActionOptions) SkillNames() []string {
	names := make([]string, 0, len(o.Skills))
	for _, s := range o.Skills {
		names = append(names, string(s))
	}
	return names
}

func listOrNone(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	return strings.Join(values, ", ")
}

// Prompt renders the options as the closed lists a parser must choose from.
func (o ActionOptions) Prompt() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Actor: %s\n", o.Actor)
	fmt.Fprintf(&b, "Targets in play: %s\n", listOrNone(o.Targets))
	fmt.Fprintf(&b, "People present to talk to: %s\n", listOrNone(o.NPCs))
	fmt.Fprintf(&b, "Things here to interact with: %s\n", listOrNone(o.Interactables))
	fmt.Fprintf(&b, "Ways out: %s\n", listOrNone(o.Exits))
	fmt.Fprintf(&b, "Weapons carried: %s\n", listOrNone(o.Weapons))
	fmt.Fprintf(&b, "Spells known: %s\n", listOrNone(o.Spells))
	fmt.Fprintf(&b, "Items carried: %s\n", listOrNone(o.Items))
	fmt.Fprintf(&b, "Skills: %s", listOrNone(o.SkillNames()))
	return b.String()
}

// Normalise tidies a parsed intent: lowercases the enumerations, maps spaces
// to underscores, and clamps the suggested DC into the table's range.
//
// Models are inconsistent about casing and spacing, and rejecting "Sleight of
// Hand" for not being "sleight_of_hand" would be pedantry rather than safety.
func (i *Intent) Normalise() {
	slug := func(s string) string {
		return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "_")
	}

	i.Action = IntentAction(slug(string(i.Action)))
	i.Confidence = Confidence(slug(string(i.Confidence)))
	if i.Skill != "" {
		i.Skill = Skill(slug(string(i.Skill)))
	}
	if i.Ability != "" {
		i.Ability = Ability(slug(string(i.Ability)))
	}

	i.Actor = strings.TrimSpace(i.Actor)
	i.Target = strings.TrimSpace(i.Target)
	i.Weapon = strings.TrimSpace(i.Weapon)
	i.Spell = strings.TrimSpace(i.Spell)
	i.Item = strings.TrimSpace(i.Item)

	// A skill check with no ability named can infer it from the skill.
	if i.Skill != "" && i.Skill.Valid() && i.Ability == "" {
		i.Ability = i.Skill.Ability()
	}

	if i.Confidence == "" {
		i.Confidence = ConfidenceMedium
	}
	// An unrecognised circumstance grants nothing rather than breaking the
	// turn: a model that invents one should change no dice.
	i.Advantage = AdvantageReason(slug(string(i.Advantage)))
	if !i.Advantage.Valid() {
		i.Advantage = ReasonNone
	}

	i.Interaction = InteractionKind(slug(string(i.Interaction)))
	if i.Interaction != "" && !i.Interaction.Valid() {
		i.Interaction = ""
	}

	i.NPCOutcome = InteractionOutcome(slug(string(i.NPCOutcome)))
	if !i.NPCOutcome.Valid() {
		i.NPCOutcome = OutcomeNone
	}

	if i.SlotLevel < 0 {
		i.SlotLevel = 0
	}
	if i.SlotLevel > 9 {
		i.SlotLevel = 9
	}
	i.SuggestedDC = SnapToDifficulty(i.SuggestedDC)
}

// Validate checks an intent against what the character can actually do.
//
// The parser is a language model and will occasionally name a weapon the
// character is not carrying. Catching that here turns a hallucination into a
// clarifying question rather than an illegal action.
func (i Intent) Validate(opts ActionOptions) error {
	var problems []string

	if !i.Action.Valid() {
		return Invalid("unknown intent action %q", i.Action)
	}
	if !i.Confidence.Valid() {
		problems = append(problems, fmt.Sprintf("unknown confidence %q", i.Confidence))
	}
	if !i.Advantage.Valid() {
		problems = append(problems, fmt.Sprintf("unknown advantage reason %q", i.Advantage))
	}
	if !i.NPCOutcome.Valid() {
		problems = append(problems, fmt.Sprintf("unknown npc outcome %q", i.NPCOutcome))
	}

	switch i.Action {
	case IntentAttack:
		if i.Target == "" {
			problems = append(problems, "an attack needs a target")
		} else if len(opts.Targets) > 0 && !containsFold(opts.Targets, i.Target) {
			problems = append(problems, fmt.Sprintf("%q is not a creature in play", i.Target))
		}
		if i.Weapon != "" && len(opts.Weapons) > 0 && !containsFold(opts.Weapons, i.Weapon) {
			problems = append(problems, fmt.Sprintf("%s is not carrying %q", opts.Actor, i.Weapon))
		}

	case IntentSkillCheck:
		if i.Skill == "" {
			problems = append(problems, "a skill check needs a skill")
		} else if !i.Skill.Valid() {
			problems = append(problems, fmt.Sprintf("%q is not a 5e skill", i.Skill))
		}

	case IntentSavingThrow:
		if !i.Ability.Valid() {
			problems = append(problems, fmt.Sprintf("%q is not an ability", i.Ability))
		}

	case IntentCastSpell:
		if i.Spell == "" {
			problems = append(problems, "casting needs a spell")
		} else if len(opts.Spells) > 0 && !containsFold(opts.Spells, i.Spell) {
			problems = append(problems, fmt.Sprintf("%s does not know %q", opts.Actor, i.Spell))
		}

	case IntentUseItem:
		if i.Item == "" {
			problems = append(problems, "using an item needs an item")
		} else if len(opts.Items) > 0 && !containsFold(opts.Items, i.Item) {
			problems = append(problems, fmt.Sprintf("%s is not carrying %q", opts.Actor, i.Item))
		}

	case IntentTalk:
		// An unnamed listener is fine -- calling out to an empty room is still
		// talking -- but a named one must be someone who is actually here.
		if i.Target != "" && len(opts.NPCs) > 0 && !containsFold(opts.NPCs, i.Target) {
			problems = append(problems, fmt.Sprintf("%q is not here to talk to", i.Target))
		}

	case IntentInteract:
		if i.Target == "" {
			problems = append(problems, "interacting needs something to interact with")
		} else if len(opts.Interactables) > 0 && !containsFold(opts.Interactables, i.Target) {
			problems = append(problems, fmt.Sprintf("there is no %q here", i.Target))
		}

	case IntentUnclear:
		if i.Clarification == "" {
			problems = append(problems, "an unclear intent must supply a clarifying question")
		}
	}

	if len(problems) > 0 {
		return Invalid("intent does not fit the situation: %s", strings.Join(problems, "; "))
	}
	return nil
}

// AsUnclear converts an intent into a request for clarification, which is what
// to do when validation fails rather than resolving something wrong.
func (i Intent) AsUnclear(question string) Intent {
	return Intent{
		Action:        IntentUnclear,
		Actor:         i.Actor,
		Confidence:    ConfidenceLow,
		Clarification: question,
		Rationale:     i.Rationale,
		RawInput:      i.RawInput,
	}
}

func containsFold(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(needle)) {
			return true
		}
	}
	return false
}
