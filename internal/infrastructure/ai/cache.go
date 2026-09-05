package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// narrativeCache serves a reply the provider has already given for the exact
// same prompt.
//
// **Keyed on the whole rendered prompt**, which is what makes it safe. Two
// different situations produce two different prompts, so a hit can never
// describe the wrong one -- there is no clever key to get wrong, because the
// key is everything the model was told.
//
// It is an in-process map behind a mutex, which is what docs/ARCHITECTURE.md §0
// says to reach for when caching is ever needed. Redis was cut; a shared cache
// for a single-process personal campaign would be infrastructure to run and
// nothing more.
type narrativeCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
	order   []string

	capacity int
	ttl      time.Duration
	now      func() time.Time
}

type cacheEntry struct {
	text     string
	storedAt time.Time
}

// newNarrativeCache returns a cache. A capacity of zero or less disables it.
func newNarrativeCache(capacity int, ttl time.Duration, now func() time.Time) *narrativeCache {
	if now == nil {
		now = time.Now
	}
	return &narrativeCache{
		entries:  make(map[string]cacheEntry),
		capacity: capacity,
		ttl:      ttl,
		now:      now,
	}
}

// enabled reports whether this cache stores anything. A nil cache is a disabled
// one, so a service without caching needs no branch at every call site.
func (c *narrativeCache) enabled() bool { return c != nil && c.capacity > 0 }

// cacheKey is the fingerprint of a prompt.
//
// Hashed rather than stored whole: prompts carry the character sheet and the
// campaign history, and keeping a second copy of all of it in a map would cost
// more memory than the replies it is saving.
func cacheKey(template string, messages []Message) string {
	h := sha256.New()
	h.Write([]byte(template))
	for _, m := range messages {
		h.Write([]byte{0})
		h.Write([]byte(m.Role))
		h.Write([]byte{0})
		h.Write([]byte(m.Content))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// get returns a stored reply for this exact prompt.
func (c *narrativeCache) get(template string, messages []Message) (string, bool) {
	if !c.enabled() {
		return "", false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey(template, messages)
	entry, ok := c.entries[key]
	if !ok {
		return "", false
	}
	// A stale description of a room is worse than no description: the world
	// moves, and the party notices when it does not.
	if c.now().Sub(entry.storedAt) > c.ttl {
		c.remove(key)
		return "", false
	}
	return entry.text, true
}

// put stores a reply against the prompt that produced it.
func (c *narrativeCache) put(template string, messages []Message, text string) {
	if !c.enabled() || text == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey(template, messages)
	if _, exists := c.entries[key]; !exists {
		c.order = append(c.order, key)
	}
	c.entries[key] = cacheEntry{text: text, storedAt: c.now()}

	// An unbounded cache in a long-lived process is a leak, so the oldest go.
	for len(c.order) > c.capacity {
		c.remove(c.order[0])
	}
}

// remove drops one entry. The caller holds the lock.
func (c *narrativeCache) remove(key string) {
	delete(c.entries, key)
	for i, held := range c.order {
		if held == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

// size is how many entries are held, for tests and reporting.
func (c *narrativeCache) size() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// cacheableTemplates are the prompts whose replies may be reused.
//
// Scene description only, and deliberately so. Narration of a resolved outcome
// is excluded even though the prompt key would make it safe: two swings that
// happen to produce identical facts are still two different moments, and
// hearing the same sentence twice is how a table notices it is talking to a
// machine. Dialogue is excluded for the same reason.
//
// The saving that matters is a party walking back into a room they have already
// had described, which is common and costs a full call every time.
var cacheableTemplates = map[string]bool{
	"dm_base":              true,
	"narrative_generation": true,
}

// cachedCompletion runs a prompt, serving a stored reply when one exists.
//
// Returns the text, whether it came from the cache, and the usage. A hit
// reports no tokens and no cost, because none were spent -- charging for one
// would inflate the campaign's reported spend with money nobody paid.
func (s *Service) cachedCompletion(
	ctx context.Context,
	template string,
	req *ChatRequest,
) (text string, cached bool, usage Usage, err error) {
	if cacheableTemplates[template] {
		if hit, ok := s.cache.get(template, req.Messages); ok {
			return hit, true, Usage{}, nil
		}
	}

	resp, err := s.client.ChatCompletion(ctx, req)
	if err != nil {
		return "", false, Usage{}, err
	}
	if len(resp.Choices) == 0 {
		return "", false, Usage{}, fmt.Errorf("no response from AI")
	}

	text = resp.Choices[0].Message.Content
	if cacheableTemplates[template] {
		s.cache.put(template, req.Messages, text)
	}
	return text, false, resp.Usage, nil
}

// EnableCache turns on reuse of scene descriptions.
//
// Off by default: caching changes what the model says on a second identical
// prompt, and that should be a choice rather than a surprise.
func (s *Service) EnableCache(capacity int, ttl time.Duration) {
	s.cache = newNarrativeCache(capacity, ttl, nil)
}
