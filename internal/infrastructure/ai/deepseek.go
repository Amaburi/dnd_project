package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DeepSeekClient implements the Client interface for DeepSeek API
type DeepSeekClient struct {
	config     ClientConfig
	httpClient *http.Client
}

// NewDeepSeekClient creates a new DeepSeek client
func NewDeepSeekClient(config ClientConfig) *DeepSeekClient {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.deepseek.com"
	}
	if config.Model == "" {
		config.Model = "deepseek-chat"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	return &DeepSeekClient{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// ChatCompletion sends a chat completion request to DeepSeek
func (c *DeepSeekClient) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// Set default model if not specified
	if req.Model == "" {
		req.Model = c.config.Model
	}

	// Retry logic
	var lastErr error
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
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

// doRequest performs the actual HTTP request
func (c *DeepSeekClient) doRequest(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
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
		return nil, c.handleErrorResponse(httpResp.StatusCode, respBody)
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
func (c *DeepSeekClient) handleErrorResponse(statusCode int, body []byte) error {
	var errorResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	_ = json.Unmarshal(body, &errorResp)

	retriable := false
	switch statusCode {
	case http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		retriable = true
	}

	return &Error{
		Code:       errorResp.Error.Code,
		Message:    fmt.Sprintf("API error (%d): %s", statusCode, errorResp.Error.Message),
		Retriable:  retriable,
		StatusCode: statusCode,
	}
}

// StreamChatCompletion sends a streaming chat completion request
func (c *DeepSeekClient) StreamChatCompletion(ctx context.Context, req *ChatRequest) (<-chan *ChatStreamResponse, error) {
	// Set streaming flag
	req.Stream = true

	// Set default model if not specified
	if req.Model == "" {
		req.Model = c.config.Model
	}

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
		body, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		return nil, c.handleErrorResponse(httpResp.StatusCode, body)
	}

	// Create response channel
	responseChan := make(chan *ChatStreamResponse, 10)

	// Start goroutine to read stream
	go func() {
		defer close(responseChan)
		defer httpResp.Body.Close()

		scanner := bufio.NewScanner(httpResp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines
			if line == "" {
				continue
			}

			// Skip "data: " prefix
			if len(line) > 6 && line[:6] == "data: " {
				line = line[6:]
			}

			// Check for end of stream
			if line == "[DONE]" {
				break
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
				return
			}
		}
	}()

	return responseChan, nil
}

// Close closes the client
func (c *DeepSeekClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
