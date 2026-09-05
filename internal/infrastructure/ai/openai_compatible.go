package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxStreamLineBytes caps a single server-sent-event line. bufio.Scanner
// defaults to 64KB and reports an error rather than growing, which silently
// truncated long narrative chunks.
const maxStreamLineBytes = 1024 * 1024

// OpenAICompatibleClient implements Client against any OpenAI-compatible
// /chat/completions endpoint -- Groq, DeepSeek, OpenAI, or a local server.
// The provider is chosen entirely by ClientConfig.BaseURL.
type OpenAICompatibleClient struct {
	config     ClientConfig
	httpClient *http.Client

	// budget caps calls to the provider over a rolling hour. Nil-safe and
	// disabled when the limit is zero.
	budget *budget
}

// Validate reports whether the configuration can address a provider at all.
//
// Called at startup so a missing key or endpoint surfaces before a player is
// mid-turn rather than as a 401 on the first narration.
func (c ClientConfig) Validate() error {
	var missing []string
	if c.APIKey == "" {
		missing = append(missing, "api_key")
	}
	if c.BaseURL == "" {
		missing = append(missing, "base_url")
	}
	if c.Model == "" {
		missing = append(missing, "model")
	}
	if len(missing) > 0 {
		return fmt.Errorf("ai configuration incomplete: missing %s", strings.Join(missing, ", "))
	}
	return nil
}

// NewOpenAICompatibleClient creates a client for the configured endpoint.
//
// BaseURL and Model have no defaults on purpose: guessing a provider would
// silently send traffic somewhere the operator did not choose. Validate the
// config first.
func NewOpenAICompatibleClient(config ClientConfig) *OpenAICompatibleClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	return &OpenAICompatibleClient{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		budget: newBudget(config.RequestsPerHour, nil),
	}
}

// ChatCompletion sends a chat completion request
func (c *OpenAICompatibleClient) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// The budget is checked before the retry loop, not inside it: a refusal is
	// a local decision and retrying it would spend the same budget again.
	if err := c.budget.reserve(); err != nil {
		return nil, err
	}

	// Set default model if not specified
	if req.Model == "" {
		req.Model = c.config.Model
	}

	// Retry logic
	var lastErr error
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(waitBefore(attempt, lastErr)):
			}
		}

		resp, err := c.doRequest(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Check if error is retriable
		if aiErr, ok := err.(*Error); ok && !aiErr.IsRetriable() {
			return nil, err
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// maxRetryWait bounds how long a single retry will sleep, including a wait the
// provider asked for. A provider under load can name a very long Retry-After;
// failing the request is better than blocking a player's turn indefinitely.
const maxRetryWait = 30 * time.Second

// backoffFor returns the delay before the given retry attempt: 1s, 2s, 4s,
// capped at maxRetryWait.
func backoffFor(attempt int) time.Duration {
	if attempt < 1 {
		return 0
	}
	backoff := time.Second << (attempt - 1)
	if backoff > maxRetryWait {
		return maxRetryWait
	}
	return backoff
}

// waitBefore chooses how long to sleep before a retry.
//
// A provider that sent Retry-After has told us exactly when it will accept
// traffic again; an independent backoff curve just spends more of a rate limit
// finding that out.
func waitBefore(attempt int, lastErr error) time.Duration {
	var aiErr *Error
	if errors.As(lastErr, &aiErr) && aiErr.RetryAfter > 0 {
		if aiErr.RetryAfter > maxRetryWait {
			return maxRetryWait
		}
		return aiErr.RetryAfter
	}
	return backoffFor(attempt)
}

// parseRetryAfter reads a Retry-After header, which RFC 9110 allows to be
// either a delay in seconds or an HTTP date. Some providers send fractional
// seconds, which the spec does not cover but which parse fine as a float.
func parseRetryAfter(h http.Header) time.Duration {
	value := strings.TrimSpace(h.Get("Retry-After"))
	if value == "" {
		return 0
	}

	if seconds, err := strconv.ParseFloat(value, 64); err == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds * float64(time.Second))
	}

	if when, err := http.ParseTime(value); err == nil {
		if d := time.Until(when); d > 0 {
			return d
		}
	}

	return 0
}

// doRequest performs the actual HTTP request
func (c *OpenAICompatibleClient) doRequest(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// Marshal request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, &Error{
			Code:      "marshal_error",
			Message:   fmt.Sprintf("failed to marshal request: %v", err),
			Retriable: false,
		}
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.config.BaseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, &Error{
			Code:      "request_error",
			Message:   fmt.Sprintf("failed to create request: %v", err),
			Retriable: false,
		}
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	// Send request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &Error{
			Code:      "network_error",
			Message:   fmt.Sprintf("failed to send request: %v", err),
			Retriable: true,
		}
	}
	defer httpResp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, &Error{
			Code:      "read_error",
			Message:   fmt.Sprintf("failed to read response: %v", err),
			Retriable: true,
		}
	}

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(httpResp.StatusCode, httpResp.Header, respBody)
	}

	// Parse response
	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, &Error{
			Code:      "unmarshal_error",
			Message:   fmt.Sprintf("failed to unmarshal response: %v", err),
			Retriable: false,
		}
	}

	return &chatResp, nil
}

// handleErrorResponse handles error responses from the API
func (c *OpenAICompatibleClient) handleErrorResponse(statusCode int, header http.Header, body []byte) error {
	var errorResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	_ = json.Unmarshal(body, &errorResp)

	message := errorResp.Error.Message
	if message == "" {
		// Not the documented error shape; keep a bounded excerpt so the
		// failure is diagnosable instead of an empty string.
		message = strings.TrimSpace(string(body))
		if len(message) > 512 {
			message = message[:512] + "..."
		}
	}

	retriable := false
	switch statusCode {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		retriable = true
	}

	return &Error{
		Code:       errorResp.Error.Code,
		Message:    fmt.Sprintf("API error (%d): %s", statusCode, message),
		Retriable:  retriable,
		StatusCode: statusCode,
		RetryAfter: parseRetryAfter(header),
	}
}

// StreamChatCompletion sends a streaming chat completion request.
//
// Establishing the stream is retried on the same terms as a unary call. Once
// bytes are flowing a retry would have to replay the partial completion, so a
// mid-stream failure ends the stream and is reported on the error channel
// rather than silently closing it.
func (c *OpenAICompatibleClient) StreamChatCompletion(ctx context.Context, req *ChatRequest) (*ChatStream, error) {
	if err := c.budget.reserve(); err != nil {
		return nil, err
	}

	// Set streaming flag
	req.Stream = true

	// Set default model if not specified
	if req.Model == "" {
		req.Model = c.config.Model
	}

	var (
		httpResp *http.Response
		lastErr  error
	)
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(waitBefore(attempt, lastErr)):
			}
		}

		resp, err := c.openStream(ctx, req)
		if err == nil {
			httpResp = resp
			break
		}

		lastErr = err
		if aiErr, ok := err.(*Error); ok && !aiErr.IsRetriable() {
			return nil, err
		}
	}
	if httpResp == nil {
		return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
	}

	// Create response channel
	responseChan := make(chan *ChatStreamResponse, 10)
	stream := &ChatStream{Chunks: responseChan}

	// Start goroutine to read stream
	go func() {
		defer close(responseChan)
		defer httpResp.Body.Close()

		scanner := bufio.NewScanner(httpResp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), maxStreamLineBytes)

		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines and SSE comments
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}

			// Strip the "data: " prefix
			line = strings.TrimPrefix(line, "data: ")

			// Check for end of stream
			if line == "[DONE]" {
				return
			}

			// Parse JSON
			var streamResp ChatStreamResponse
			if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
				continue
			}

			// Send to channel
			select {
			case responseChan <- &streamResp:
			case <-ctx.Done():
				stream.err = ctx.Err()
				return
			}
		}

		// Assigned before the deferred close(responseChan) runs, so a caller
		// that has drained Chunks sees it.
		if err := scanner.Err(); err != nil {
			stream.err = &Error{
				Code:      "stream_error",
				Message:   fmt.Sprintf("stream interrupted: %v", err),
				Retriable: true,
			}
		}
	}()

	return stream, nil
}

// openStream performs one attempt at opening the SSE response.
func (c *OpenAICompatibleClient) openStream(ctx context.Context, req *ChatRequest) (*http.Response, error) {
	// Marshal request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, &Error{
			Code:      "marshal_error",
			Message:   fmt.Sprintf("failed to marshal request: %v", err),
			Retriable: false,
		}
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.config.BaseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, &Error{
			Code:      "request_error",
			Message:   fmt.Sprintf("failed to create request: %v", err),
			Retriable: false,
		}
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	// Send request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &Error{
			Code:      "network_error",
			Message:   fmt.Sprintf("failed to send request: %v", err),
			Retriable: true,
		}
	}

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		return nil, c.handleErrorResponse(httpResp.StatusCode, httpResp.Header, errBody)
	}

	return httpResp, nil
}

// Close closes the client
func (c *OpenAICompatibleClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
