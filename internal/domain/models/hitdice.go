package models

import (
	"fmt"
	"sort"
	"strings"
)

// HitDicePool is the hit dice a character has from one class.
type HitDicePool struct {
	Die   int `json:"die" bson:"die"`     // 6, 8, 10 or 12
	Total int `json:"total" bson:"total"` // levels taken in that class
	Spent int `json:"spent" bson:"spent"`
}

// Available returns how many dice in this pool remain.
func (p HitDicePool) Available() int {
	if p.Spent > p.Total {
		return 0
	}
	return p.Total - p.Spent
}

// HitDice is a character's hit dice across every class they have levels in.
//
// A single {die, total, spent} could not describe a multiclassed character:
// Fighter 3 / Wizard 2 has 3d10 and 2d6, and spending one of each is a
// different thing from spending two of either.
type HitDice []HitDicePool

// HitDiceForClasses builds the pools implied by a character's class levels,
// merging any repeated class and carrying over dice already spent.
func HitDiceForClasses(classes []ClassLevel, existing HitDice) HitDice {
	spent := map[int]int{}
	for _, pool := range existing {
		spent[pool.Die] += pool.Spent
	}

	totals := map[int]int{}
	var order []int
	for _, cl := range classes {
		die := cl.Class.HitDie()
		if die == 0 {
			continue
		}
		if _, seen := totals[die]; !seen {
			order = append(order, die)
		}
		totals[die] += cl.Level
	}

	sort.Sort(sort.Reverse(sort.IntSlice(order)))

	var out HitDice
	for _, die := range order {
		pool := HitDicePool{Die: die, Total: totals[die]}
		if s := spent[die]; s > 0 {
			pool.Spent = s
			if pool.Spent > pool.Total {
				pool.Spent = pool.Total
			}
		}
		out = append(out, pool)
	}
	return out
}

// Available returns how many hit dice remain across every pool.
func (h HitDice) Available() int {
	total := 0
	for _, pool := range h {
		total += pool.Available()
	}
	return total
}

// TotalDice returns the character's full complement of hit dice, which equals
// their total character level.
func (h HitDice) TotalDice() int {
	total := 0
	for _, pool := range h {
		total += pool.Total
	}
	return total
}

// Spend consumes one die of the given size, as spending a hit die on a short
// rest does.
func (h HitDice) Spend(die int) error {
	for i := range h {
		if h[i].Die != die {
			continue
		}
		if h[i].Available() < 1 {
			return Invalid("no d%d hit dice remaining", die)
		}
		h[i].Spent++
		return nil
	}
	return Invalid("character has no d%d hit dice", die)
}

// RegainOnLongRest returns hit dice as a long rest does: half the character's
// total, rounded down, but never fewer than one.
//
// The player chooses which dice come back; this recovers the largest first,
// which is the usual choice since bigger dice heal more.
func (h HitDice) RegainOnLongRest() {
	regain := h.TotalDice() / 2
	if regain < 1 {
		regain = 1
	}

	for i := range h {
		if regain <= 0 {
			return
		}
		back := h[i].Spent
		if back > regain {
			back = regain
		}
		h[i].Spent -= back
		regain -= back
	}
}

// String renders the pools, e.g. "3/3d10, 1/2d6".
func (h HitDice) String() string {
	if len(h) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(h))
	for _, pool := range h {
		parts = append(parts, fmt.Sprintf("%d/%dd%d", pool.Available(), pool.Total, pool.Die))
	}
	return strings.Join(parts, ", ")
}
