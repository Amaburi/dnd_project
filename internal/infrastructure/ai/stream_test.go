package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func streamingClient(t *testing.T, handler http.HandlerFunc) *OpenAICompatibleClient {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return NewOpenAICompatibleClient(ClientConfig{
		APIKey:     "test-key",
		BaseURL:    srv.URL,
		Model:      "deepseek-chat",
		Timeout:    5 * time.Second,
		MaxRetries: 0,
	})
}

func TestStreamChatCompletionDeliversChunksUntilDone(t *testing.T) {
	c := streamingClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, line := range []string{
			`: keep-alive comment`,
			`data: {"choices":[{"delta":{"content":"The door "}}]}`,
			``,
			`data: {"choices":[{"delta":{"content":"creaks open."}}]}`,
			`data: [DONE]`,
		} {
			w.Write([]byte(line + "\n")) //nolint:errcheck
		}
	})

	stream, err := c.StreamChatCompletion(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "open the door"}},
	})
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}

	var got strings.Builder
	for chunk := range stream.Chunks {
		if len(chunk.Choices) > 0 {
			got.WriteString(chunk.Choices[0].Delta.Content)
		}
	}

	if err := stream.Err(); err != nil {
		t.Fatalf("stream reported an error on a clean finish: %v", err)
	}
	if want := "The door creaks open."; got.String() != want {
		t.Errorf("streamed text = %q, want %q", got.String(), want)
	}
}

// A stream that dies part way through used to close its channel exactly like a
// clean finish, so a truncated narrative looked complete.
func TestStreamChatCompletionReportsMidStreamFailure(t *testing.T) {
	c := streamingClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(`data: {"choices":[{"delta":{"content":"The door "}}]}` + "\n")) //nolint:errcheck
		w.(http.Flusher).Flush()

		// Break the connection without sending [DONE].
		hijacked, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		hijacked.Close()
	})

	stream, err := c.StreamChatCompletion(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "open the door"}},
	})
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}

	for range stream.Chunks { //nolint:revive // draining
	}

	if stream.Err() == nil {
		t.Fatal("stream ended without [DONE] but Err() is nil; a truncated stream must be detectable")
	}
}

// An SSE line longer than bufio.Scanner's 64KB default used to end the stream
// silently.
func TestStreamChatCompletionHandlesLongLines(t *testing.T) {
	long := strings.Repeat("a", 100_000)

	c := streamingClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(`data: {"choices":[{"delta":{"content":"` + long + `"}}]}` + "\n")) //nolint:errcheck
		w.Write([]byte("data: [DONE]\n"))                                                  //nolint:errcheck
	})

	stream, err := c.StreamChatCompletion(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "describe at length"}},
	})
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}

	var total int
	for chunk := range stream.Chunks {
		if len(chunk.Choices) > 0 {
			total += len(chunk.Choices[0].Delta.Content)
		}
	}

	if err := stream.Err(); err != nil {
		t.Fatalf("stream error on a long line: %v", err)
	}
	if total != len(long) {
		t.Errorf("received %d bytes, want %d", total, len(long))
	}
}

func TestStreamChatCompletionSurfacesNonRetriableStatus(t *testing.T) {
	c := streamingClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"invalid api key"}}`)) //nolint:errcheck
	})

	_, err := c.StreamChatCompletion(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatal("StreamChatCompletion succeeded on a 401, want an error")
	}
	if !strings.Contains(err.Error(), "invalid api key") {
		t.Errorf("error %q does not carry the API message", err)
	}
}
