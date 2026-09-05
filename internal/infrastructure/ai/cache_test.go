package ai

import (
	"context"
	"strings"
	"testing"
	"time"
)

func messages(text string) []Message {
	return []Message{
		{Role: "system", Content: "You describe scenes."},
		{Role: "user", Content: text},
	}
}

// The cache is keyed on the exact prompt, which is what makes it safe: two
// different situations produce two different prompts, so a hit can never
// describe the wrong one.
func TestCacheReturnsWhatItStored(t *testing.T) {
	now := time.Now()
	c := newNarrativeCache(10, time.Hour, func() time.Time { return now })

	if _, ok := c.get("dm_base", messages("a crypt")); ok {
		t.Fatal("an empty cache returned something")
	}

	c.put("dm_base", messages("a crypt"), "Cold air rolls up the stairs.")

	got, ok := c.get("dm_base", messages("a crypt"))
	if !ok {
		t.Fatal("the cache did not return what it stored")
	}
	if got != "Cold air rolls up the stairs." {
		t.Errorf("got %q", got)
	}

	// A different prompt is a different entry. This is the whole safety
	// property: a changed fact changes the prompt, so it cannot hit.
	if _, ok := c.get("dm_base", messages("a sunlit meadow")); ok {
		t.Error("a different prompt hit the cache")
	}
	// So is a different template with the same text.
	if _, ok := c.get("npc_dialogue", messages("a crypt")); ok {
		t.Error("a different template hit the cache")
	}
}

// A stale description of a room is worse than no description: the world moves.
func TestCacheEntriesExpire(t *testing.T) {
	now := time.Now()
	c := newNarrativeCache(10, time.Minute, func() time.Time { return now })
	c.put("dm_base", messages("a crypt"), "Cold air.")

	now = now.Add(59 * time.Second)
	if _, ok := c.get("dm_base", messages("a crypt")); !ok {
		t.Error("the entry expired early")
	}

	now = now.Add(2 * time.Second)
	if _, ok := c.get("dm_base", messages("a crypt")); ok {
		t.Error("an expired entry was served")
	}
}

// An unbounded cache in a long-lived process is a leak, so the oldest entries
// go when it is full.
func TestCacheIsBounded(t *testing.T) {
	now := time.Now()
	c := newNarrativeCache(3, time.Hour, func() time.Time { return now })

	for i := 0; i < 10; i++ {
		now = now.Add(time.Second)
		c.put("dm_base", messages(strings.Repeat("x", i+1)), "text")
	}
	if size := c.size(); size > 3 {
		t.Errorf("the cache holds %d entries, want at most 3", size)
	}

	// The newest survives; the oldest does not.
	if _, ok := c.get("dm_base", messages(strings.Repeat("x", 10))); !ok {
		t.Error("the newest entry was evicted")
	}
	if _, ok := c.get("dm_base", messages("x")); ok {
		t.Error("the oldest entry survived eviction")
	}
}

// Zero capacity disables it, the way every other limit in this project does.
func TestACacheOfZeroIsDisabled(t *testing.T) {
	c := newNarrativeCache(0, time.Hour, nil)
	c.put("dm_base", messages("a crypt"), "Cold air.")
	if _, ok := c.get("dm_base", messages("a crypt")); ok {
		t.Error("a disabled cache stored something")
	}
	if c.size() != 0 {
		t.Errorf("a disabled cache holds %d entries", c.size())
	}
}

// A nil cache must behave like a disabled one rather than panicking, because
// that is the configuration a service without caching runs in.
func TestANilCacheIsSafe(t *testing.T) {
	var c *narrativeCache
	c.put("dm_base", messages("a crypt"), "text")
	if _, ok := c.get("dm_base", messages("a crypt")); ok {
		t.Error("a nil cache returned something")
	}
}

// --- the service side --------------------------------------------------------

// The saving that matters: a party walking back into the same room should not
// be billed for describing it again.
func TestRepeatedSceneDescriptionsAreServedFromCache(t *testing.T) {
	service, stub := NewStubService("A cold crypt, its air thick with dust.")
	service.EnableCache(10, time.Hour)

	req := &NarrativeRequest{PlayerInput: "I look around", Location: "a crypt"}

	first, err := service.GenerateNarrative(context.Background(), req)
	if err != nil {
		t.Fatalf("GenerateNarrative: %v", err)
	}
	second, err := service.GenerateNarrative(context.Background(), req)
	if err != nil {
		t.Fatalf("GenerateNarrative (second): %v", err)
	}

	if second.Narrative != first.Narrative {
		t.Errorf("the cached reply differs: %q then %q", first.Narrative, second.Narrative)
	}
	if len(stub.Requests) != 1 {
		t.Errorf("the provider was called %d times, want once", len(stub.Requests))
	}
	// A cache hit is free, and reporting a cost for it would inflate the
	// campaign's spend with money nobody paid.
	if second.Cost != 0 || second.TokensUsed != 0 {
		t.Errorf("a cache hit reported %d tokens and $%v", second.TokensUsed, second.Cost)
	}
	if !second.Cached {
		t.Error("the response does not say it came from the cache")
	}
	if first.Cached {
		t.Error("the first, uncached response claims to be cached")
	}
}

// A different room is a different prompt and must reach the provider.
func TestADifferentSceneIsNotServedFromCache(t *testing.T) {
	service, stub := NewStubService("A crypt.", "A meadow.")
	service.EnableCache(10, time.Hour)
	ctx := context.Background()

	if _, err := service.GenerateNarrative(ctx, &NarrativeRequest{PlayerInput: "look", Location: "a crypt"}); err != nil {
		t.Fatalf("GenerateNarrative: %v", err)
	}
	if _, err := service.GenerateNarrative(ctx, &NarrativeRequest{PlayerInput: "look", Location: "a meadow"}); err != nil {
		t.Fatalf("GenerateNarrative: %v", err)
	}
	if len(stub.Requests) != 2 {
		t.Errorf("the provider was called %d times, want twice", len(stub.Requests))
	}
}

// Narration of a resolved outcome must never be cached. Two identical-looking
// swings are different events, and serving one for the other would describe
// dice that were never rolled.
func TestOutcomeNarrationIsNeverCached(t *testing.T) {
	service, stub := NewStubService("The blade bites.", "The blade bites again.")
	service.EnableCache(10, time.Hour)
	ctx := context.Background()

	req := &NarrationRequest{Facts: attackFacts()}
	if _, err := service.NarrateAction(ctx, req); err != nil {
		t.Fatalf("NarrateAction: %v", err)
	}
	if _, err := service.NarrateAction(ctx, req); err != nil {
		t.Fatalf("NarrateAction: %v", err)
	}
	if len(stub.Requests) != 2 {
		t.Errorf("the provider was called %d times; outcome narration must not be cached",
			len(stub.Requests))
	}
}

// Without EnableCache nothing is cached, so the default costs nothing and
// surprises nobody.
func TestCachingIsOffByDefault(t *testing.T) {
	service, stub := NewStubService("A crypt.", "A crypt again.")
	ctx := context.Background()
	req := &NarrativeRequest{PlayerInput: "look", Location: "a crypt"}

	if _, err := service.GenerateNarrative(ctx, req); err != nil {
		t.Fatalf("GenerateNarrative: %v", err)
	}
	if _, err := service.GenerateNarrative(ctx, req); err != nil {
		t.Fatalf("GenerateNarrative: %v", err)
	}
	if len(stub.Requests) != 2 {
		t.Errorf("the provider was called %d times with caching off, want twice", len(stub.Requests))
	}
}
