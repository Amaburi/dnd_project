package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimitConfig describes how much traffic one client may send.
type RateLimitConfig struct {
	// RequestsPerMinute is the sustained rate. Zero disables limiting, which
	// is the right default for a personal deployment.
	RequestsPerMinute int

	// Burst is how many requests may arrive at once. It defaults to a
	// quarter of the per-minute rate, so a page that fires several calls on
	// load is not punished for it.
	Burst int
}

// bucket is one client's allowance.
//
// A token bucket rather than a fixed window: a window lets a client spend its
// whole budget in the last instant of one window and again in the first of the
// next, which is twice the intended rate at exactly the wrong moment.
type bucket struct {
	tokens float64
	last   time.Time
}

// limiter tracks a bucket per client.
type limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket

	rate  float64 // tokens per second
	burst float64
	now   func() time.Time

	// lastSweep is when idle buckets were last discarded, so a long-running
	// server does not accumulate one entry per address it has ever seen.
	lastSweep time.Time
}

const bucketIdleTTL = 10 * time.Minute

// allow reports whether a request may proceed, and how long to wait if not.
func (l *limiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}

	// Refill for the time that has passed, capped at the burst size.
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens = math.Min(l.burst, b.tokens+elapsed*l.rate)
		b.last = now
	}

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}

	// Tell the caller when one token will exist rather than a flat guess.
	needed := (1 - b.tokens) / l.rate
	wait := time.Duration(math.Ceil(needed)) * time.Second
	if wait < time.Second {
		wait = time.Second
	}
	return false, wait
}

// sweep drops buckets nobody has touched recently.
func (l *limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < bucketIdleTTL {
		return
	}
	l.lastSweep = now

	for key, b := range l.buckets {
		if now.Sub(b.last) > bucketIdleTTL {
			delete(l.buckets, key)
		}
	}
}

// RateLimit limits each client to the configured rate.
func RateLimit(config RateLimitConfig) gin.HandlerFunc {
	return rateLimitWithClock(config, time.Now)
}

// rateLimitWithClock is RateLimit with an injectable clock, because a limiter
// tested against wall time is either slow or flaky.
func rateLimitWithClock(config RateLimitConfig, now func() time.Time) gin.HandlerFunc {
	if config.RequestsPerMinute <= 0 {
		// Not configured means not limited.
		return func(c *gin.Context) { c.Next() }
	}

	burst := config.Burst
	if burst <= 0 {
		burst = config.RequestsPerMinute / 4
	}
	if burst < 1 {
		burst = 1
	}

	l := &limiter{
		buckets:   map[string]*bucket{},
		rate:      float64(config.RequestsPerMinute) / 60.0,
		burst:     float64(burst),
		now:       now,
		lastSweep: now(),
	}

	return func(c *gin.Context) {
		ok, wait := l.allow(c.ClientIP())
		if ok {
			c.Next()
			return
		}

		seconds := int(wait.Seconds())
		if seconds < 1 {
			seconds = 1
		}
		c.Header("Retry-After", strconv.Itoa(seconds))
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"error":       "too many requests",
			"retry_after": seconds,
			"request_id":  RequestIDFrom(c),
		})
	}
}
