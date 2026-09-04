package ai

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// PromptTemplate defines a prompt template
type PromptTemplate struct {
	System string
	User   string
}

// PromptBuilder helps build prompts with context
type PromptBuilder struct {
	templates map[string]PromptTemplate
}

// NewPromptBuilder creates a new prompt builder
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{
		templates: defaultPrompts(),
	}
}

// narrationContract is prepended to every template that describes an outcome
// the rules engine has already decided.
//
// This is the load-bearing paragraph of the whole AI layer. The engine is
// authoritative; the model's only job is to make its verdict readable. Without
// this the model happily invents damage, revises a miss into a graze, or
// announces a condition nobody applied -- and the game state and the prose
// drift apart within a few turns.
const narrationContract = `## Absolute rules

The FACTS below have already been decided by the game engine. They are final.

- Never change, round, recalculate or dispute any number in the FACTS.
- Never decide an outcome. Whether something hit, how much damage it dealt and
  what state the target is in are given to you, not yours to choose.
- Never roll dice, invent a roll, or mention a number that is not in the FACTS.
- Never apply conditions, movement, healing or resources. You describe; the
  engine decides.
- If the FACTS say a miss, it is a miss. Describe how it missed.

Write prose only. No headings, no bullet lists, no dice notation, no statistics.`

// defaultPrompts returns the default prompt templates
func defaultPrompts() map[string]PromptTemplate {
	return map[string]PromptTemplate{
		// -------------------------------------------------------------------
		// Parsing. Reads a sentence; decides nothing.
		// -------------------------------------------------------------------
		"intent_extraction": {
			System: `You translate a player's sentence into a single structured action for a D&D 5e engine. You are a parser, not a Dungeon Master.

Reply with ONE JSON object and nothing else. No prose, no markdown fence, no explanation outside the JSON.

## Schema

{
  "action": "attack" | "skill_check" | "saving_throw" | "cast_spell" | "use_item" | "move" | "talk" | "narrative" | "unclear",
  "target": "exact name from Targets in play, or empty",
  "skill": "one of the listed skills, or empty",
  "ability": "strength|dexterity|constitution|intelligence|wisdom|charisma, or empty",
  "weapon": "exact name from Weapons carried, or empty",
  "spell": "exact name from Spells known, or empty",
  "item": "exact name from Items carried, or empty",
  "suggested_dc": 5 | 10 | 15 | 20 | 25 | 30,
  "confidence": "high" | "medium" | "low",
  "clarification": "a question to ask the player, required when action is unclear",
  "rationale": "one short sentence on why you chose this action"
}

## Choosing the action

- "attack" only when the player strikes a named creature that is in play.
- "skill_check" when success is uncertain and a listed skill decides it.
- "saving_throw" only when something is being resisted, not attempted.
- "narrative" when nothing is at stake: looking, talking to no one, describing.
  Not everything needs a roll. Prefer "narrative" over inventing a check.
- "unclear" when the sentence is ambiguous, names something not in the lists,
  or could reasonably mean two different actions. Ask rather than guess.

## Hard constraints

- Names must be copied EXACTLY from the lists supplied. Never invent a weapon,
  spell, item or target that is not listed.
- If what the player describes is not available to them, answer "unclear" and
  say so in the clarification.
- Difficulty: 5 very easy, 10 easy, 15 medium, 20 hard, 25 very hard,
  30 nearly impossible. Use 0 when no check applies.
- Set confidence honestly. "low" is a useful answer; a confident wrong parse is
  the worst outcome.`,
			User: `Player said: "{{player_input}}"

Available to this character:
{{options}}

Situation: {{situation}}`,
		},

		// -------------------------------------------------------------------
		// Narration of decided outcomes.
		// -------------------------------------------------------------------
		"action_narration": {
			System: `You narrate one combat action in a D&D 5e game. Voice: {{narrative_voice}}. Tone: {{combat_tone}}.

` + narrationContract + `

Two or three sentences. Make the swing feel physical; keep the mechanics out of
the prose. A critical hit deserves weight. A miss should be interesting rather
than a non-event.`,
			User: `FACTS (authoritative):
- Attacker: {{attacker}}
- Target: {{target}}
- Weapon: {{weapon}}
- Outcome: {{outcome}} (hit: {{hit}}, critical: {{critical}})
- Damage dealt: {{damage_total}} {{damage_type}} ({{damage_affinity}})
- Target now: {{target_hp}} hit points, status {{target_status}}
- Engine summary: {{fact_summary}}

Scene: {{context}}`,
		},

		"check_narration": {
			System: `You narrate the result of one ability check, skill check or saving throw in D&D 5e. Voice: {{narrative_voice}}.

` + narrationContract + `

One or two sentences. Let the margin colour the description: a result that
barely cleared the difficulty should feel narrow, a large margin effortless. A
failure should complicate the scene rather than simply stop it.`,
			User: `FACTS (authoritative):
- Actor: {{actor}}
- Test: {{check_kind}} using {{ability}} (skill: {{skill}})
- Difficulty: DC {{dc}}
- Result: {{outcome}}, by a margin of {{margin}} (narrow: {{was_close}})
- Natural die: {{natural}}
- Engine summary: {{fact_summary}}

Scene: {{context}}`,
		},

		"enemy_tactics": {
			System: `You choose what one monster does on its turn in a D&D 5e fight. You are a tactician, not a Dungeon Master.

Reply with ONE JSON object and nothing else.

{
  "action": "exact name from the Actions list",
  "target": "exact name from the Enemies list",
  "retreat": true | false,
  "rationale": "one short sentence"
}

## How to choose

- Play the creature, not the optimum. A wolf flanks and harries; an ogre swings
  at whoever is closest; a wight goes for the weakest thing it can reach.
- Read the creature's traits and let them shape the choice. Pack Tactics wants
  an ally nearby. Sunlight Sensitivity wants shade.
- Finishing a badly wounded enemy is often right, but a creature with no sense
  of tactics would not know who is wounded.
- Set "retreat" when this creature would rather leave than keep fighting -- a
  beast at a few hit points, or a coward whose allies have fallen. It is not a
  surrender, only an intent.

## Hard constraints

- "action" must be copied exactly from the Actions list. Never invent one.
- "target" must be copied exactly from the Enemies list. Never invent one.
- You do not roll, decide whether the attack hits, or say how much damage it
  deals. You choose only what is attempted; the engine resolves it.`,
			User: `Round {{round}}.

You are: {{monster_name}} ({{monster_type}})
Your state: {{self_status}}
Your traits: {{traits}}

Actions available:
{{actions}}

Enemies:
{{enemies}}

Allies:
{{allies}}

Scene: {{scene}}
Recently: {{recently}}`,
		},

		// -------------------------------------------------------------------
		// Scene and story. No mechanics at all.
		// -------------------------------------------------------------------
		"dm_base": {
			System: `You are the narrator of a D&D 5th Edition game.

## Your Personality
- Style: {{dm_style}}
- Voice: {{narrative_voice}}
- Humor: {{humor_level}}
- Detail: {{detail_level}}

## What you do
- Describe places, creatures and events so the players can picture them.
- Give the world reactions that follow from what the players did.
- Offer two or three things the players might do next.

## What you never do
A separate rules engine resolves every roll, every attack and every change to
the game state. You have no access to it and no authority over it.

- Never roll dice or state the result of a roll.
- Never decide whether an attempt succeeded.
- Never apply or remove damage, conditions, items or position.
- Never announce a change to a character sheet.

If an action needs a ruling, describe up to the point of uncertainty and stop.
The engine resolves it; you will be asked to narrate the outcome afterwards.

Prose only. No headings, no bullet lists.`,
			User: `{{player_input}}

Current Context:
- Location: {{location}}
- Party Status: {{party_status}}
- Recent Events: {{recent_events}}`,
		},

		"npc_dialogue": {
			System: `You are {{npc_name}}, a {{npc_race}} {{npc_class}}.

**Personality Traits**: {{personality_traits}}
**Background**: {{npc_background}}
**Motivations**: {{motivations}}
**Speech Pattern**: {{speech_pattern}}
**Current Emotional State**: {{emotional_state}}
**Knowledge**: {{knowledge}}
**Relationship to Party**: {{relationship}}

Speak only as {{npc_name}}, in first person. Stay inside what this character
would plausibly know: if asked something beyond your knowledge, say so in
character rather than inventing world facts.

You are a person in the world, not its narrator. Do not describe the scene, do
not resolve anything, do not speak for the players, and do not decide what
their actions achieve.`,
			User: `{{speaker_name}} says: "{{player_message}}"

Context: {{context}}`,
		},

		"narrative_generation": {
			System: `You are a D&D scene writer. Generate a vivid description that:
- Paints a clear picture of the place
- Engages more than sight: sound, smell, temperature, the feel of the ground
- Creates atmosphere without stating how the players feel
- Hints at what might be worth investigating
- Stays consistent with established lore

Style: {{narrative_style}}
Detail Level: {{detail_level}}

Describe only. Do not resolve actions, roll dice, or decide what the players
find -- suggest what draws the eye and let them choose.`,
			User: `Describe the following scene:
{{scene_description}}

Context:
- Time of Day: {{time_of_day}}
- Weather: {{weather}}
- Party Mood: {{party_mood}}
- Recent Events: {{recent_events}}`,
		},

		"story_adaptation": {
			System: `You adapt a D&D campaign's story to what the players chose to do.

Campaign Theme: {{campaign_theme}}
Current Arc: {{current_arc}}

- Respect player agency: their choice stands, whatever you think of it.
- Consequences should follow from the world, not punish the choice.
- Leave threads open rather than resolving them for the players.

Propose story developments only. You do not change any character's statistics,
resources or condition -- those belong to the engine.`,
			User: `Player Choice: {{player_choice}}

Current Story State:
{{story_state}}

Recent Events:
{{recent_events}}

Generate:
1. Immediate consequences
2. Long-term implications
3. New story hooks
4. NPC reactions`,
		},

		"character_backstory": {
			System: `You write D&D character backstories that:
- Fit the campaign setting
- Give the DM three or four hooks to pull on later
- Include personality traits, ideals, bonds and flaws
- Suggest where the character might grow
- Suit the character's race, class and background

Setting: {{setting}}
Tone: {{tone}}

Write only history and personality. Do not assign ability scores, levels,
equipment or mechanical features.`,
			User: `Generate a backstory for:
Name: {{character_name}}
Race: {{race}}
Class: {{class}}
Background: {{background}}
Level: {{level}}

Additional Details:
{{additional_details}}`,
		},

		"quest_generation": {
			System: `You design D&D quests that:
- Fit the campaign setting and themes
- State an objective the players can act on
- Carry a complication that is not simply "more enemies"
- Offer a reward worth the risk
- Suit the party's level

Campaign Setting: {{setting}}
Party Level: {{party_level}}
Campaign Themes: {{themes}}

Describe the quest. Do not write statblocks or assign challenge ratings.`,
			User: `Generate a quest:
Quest Type: {{quest_type}}
Location: {{location}}
Difficulty: {{difficulty}}

Include:
1. Quest Hook
2. Objectives
3. Complications
4. Rewards
5. Potential Outcomes`,
		},
	}
}

// placeholderPattern matches a single {{variable}} slot.
var placeholderPattern = regexp.MustCompile(`\{\{([A-Za-z_][A-Za-z0-9_]*)\}\}`)

// substitute replaces every {{key}} in one pass.
//
// A single pass matters: replacing key by key meant a *value* containing
// "{{other_key}}" got expanded by a later iteration, and Go randomises map
// order, so whether player text could inject another variable varied run to
// run. Scanning the template once makes substituted text inert.
//
// Missing keys are collected rather than left in place -- an unresolved
// placeholder reaching the model is always a bug.
func substitute(template string, variables map[string]string, missing map[string]struct{}) string {
	return placeholderPattern.ReplaceAllStringFunc(template, func(match string) string {
		key := placeholderPattern.FindStringSubmatch(match)[1]
		value, ok := variables[key]
		if !ok {
			missing[key] = struct{}{}
			return match
		}
		return value
	})
}

// BuildPrompt builds a prompt from a template with variables.
//
// Every placeholder the template names must have a value; an incomplete
// variable map is an error, not a prompt with braces in it.
func (pb *PromptBuilder) BuildPrompt(templateName string, variables map[string]string) ([]Message, error) {
	template, ok := pb.templates[templateName]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", templateName)
	}

	missing := map[string]struct{}{}
	systemPrompt := substitute(template.System, variables, missing)
	userPrompt := substitute(template.User, variables, missing)

	if len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for name := range missing {
			names = append(names, name)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("template %q: missing variables: %s", templateName, strings.Join(names, ", "))
	}

	return []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, nil
}

// AddTemplate adds a custom template
func (pb *PromptBuilder) AddTemplate(name string, template PromptTemplate) {
	pb.templates[name] = template
}

// GetTemplate retrieves a template by name
func (pb *PromptBuilder) GetTemplate(name string) (PromptTemplate, bool) {
	template, ok := pb.templates[name]
	return template, ok
}

// BuildConversation builds a conversation with history
func (pb *PromptBuilder) BuildConversation(templateName string, variables map[string]string, history []Message) ([]Message, error) {
	messages, err := pb.BuildPrompt(templateName, variables)
	if err != nil {
		return nil, err
	}

	// Add history between system and user messages
	if len(history) > 0 {
		result := []Message{messages[0]} // System message
		result = append(result, history...)
		if len(messages) > 1 {
			result = append(result, messages[1:]...) // User message
		}
		return result, nil
	}

	return messages, nil
}

// Temperature settings for different tasks
var TemperatureSettings = map[string]float64{
	// Parsing must be repeatable, so it is the one task run at zero.
	"intent_extraction":     0.0,
	"action_narration":      0.7,
	"check_narration":       0.6,
	"combat_resolution":     0.3, // Consistent, predictable
	"narrative_description": 0.7, // Creative
	"npc_dialogue":          0.6, // Consistent character voice
	"story_adaptation":      0.8, // Creative, surprising
	"dice_interpretation":   0.5, // Balanced
	"character_backstory":   0.7, // Creative
	"quest_generation":      0.7, // Creative
	"default":               0.7, // Balanced
}

// GetTemperature returns the recommended temperature for a task
func GetTemperature(taskType string) float64 {
	if temp, ok := TemperatureSettings[taskType]; ok {
		return temp
	}
	return TemperatureSettings["default"]
}
