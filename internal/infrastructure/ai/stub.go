package ai

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// StubClient is a Client that returns canned replies instead of calling a
// provider.
//
// Calling a real model to check that a prompt was assembled correctly is slow
// and costs money for an answer that tells you nothing about your own code. The
// stub lets every test of prompt assembly, intent parsing and the narration
// contract run offline and free; only the quality of the model's prose needs a
// real call.
type StubClient struct {
	mu sync.Mutex

	// Replies are returned in order, the last one repeating once exhausted.
	Replies []string

	// Err, when set, is returned instead of a reply.
	Err error

	// Requests records every request received, so a test can assert on what
	// the prompt actually said.
	Requests []*ChatRequest
}

// NewStubClient returns a stub that answers with the given replies in order.
func NewStubClient(replies ...string) *StubClient {
	return &StubClient{Replies: replies}
}

// LastRequest returns the most recent request, or nil.
func (c *StubClient) LastRequest() *ChatRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.Requests) == 0 {
		return nil
	}
	return c.Requests[len(c.Requests)-1]
}

// LastPrompt returns the system and user messages of the most recent request
// joined together, which is what an assertion about wording wants.
func (c *StubClient) LastPrompt() string {
	req := c.LastRequest()
	if req == nil {
		return ""
	}
	parts := make([]string, 0, len(req.Messages))
	for _, m := range req.Messages {
		parts = append(parts, m.Content)
	}
	return strings.Join(parts, "\n---\n")
}

// CallCount returns how many requests the stub has received.
func (c *StubClient) CallCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.Requests)
}

func (c *StubClient) next() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Err != nil {
		return "", c.Err
	}
	if len(c.Replies) == 0 {
		return "", fmt.Errorf("stub client has no replies configured")
	}

	index := len(c.Requests) - 1
	if index >= len(c.Replies) {
		index = len(c.Replies) - 1
	}
	return c.Replies[index], nil
}

// ChatCompletion records the request and returns the next canned reply.
func (c *StubClient) ChatCompletion(_ context.Context, req *ChatRequest) (*ChatResponse, error) {
	c.mu.Lock()
	c.Requests = append(c.Requests, req)
	c.mu.Unlock()

	reply, err := c.next()
	if err != nil {
		return nil, err
	}

	return &ChatResponse{
		Model:   req.Model,
		Choices: []Choice{{Message: Message{Role: "assistant", Content: reply}, FinishReason: "stop"}},
		// Plausible non-zero usage, so cost reporting is exercised too.
		Usage: Usage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
	}, nil
}

// StreamChatCompletion delivers the canned reply as a single chunk.
func (c *StubClient) StreamChatCompletion(_ context.Context, req *ChatRequest) (*ChatStream, error) {
	c.mu.Lock()
	c.Requests = append(c.Requests, req)
	c.mu.Unlock()

	reply, err := c.next()
	if err != nil {
		return nil, err
	}

	chunks := make(chan *ChatStreamResponse, 1)
	chunks <- &ChatStreamResponse{
		Model:   req.Model,
		Choices: []StreamChoice{{Delta: MessageDelta{Content: reply}}},
	}
	close(chunks)

	return &ChatStream{Chunks: chunks}, nil
}

// Close does nothing.
func (c *StubClient) Close() error { return nil }

// NewStubService builds a Service backed by a stub client, for tests and
// offline demos.
func NewStubService(replies ...string) (*Service, *StubClient) {
	stub := NewStubClient(replies...)
	service := &Service{
		client:        stub,
		promptBuilder: NewPromptBuilder(),
		config: ClientConfig{
			Provider: "stub", APIKey: "stub", BaseURL: "stub", Model: "stub-model",
			Pricing: Pricing{PromptUSDPerMillion: 0.59, CompletionUSDPerMillion: 0.79},
		},
	}
	return service, stub
}
