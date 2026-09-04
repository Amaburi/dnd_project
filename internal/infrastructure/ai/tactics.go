package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

// TacticsRequest asks what a monster does on its turn.
type TacticsRequest struct {
	Monster  *models.Monster
	Self     models.Combatant
	Enemies  []models.Combatant
	Allies   []models.Combatant
	Round    int
	Scene    string
	Recently string
}

// TacticalChoice is what the model proposes a monster does.
//
// Like a player's intent, this is a *proposal*. The engine still resolves it,
// so a model that picks an action the statblock does not have, or a target
// already dead, changes nothing about what actually happens.
type TacticalChoice struct {
	ActionName string `json:"action"`
	TargetName string `json:"target"`
	Rationale  string `json:"rationale"`

	// Retreat marks a creature that would rather leave than fight on, which a
	// wounded animal should be allowed to do.
	Retreat bool `json:"retreat"`
}

// TacticsResponse is a validated choice plus what it cost.
type TacticsResponse struct {
	Choice TacticalChoice

	// Action and Target are the resolved statblock entry and combatant, set
	// only when the choice survived validation.
	Action *models.MonsterAction
	Target *models.Combatant

	// Fallback is set when the model's choice was unusable and a sensible
	// default was substituted rather than the turn being skipped.
	Fallback     bool
	FallbackNote string

	TokensUsed     int
	Cost           float64
	ProcessingTime time.Duration
}

// ChooseTactics asks the model how a monster should spend its turn.
func (s *Service) ChooseTactics(ctx context.Context, req *TacticsRequest) (*TacticsResponse, error) {
	startTime := time.Now()

	if req.Monster == nil {
		return nil, models.Invalid("a statblock is required")
	}

	living := livingTargets(req.Enemies)
	if len(living) == 0 {
		return nil, models.Invalid("there is nothing left to act against")
	}

	actions := attackActions(req.Monster)
	if len(actions) == 0 {
		return nil, models.Invalid("%s has no actions to take", req.Monster.Name)
	}

	messages, err := s.promptBuilder.BuildPrompt("enemy_tactics", map[string]string{
		"monster_name": req.Monster.Name,
		"monster_type": orPlaceholder(req.Monster.Type, "creature"),
		"self_status":  combatantLine(req.Self),
		"traits":       traitNames(req.Monster),
		"actions":      actionMenu(actions),
		"enemies":      combatantList(living),
		"allies":       combatantList(livingTargets(req.Allies)),
		"round":        fmt.Sprintf("%d", maxInt(req.Round, 1)),
		"scene":        orPlaceholder(req.Scene, "no additional context"),
		"recently":     orPlaceholder(req.Recently, "nothing of note"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Tactics are a decision, not prose: low temperature and JSON, the same
	// treatment intent extraction gets.
	resp, err := s.client.ChatCompletion(ctx, &ChatRequest{
		Messages:       messages,
		Model:          s.config.Model,
		Temperature:    Float(GetTemperature("enemy_tactics")),
		MaxTokens:      300,
		ResponseFormat: JSONObjectFormat(),
	})
	if err != nil {
		return nil, fmt.Errorf("AI request failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from AI")
	}

	choice, err := ParseTacticalChoice(resp.Choices[0].Message.Content)
	if err != nil {
		return nil, err
	}

	out := &TacticsResponse{
		Choice:         choice,
		TokensUsed:     resp.Usage.TotalTokens,
		Cost:           s.calculateCost(resp.Usage),
		ProcessingTime: time.Since(startTime),
	}

	// A monster whose turn is skipped because the model named a nonexistent
	// action is worse than one that simply attacks: fall back rather than
	// stall the fight.
	action, ok := findAction(actions, choice.ActionName)
	if !ok {
		out.Fallback = true
		out.FallbackNote = fmt.Sprintf("%q is not an action %s has; using %s",
			choice.ActionName, req.Monster.Name, actions[0].Name)
		action = actions[0]
		out.Choice.ActionName = action.Name
	}
	out.Action = &action

	target, ok := findCombatant(living, choice.TargetName)
	if !ok {
		out.Fallback = true
		if out.FallbackNote == "" {
			out.FallbackNote = fmt.Sprintf("%q is not a living target; using %s",
				choice.TargetName, living[0].Name)
		}
		target = living[0]
		out.Choice.TargetName = target.Name
	}
	out.Target = &target

	return out, nil
}

// ParseTacticalChoice reads the JSON object a model returned.
func ParseTacticalChoice(content string) (TacticalChoice, error) {
	payload := extractJSONObject(content)
	if payload == "" {
		return TacticalChoice{}, models.Invalid("no JSON object found in the model's reply")
	}

	var choice TacticalChoice
	if err := json.Unmarshal([]byte(payload), &choice); err != nil {
		return TacticalChoice{}, models.Invalid("could not parse the model's reply as a tactical choice: %v", err)
	}

	choice.ActionName = strings.TrimSpace(choice.ActionName)
	choice.TargetName = strings.TrimSpace(choice.TargetName)
	return choice, nil
}

// attackActions returns the statblock entries a monster can actually attack
// with, skipping multiattack routines that only refer to others.
func attackActions(m *models.Monster) []models.MonsterAction {
	var out []models.MonsterAction
	for _, a := range m.Actions {
		if a.IsMultiattack() || a.AttackBonus == nil {
			continue
		}
		out = append(out, a)
	}
	return out
}

func findAction(actions []models.MonsterAction, name string) (models.MonsterAction, bool) {
	for _, a := range actions {
		if strings.EqualFold(a.Name, name) {
			return a, true
		}
	}
	return models.MonsterAction{}, false
}

func livingTargets(combatants []models.Combatant) []models.Combatant {
	var out []models.Combatant
	for _, c := range combatants {
		if c.Status == models.CombatantDead {
			continue
		}
		out = append(out, c)
	}
	return out
}

func findCombatant(combatants []models.Combatant, name string) (models.Combatant, bool) {
	for _, c := range combatants {
		if strings.EqualFold(c.Name, name) {
			return c, true
		}
	}
	return models.Combatant{}, false
}

func combatantLine(c models.Combatant) string {
	return fmt.Sprintf("%s, %d/%d hit points, AC %d, status %s",
		c.Name, c.HitPoints.Current, c.HitPoints.Maximum, c.ArmorClass, c.Status)
}

func combatantList(combatants []models.Combatant) string {
	if len(combatants) == 0 {
		return "(none)"
	}
	lines := make([]string, 0, len(combatants))
	for _, c := range combatants {
		lines = append(lines, "- "+combatantLine(c))
	}
	return strings.Join(lines, "\n")
}

func actionMenu(actions []models.MonsterAction) string {
	lines := make([]string, 0, len(actions))
	for _, a := range actions {
		bonus := 0
		if a.AttackBonus != nil {
			bonus = *a.AttackBonus
		}
		reach := a.ReachFeet
		if a.RangeNormal > 0 {
			reach = a.RangeNormal
		}
		lines = append(lines, fmt.Sprintf("- %s: %+d to hit, %s %s, reach/range %d ft.",
			a.Name, bonus, a.DamageDice, a.DamageType, reach))
	}
	return strings.Join(lines, "\n")
}

func traitNames(m *models.Monster) string {
	if len(m.Traits) == 0 {
		return "(none)"
	}
	names := make([]string, 0, len(m.Traits))
	for _, t := range m.Traits {
		names = append(names, t.Name)
	}
	return strings.Join(names, ", ")
}
