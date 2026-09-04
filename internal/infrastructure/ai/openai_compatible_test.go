package ai

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

// A blended rate over total tokens misreports every request whose output
// length differs from its input length.
func TestPricingChargesPromptAndCompletionSeparately(t *testing.T) {
	p := Pricing{PromptUSDPerMillion: 0.14, CompletionUSDPerMillion: 0.28}

	// 1M prompt tokens and no output costs the prompt rate.
	if got, want := p.Cost(Usage{PromptTokens: 1_000_000, TotalTokens: 1_000_000}), 0.14; got != want {
		t.Errorf("prompt-only cost = %v, want %v", got, want)
	}

	// 1M completion tokens and no input costs the (higher) completion rate.
	if got, want := p.Cost(Usage{CompletionTokens: 1_000_000, TotalTokens: 1_000_000}), 0.28; got != want {
		t.Errorf("completion-only cost = %v, want %v", got, want)
	}

	// The same total split differently must not cost the same.
	promptHeavy := p.Cost(Usage{PromptTokens: 900, CompletionTokens: 100, TotalTokens: 1000})
	outputHeavy := p.Cost(Usage{PromptTokens: 100, CompletionTokens: 900, TotalTokens: 1000})
	if promptHeavy >= outputHeavy {
		t.Errorf("prompt-heavy (%v) should cost less than output-heavy (%v) for equal totals", promptHeavy, outputHeavy)
	}
}

// Pricing is per-provider configuration, so an unconfigured provider reports
// no cost rather than DeepSeek's old hardcoded rates.
func TestPricingZeroValueReportsNoCost(t *testing.T) {
	var p Pricing
	if got := p.Cost(Usage{PromptTokens: 5000, CompletionTokens: 5000, TotalTokens: 10000}); got != 0 {
		t.Errorf("unconfigured pricing reported %v, want 0", got)
	}
}

func TestClientConfigValidateNamesEveryMissingField(t *testing.T) {
	err := ClientConfig{}.Validate()
	if err == nil {
		t.Fatal("empty config passed validation")
	}
	for _, want := range []string{"api_key", "base_url", "model"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name missing field %q", err, want)
		}
	}

	complete := ClientConfig{APIKey: "k", BaseURL: "https://example.test/v1", Model: "m"}
	if err := complete.Validate(); err != nil {
		t.Errorf("complete config rejected: %v", err)
	}
}

func TestBackoffForIsExponentialAndCapped(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 0},
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{10, 30 * time.Second}, // capped
	}

	for _, tc := range cases {
		if got := backoffFor(tc.attempt); got != tc.want {
			t.Errorf("backoffFor(%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
}

func TestHandleErrorResponseMarksTransientStatusesRetriable(t *testing.T) {
	c := testClient()

	retriable := []int{
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	}
	for _, status := range retriable {
		err := c.handleErrorResponse(status, http.Header{}, []byte(`{"error":{"message":"boom"}}`))
		var aiErr *Error
		if !errors.As(err, &aiErr) {
			t.Fatalf("status %d: got %T, want *Error", status, err)
		}
		if !aiErr.IsRetriable() {
			t.Errorf("status %d should be retriable", status)
		}
	}

	for _, status := range []int{http.StatusUnauthorized, http.StatusBadRequest, http.StatusNotFound} {
		err := c.handleErrorResponse(status, http.Header{}, []byte(`{"error":{"message":"nope"}}`))
		var aiErr *Error
		if !errors.As(err, &aiErr) {
			t.Fatalf("status %d: got %T, want *Error", status, err)
		}
		if aiErr.IsRetriable() {
			t.Errorf("status %d should not be retriable", status)
		}
	}
}

// A non-JSON error body used to produce "API error (500): " with no detail.
func TestHandleErrorResponseKeepsUnparseableBody(t *testing.T) {
	c := testClient()

	err := c.handleErrorResponse(http.StatusBadGateway, http.Header{}, []byte("<html>upstream down</html>"))
	if got := err.Error(); got == "API error (502): " {
		t.Fatalf("error lost the response body entirely: %q", got)
	}
	if want := "upstream down"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not include %q", err, want)
	}
}

// testClient builds a client with just enough config to exercise error paths.
func testClient() *OpenAICompatibleClient {
	return NewOpenAICompatibleClient(ClientConfig{
		APIKey:  "test",
		BaseURL: "https://example.test/v1",
		Model:   "test-model",
	})
}

// A rate-limited provider states exactly when it will accept traffic again;
// an independent backoff curve just spends more of the quota discovering it.
func TestParseRetryAfterAcceptsSecondsAndDates(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{"absent", "", 0},
		{"integer seconds", "3", 3 * time.Second},
		{"fractional seconds", "2.5", 2500 * time.Millisecond},
		{"zero", "0", 0},
		{"negative is ignored", "-5", 0},
		{"garbage is ignored", "soon", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			if tc.header != "" {
				h.Set("Retry-After", tc.header)
			}
			if got := parseRetryAfter(h); got != tc.want {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.header, got, tc.want)
			}
		})
	}

	// An HTTP-date in the future yields a positive delay.
	h := http.Header{}
	h.Set("Retry-After", time.Now().Add(10*time.Second).UTC().Format(http.TimeFormat))
	if got := parseRetryAfter(h); got <= 0 || got > 11*time.Second {
		t.Errorf("parseRetryAfter(http-date) = %v, want ~10s", got)
	}

	// A date in the past must not produce a negative wait.
	h.Set("Retry-After", time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat))
	if got := parseRetryAfter(h); got != 0 {
		t.Errorf("parseRetryAfter(past date) = %v, want 0", got)
	}
}

func TestRetryAfterOverridesBackoff(t *testing.T) {
	rateLimited := &Error{Retriable: true, RetryAfter: 7 * time.Second}

	// Attempt 1 would otherwise back off 1s.
	if got, want := waitBefore(1, rateLimited), 7*time.Second; got != want {
		t.Errorf("waitBefore with Retry-After = %v, want %v", got, want)
	}

	// Without a Retry-After the backoff curve applies.
	if got, want := waitBefore(3, &Error{Retriable: true}), 4*time.Second; got != want {
		t.Errorf("waitBefore without Retry-After = %v, want %v", got, want)
	}

	// A provider naming an absurd delay must not block a turn indefinitely.
	if got, want := waitBefore(1, &Error{Retriable: true, RetryAfter: time.Hour}), maxRetryWait; got != want {
		t.Errorf("waitBefore with huge Retry-After = %v, want the %v cap", got, want)
	}
}

func TestHandleErrorResponseCarriesRetryAfter(t *testing.T) {
	c := testClient()

	h := http.Header{}
	h.Set("Retry-After", "12")

	err := c.handleErrorResponse(http.StatusTooManyRequests, h, []byte(`{"error":{"message":"rate limit reached"}}`))

	var aiErr *Error
	if !errors.As(err, &aiErr) {
		t.Fatalf("got %T, want *Error", err)
	}
	if !aiErr.IsRetriable() {
		t.Error("429 should be retriable")
	}
	if got, want := aiErr.RetryAfter, 12*time.Second; got != want {
		t.Errorf("RetryAfter = %v, want %v", got, want)
	}
}

// temperature: 0 used to vanish from the request body because the field was a
// plain float64 with omitempty, so asking for a deterministic answer silently
// got the provider's default instead. Intent extraction depends on this.
func TestSamplingParametersSurviveSerialisation(t *testing.T) {
	body, err := json.Marshal(ChatRequest{
		Model:       "test-model",
		Messages:    []Message{{Role: "user", Content: "hi"}},
		Temperature: Float(0),
		TopP:        Float(0),
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	temp, ok := decoded["temperature"]
	if !ok {
		t.Fatalf("temperature was dropped from the request: %s", body)
	}
	if temp != float64(0) {
		t.Errorf("temperature = %v, want 0", temp)
	}
	if _, ok := decoded["top_p"]; !ok {
		t.Errorf("top_p was dropped from the request: %s", body)
	}

	// An unset parameter is still omitted, so the provider keeps its default.
	body, _ = json.Marshal(ChatRequest{Model: "test-model"})
	decoded = map[string]any{}
	json.Unmarshal(body, &decoded) //nolint:errcheck
	if _, ok := decoded["temperature"]; ok {
		t.Errorf("an unset temperature should be omitted, got %s", body)
	}
}

func TestJSONObjectFormatIsRequestable(t *testing.T) {
	body, _ := json.Marshal(ChatRequest{
		Model:          "test-model",
		ResponseFormat: JSONObjectFormat(),
	})
	if !strings.Contains(string(body), `"response_format":{"type":"json_object"}`) {
		t.Errorf("response_format missing from %s", body)
	}
}
