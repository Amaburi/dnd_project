package ai

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// budgetWindow is the period a per-hour budget is measured over.
const budgetWindow = time.Hour

// budget caps how many requests reach the provider in a rolling hour.
//
// It belongs here rather than in HTTP middleware: the limit being protected is
// the provider's, not this server's, and a single HTTP request can make several
// provider calls -- a turn makes two, and three when it compacts history.
// Counting requests at the edge would measure the wrong thing.
//
// Refusing locally is cheaper than being refused remotely and far cheaper than
// being billed, which is what makes this worth having on a free tier.
type budget struct {
	mu    sync.Mutex
	limit int
	now   func() time.Time

	// calls holds the timestamps still inside the window, oldest first. It is
	// pruned on every reserve, so it never holds more than limit entries.
	calls []time.Time
}

// newBudget returns a budget. A limit of zero or less disables it, the way
// every other limit in this project does.
func newBudget(limit int, now func() time.Time) *budget {
	if now == nil {
		now = time.Now
	}
	return &budget{limit: limit, now: now}
}

// reserve records a call, or refuses when the window is full.
func (b *budget) reserve() error {
	if b == nil || b.limit <= 0 {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	cutoff := now.Add(-budgetWindow)

	kept := b.calls[:0]
	for _, t := range b.calls {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	b.calls = kept

	if len(b.calls) >= b.limit {
		// The oldest call is the one whose expiry frees a slot.
		retryAfter := b.calls[0].Add(budgetWindow).Sub(now)
		return &Error{
			StatusCode: http.StatusTooManyRequests,
			Message: fmt.Sprintf(
				"local AI budget of %d requests per hour is spent; the next slot frees in %s",
				b.limit, retryAfter.Round(time.Second)),
			// Not retriable: this is a local decision, so retrying the same
			// call immediately cannot help. Backing off is the caller's choice
			// to make deliberately.
			Retriable: false,
		}
	}

	b.calls = append(b.calls, now)
	return nil
}
