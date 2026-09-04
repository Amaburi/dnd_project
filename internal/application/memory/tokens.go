// Package memory assembles what the AI DM is allowed to remember.
//
// The campaign log grows without limit and a provider's context window does
// not, so something has to choose. The scope decision recorded in
// docs/ARCHITECTURE.md rules out embeddings and semantic retrieval: memory here
// is the last N events plus a rolling summary, ordered by recency, and nothing
// cleverer.
//
// Nothing in this file talks to a provider or a database. Assembling context is
// the part most worth testing and the part hardest to reach through either.
package memory

import "strings"

// EstimateTokens approximates how many tokens a string costs.
//
// It is an estimate on purpose: the real answer depends on the provider's
// tokenizer, and vendoring one per provider to save a few percent of a budget
// would trade a real dependency for an imaginary gain. What matters is the
// direction of the error -- this over-counts rather than under-counts, because
// an over-count wastes a little context and an under-count gets the request
// refused.
//
// English prose runs about four characters per token, but no word is free, so
// the word count is a floor.
func EstimateTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	byLength := (len(text) + 3) / 4
	byWords := len(strings.Fields(text))
	if byWords > byLength {
		return byWords
	}
	return byLength
}
