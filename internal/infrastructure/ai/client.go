package ai

import (
	"context"
	"time"
)

// Client defines the interface for AI providers
type Client interface {
	// ChatCompletion sends a chat completion request
	ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// StreamChatCompletion opens a streaming chat completion. Errors that
	// occur after the stream is established are reported by ChatStream.Err.
	StreamChatCompletion(ctx context.Context, req *ChatRequest) (*ChatStream, error)

	// Close closes the client connection
	Close() error
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Messages         []Message `json:"messages"`
	Model            string    `json:"model,omitempty"`
	Temperature      float64   `json:"temperature,omitempty"`
	MaxTokens        int       `json:"max_tokens,omitempty"`
	TopP             float64   `json:"top_p,omitempty"`
	FrequencyPenalty float64   `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64   `json:"presence_penalty,omitempty"`
	Stream           bool      `json:"stream,omitempty"`
	Stop             []string  `json:"stop,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a completion choice
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatStreamResponse represents a streaming chat response chunk
type ChatStreamResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
}

// StreamChoice represents a streaming choice
type StreamChoice struct {
	Index        int          `json:"index"`
	Delta        MessageDelta `json:"delta"`
	FinishReason *string      `json:"finish_reason"`
}

// MessageDelta represents a message delta in streaming
type MessageDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// ChatStream is an open streaming completion.
//
// Range over Chunks until it closes, then check Err: a transport failure part
// way through a stream closes the channel just like a clean finish, so without
// the Err check a truncated narrative is indistinguishable from a complete one.
type ChatStream struct {
	Chunks <-chan *ChatStreamResponse

	// err is written before Chunks is closed, so any receiver that has
	// observed the close is guaranteed to see it.
	err error
}

// Err returns the error that terminated the stream, if any. Only valid once
// Chunks has been drained to completion.
func (s *ChatStream) Err() error {
	return s.err
}

// ClientConfig holds AI client configuration.
//
// Every field is provider-agnostic: any OpenAI-compatible chat-completions
// endpoint (Groq, DeepSeek, OpenAI, a local llama.cpp server) is reached by
// pointing BaseURL at it and naming a Model it serves. Nothing in this package
// hardcodes a provider.
type ClientConfig struct {
	// Provider is a label used in logs and errors only. It never changes
	// behaviour -- BaseURL decides where requests go.
	Provider   string
	APIKey     string
	BaseURL    string
	Model      string
	Timeout    time.Duration
	MaxRetries int
	Pricing    Pricing
}

// Pricing describes what a provider charges, in USD per million tokens.
//
// Rates differ per provider and per model and change often, so they are
// configuration rather than constants. Both zero means cost estimation is
// disabled and every reported cost is 0.
type Pricing struct {
	PromptUSDPerMillion     float64
	CompletionUSDPerMillion float64
}

// Cost estimates the USD cost of one completion.
func (p Pricing) Cost(usage Usage) float64 {
	prompt := float64(usage.PromptTokens) * p.PromptUSDPerMillion / 1_000_000.0
	completion := float64(usage.CompletionTokens) * p.CompletionUSDPerMillion / 1_000_000.0
	return prompt + completion
}

// Error types
type Error struct {
	Code       string
	Message    string
	Retriable  bool
	StatusCode int

	// RetryAfter carries the provider's own Retry-After header when it sent
	// one. Rate-limited providers say exactly how long to wait; guessing with
	// a backoff curve instead just burns more of the quota.
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	return e.Message
}

// IsRetriable checks if the error is retriable
func (e *Error) IsRetriable() bool {
	return e.Retriable
}
