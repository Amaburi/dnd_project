package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/dnd-campaign/manager/internal/domain/dice"
	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/gin-gonic/gin"
)

// DiceHandler rolls dice and answers questions about the odds.
//
// It is the one handler with no repository: a roll is not campaign state, and
// making the client name a campaign to roll a d20 would be ceremony for
// nothing. Rolls that matter to the story are recorded by the action and
// combat endpoints, which append a story event; this is the scratch pad.
type DiceHandler struct {
	roller *dice.Roller
}

// NewDiceHandler creates a new dice handler.
func NewDiceHandler(roller *dice.Roller) *DiceHandler {
	return &DiceHandler{roller: roller}
}

func badRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{"error": message})
}

// parseMode reads a roll mode, defaulting to normal.
//
// An unknown value is refused rather than quietly treated as normal: silently
// downgrading a typo'd "advantge" would cost a player the advantage they asked
// for and nothing would say so.
func parseMode(raw string) (models.RollMode, bool) {
	switch models.RollMode(raw) {
	case "":
		return models.RollNormal, true
	case models.RollNormal, models.RollAdvantage, models.RollDisadvantage:
		return models.RollMode(raw), true
	default:
		return "", false
	}
}

type rollRequest struct {
	Expression string `json:"expression"`
}

type rollResponse struct {
	dice.Result
	Notation string `json:"notation"`
}

// Roll rolls an arbitrary expression such as "2d6+3".
func (h *DiceHandler) Roll(c *gin.Context) {
	var req rollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	result, err := h.roller.Roll(req.Expression)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, rollResponse{Result: result, Notation: result.String()})
}

type d20Request struct {
	Modifier int    `json:"modifier"`
	Mode     string `json:"mode"`

	// DC is optional. A pointer distinguishes "against DC 0", which is a
	// legitimate if generous target, from "no target given".
	DC *int `json:"dc"`
}

type d20Response struct {
	Roll     models.D20Result    `json:"roll"`
	Notation string              `json:"notation"`
	DC       *int                `json:"dc,omitempty"`
	Outcome  models.CheckOutcome `json:"outcome,omitempty"`
	Odds     *dice.CheckOdds     `json:"odds,omitempty"`
}

// d20Notation renders the roll the way a table would read it out, keeping the
// discarded die visible: "d20 (19, 4 keep 19) +3 = 22" says more than "22".
// No angle brackets, so the JSON encoder does not escape them into noise.
func d20Notation(roll models.D20Result) string {
	var b strings.Builder
	b.WriteString("d20 (")
	for i, die := range roll.Rolls {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Itoa(die))
	}
	if len(roll.Rolls) > 1 {
		b.WriteString(" keep ")
		b.WriteString(strconv.Itoa(roll.Natural))
	}
	b.WriteString(")")
	if roll.Modifier != 0 {
		fmt.Fprintf(&b, " %+d", roll.Modifier)
	}
	fmt.Fprintf(&b, " = %d", roll.Total)
	return b.String()
}

// RollD20 rolls a d20, optionally resolving it against a DC.
func (h *DiceHandler) RollD20(c *gin.Context) {
	var req d20Request
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	mode, ok := parseMode(req.Mode)
	if !ok {
		badRequest(c, "mode must be normal, advantage or disadvantage")
		return
	}

	roll := h.roller.D20(req.Modifier, mode)
	response := d20Response{Roll: roll, Notation: d20Notation(roll), DC: req.DC}
	if req.DC != nil {
		response.Outcome = models.ResolveCheck(roll, *req.DC)
		odds := dice.OddsOfCheck(*req.DC, req.Modifier, mode)
		response.Odds = &odds
	}
	c.JSON(http.StatusOK, response)
}

type damageRequest struct {
	Expression string `json:"expression"`
	Critical   bool   `json:"critical"`
}

type damageResponse struct {
	dice.Result
	Critical bool   `json:"critical"`
	Notation string `json:"notation"`
}

// RollDamage rolls damage, doubling the dice but not the modifier on a crit.
func (h *DiceHandler) RollDamage(c *gin.Context) {
	var req damageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	result, err := h.roller.RollDamage(req.Expression, req.Critical)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, damageResponse{
		Result: result, Critical: req.Critical, Notation: result.String(),
	})
}

type targetOdds struct {
	Total   int     `json:"total"`
	Exactly float64 `json:"exactly"`
	AtLeast float64 `json:"at_least"`
	AtMost  float64 `json:"at_most"`
}

type distributionResponse struct {
	dice.Distribution
	Notation string      `json:"notation"`
	Target   *targetOdds `json:"target,omitempty"`
}

// Probability returns the exact distribution of an expression.
//
// GET /dice/probability?expression=2d6%2B3&target=10
func (h *DiceHandler) Probability(c *gin.Context) {
	raw := c.Query("expression")
	if raw == "" {
		badRequest(c, "expression query parameter required, e.g. ?expression=2d6%2B3")
		return
	}

	expression, err := dice.Parse(raw)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	distribution, err := dice.Distribute(expression)
	if err != nil {
		badRequest(c, err.Error())
		return
	}

	response := distributionResponse{Distribution: distribution, Notation: expression.String()}
	if rawTarget := c.Query("target"); rawTarget != "" {
		target, err := strconv.Atoi(rawTarget)
		if err != nil {
			badRequest(c, "target must be a whole number")
			return
		}
		var exactly float64
		for _, outcome := range distribution.Outcomes {
			if outcome.Total == target {
				exactly = outcome.Probability
				break
			}
		}
		response.Target = &targetOdds{
			Total:   target,
			Exactly: exactly,
			AtLeast: distribution.AtLeast(target),
			AtMost:  distribution.AtMost(target),
		}
	}
	c.JSON(http.StatusOK, response)
}

type checkOddsRequest struct {
	DC       int    `json:"dc"`
	Modifier int    `json:"modifier"`
	Mode     string `json:"mode"`
}

// CheckProbability answers "what are my chances against this DC".
func (h *DiceHandler) CheckProbability(c *gin.Context) {
	var req checkOddsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	mode, ok := parseMode(req.Mode)
	if !ok {
		badRequest(c, "mode must be normal, advantage or disadvantage")
		return
	}
	c.JSON(http.StatusOK, dice.OddsOfCheck(req.DC, req.Modifier, mode))
}

type attackOddsRequest struct {
	TargetAC int    `json:"target_ac"`
	Modifier int    `json:"modifier"`
	Damage   string `json:"damage"`
	Mode     string `json:"mode"`

	// CritRange is the lowest natural roll that crits. Zero means unset, and
	// unset means 20 -- a literal zero would make every roll a critical.
	CritRange int `json:"crit_range"`
}

// AttackProbability answers "how likely am I to hit, and for how much".
//
// The expected damage is what makes this worth an endpoint: it is the number
// behind "how long does this fight last", which is the question encounter
// balance actually asks.
func (h *DiceHandler) AttackProbability(c *gin.Context) {
	var req attackOddsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	mode, ok := parseMode(req.Mode)
	if !ok {
		badRequest(c, "mode must be normal, advantage or disadvantage")
		return
	}
	if req.CritRange == 0 {
		req.CritRange = models.NaturalCrit
	}

	odds, err := dice.OddsOfAttackWithMode(req.TargetAC, req.Modifier, req.CritRange, req.Damage, mode)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, odds)
}
