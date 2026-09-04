// Package dice rolls dice.
//
// It is deliberately separate from the models package: models owns the rules
// and stays a pure function of its inputs, while everything random lives here.
// That split is what lets the rules be tested exactly rather than statistically.
package dice

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
)

// Limits on a single expression, so a malformed or hostile input cannot ask
// for a billion dice.
const (
	MaxDiceCount = 100
	MaxDieSides  = 1000
)

// Expression is a parsed dice expression such as "2d6+3".
type Expression struct {
	Count    int `json:"count"`
	Sides    int `json:"sides"`
	Modifier int `json:"modifier"`
}

// String renders the expression in the usual notation.
func (e Expression) String() string {
	base := fmt.Sprintf("%dd%d", e.Count, e.Sides)
	switch {
	case e.Modifier > 0:
		return fmt.Sprintf("%s+%d", base, e.Modifier)
	case e.Modifier < 0:
		return fmt.Sprintf("%s%d", base, e.Modifier)
	default:
		return base
	}
}

// Min is the lowest total the expression can produce.
func (e Expression) Min() int { return e.Count + e.Modifier }

// Max is the highest total the expression can produce.
func (e Expression) Max() int { return e.Count*e.Sides + e.Modifier }

// Average is the expected total, rounded down -- the number a statblock prints.
func (e Expression) Average() int {
	return (e.Count*(e.Sides+1))/2 + e.Modifier
}

// Doubled returns the expression with twice the dice and the same modifier,
// which is how a critical hit works: the dice double, the modifier does not.
func (e Expression) Doubled() Expression {
	return Expression{Count: e.Count * 2, Sides: e.Sides, Modifier: e.Modifier}
}

// Parse reads "2d6+3", "d20", "4d6-1" and the spaced variants of each.
//
// An omitted count means one die, so "d20" and "1d20" are the same thing.
func Parse(expression string) (Expression, error) {
	text := strings.ToLower(strings.ReplaceAll(expression, " ", ""))
	if text == "" {
		return Expression{}, fmt.Errorf("dice expression is empty")
	}

	// A bare number is a constant, which damage entries like "1" use.
	if n, err := strconv.Atoi(text); err == nil {
		return Expression{Count: 0, Sides: 0, Modifier: n}, nil
	}

	parts := strings.SplitN(text, "d", 2)
	if len(parts) != 2 {
		return Expression{}, fmt.Errorf("dice expression %q is not of the form NdX+B", expression)
	}

	count := 1
	if parts[0] != "" {
		var err error
		count, err = strconv.Atoi(parts[0])
		if err != nil {
			return Expression{}, fmt.Errorf("dice expression %q has an invalid die count", expression)
		}
	}

	rest, modifier := parts[1], 0
	if i := strings.IndexAny(rest, "+-"); i >= 0 {
		var err error
		modifier, err = strconv.Atoi(rest[i:])
		if err != nil {
			return Expression{}, fmt.Errorf("dice expression %q has an invalid modifier", expression)
		}
		rest = rest[:i]
	}

	sides, err := strconv.Atoi(rest)
	if err != nil {
		return Expression{}, fmt.Errorf("dice expression %q has an invalid die size", expression)
	}

	switch {
	case count < 1 || count > MaxDiceCount:
		return Expression{}, fmt.Errorf("dice expression %q asks for %d dice, want 1-%d", expression, count, MaxDiceCount)
	case sides < 2 || sides > MaxDieSides:
		return Expression{}, fmt.Errorf("dice expression %q uses a d%d, want d2-d%d", expression, sides, MaxDieSides)
	}

	return Expression{Count: count, Sides: sides, Modifier: modifier}, nil
}

// Result is the outcome of rolling an expression.
type Result struct {
	Expression Expression `json:"expression"`
	Rolls      []int      `json:"rolls"`
	Modifier   int        `json:"modifier"`
	Total      int        `json:"total"`
}

// String renders the roll the way a table would read it out.
func (r Result) String() string {
	if len(r.Rolls) == 0 {
		return strconv.Itoa(r.Total)
	}
	parts := make([]string, len(r.Rolls))
	for i, roll := range r.Rolls {
		parts[i] = strconv.Itoa(roll)
	}
	body := strings.Join(parts, "+")
	switch {
	case r.Modifier > 0:
		return fmt.Sprintf("%s (%s) %+d = %d", r.Expression, body, r.Modifier, r.Total)
	case r.Modifier < 0:
		return fmt.Sprintf("%s (%s) %d = %d", r.Expression, body, r.Modifier, r.Total)
	default:
		return fmt.Sprintf("%s (%s) = %d", r.Expression, body, r.Total)
	}
}

// Roller produces random rolls.
//
// A Roller is safe for concurrent use. Seed one explicitly with NewSeeded to
// make a test deterministic instead of statistical.
type Roller struct {
	mu  sync.Mutex
	rng *rand.Rand
}

// New returns a Roller seeded from the operating system.
func New() *Roller {
	var seed int64
	var buf [8]byte
	if _, err := crand.Read(buf[:]); err == nil {
		seed = int64(binary.LittleEndian.Uint64(buf[:]))
	}
	return NewSeeded(seed)
}

// NewSeeded returns a Roller with a fixed seed, so a sequence of rolls repeats
// exactly. Tests use this; play does not.
func NewSeeded(seed int64) *Roller {
	return &Roller{rng: rand.New(rand.NewSource(seed))} //nolint:gosec // not cryptographic
}

// die rolls a single die of the given size.
func (r *Roller) die(sides int) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rng.Intn(sides) + 1
}

// RollExpression rolls a parsed expression.
func (r *Roller) RollExpression(e Expression) Result {
	result := Result{Expression: e, Modifier: e.Modifier, Total: e.Modifier}
	for i := 0; i < e.Count; i++ {
		roll := r.die(e.Sides)
		result.Rolls = append(result.Rolls, roll)
		result.Total += roll
	}
	return result
}

// Roll parses and rolls an expression such as "1d8+3".
func (r *Roller) Roll(expression string) (Result, error) {
	e, err := Parse(expression)
	if err != nil {
		return Result{}, err
	}
	return r.RollExpression(e), nil
}
