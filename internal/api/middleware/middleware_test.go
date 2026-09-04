package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func init() { gin.SetMode(gin.TestMode) }

// engine builds a router with the middleware under test and one route.
func engine(m gin.HandlerFunc, handler gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(m)
	r.GET("/probe", handler)
	r.POST("/probe", handler)
	return r
}

func do(r *gin.Engine, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = "192.0.2.1:1234"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func body(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	out := map[string]any{}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return out
}

// --- request id -------------------------------------------------------------

// An error a user reports is only findable in the logs if the response carried
// an id they can quote.
func TestRequestIDIsGeneratedAndReturned(t *testing.T) {
	r := engine(RequestID(), func(c *gin.Context) {
		if RequestIDFrom(c) == "" {
			t.Error("the handler cannot see the request id")
		}
		c.Status(http.StatusOK)
	})

	rec := do(r, http.MethodGet, "/probe", nil)
	if got := rec.Header().Get(RequestIDHeader); got == "" {
		t.Error("no request id was returned to the client")
	}
}

// A client or proxy that already assigned an id keeps it, so one request has
// one id across every hop.
func TestRequestIDReusesAnIncomingHeader(t *testing.T) {
	r := engine(RequestID(), func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := do(r, http.MethodGet, "/probe", map[string]string{RequestIDHeader: "abc-123"})
	if got := rec.Header().Get(RequestIDHeader); got != "abc-123" {
		t.Errorf("request id = %q, want the incoming abc-123", got)
	}
}

func TestRequestIDsAreDistinct(t *testing.T) {
	r := engine(RequestID(), func(c *gin.Context) { c.Status(http.StatusOK) })

	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		id := do(r, http.MethodGet, "/probe", nil).Header().Get(RequestIDHeader)
		if seen[id] {
			t.Fatalf("request id %q was issued twice", id)
		}
		seen[id] = true
	}
}

// --- recovery and errors ----------------------------------------------------

// gin's own recovery returns an empty body, which a JSON client cannot read.
func TestRecoveryReturnsJSONAndKeepsServing(t *testing.T) {
	logger := zerolog.Nop()
	r := gin.New()
	r.Use(RequestID(), Recovery(logger))
	r.GET("/boom", func(c *gin.Context) { panic("something went wrong") })
	r.GET("/fine", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	rec := do(r, http.MethodGet, "/boom", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if got := body(t, rec)["error"]; got == nil {
		t.Errorf("body = %q, want a JSON error", rec.Body.String())
	}
	// The panic message must not reach the client.
	if got := rec.Body.String(); contains(got, "something went wrong") {
		t.Errorf("the panic message leaked to the client: %s", got)
	}
	if rec.Header().Get(RequestIDHeader) == "" {
		t.Error("a panicking request returned no id to quote")
	}

	// The server keeps serving after a panic.
	if next := do(r, http.MethodGet, "/fine", nil); next.Code != http.StatusOK {
		t.Errorf("the next request returned %d", next.Code)
	}
}

// A handler that records an error but writes nothing would otherwise return
// 200 with an empty body.
func TestErrorHandlerAnswersAnUnwrittenError(t *testing.T) {
	logger := zerolog.Nop()
	r := gin.New()
	r.Use(RequestID(), ErrorHandler(logger))
	r.GET("/silent", func(c *gin.Context) {
		_ = c.Error(errDatabaseUnreachable)
	})

	rec := do(r, http.MethodGet, "/silent", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if contains(rec.Body.String(), "cluster0") {
		t.Errorf("the underlying error leaked: %s", rec.Body.String())
	}
}

// A handler that already answered keeps its own response, errors or not.
func TestErrorHandlerLeavesWrittenResponsesAlone(t *testing.T) {
	logger := zerolog.Nop()
	r := gin.New()
	r.Use(ErrorHandler(logger))
	r.GET("/answered", func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "campaign not found"})
		_ = c.Error(errDatabaseUnreachable)
	})

	rec := do(r, http.MethodGet, "/answered", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want the handler's 404", rec.Code)
	}
	if body(t, rec)["error"] != "campaign not found" {
		t.Errorf("body = %s, want the handler's message", rec.Body.String())
	}
}

// --- CORS -------------------------------------------------------------------

// The UI is served from another origin, so without this the browser blocks
// every call before it reaches the server.
func TestCORSAllowsAConfiguredOrigin(t *testing.T) {
	r := engine(CORS(CORSConfig{AllowedOrigins: []string{"http://localhost:3000"}}),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	rec := do(r, http.MethodGet, "/probe", map[string]string{"Origin": "http://localhost:3000"})
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("allow-origin = %q", got)
	}
	if rec.Header().Get("Vary") == "" {
		t.Error("Vary: Origin is missing, so a cache could serve one origin's response to another")
	}
}

func TestCORSRefusesAnUnknownOrigin(t *testing.T) {
	r := engine(CORS(CORSConfig{AllowedOrigins: []string{"http://localhost:3000"}}),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	rec := do(r, http.MethodGet, "/probe", map[string]string{"Origin": "http://evil.test"})
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("an unlisted origin was allowed: %q", got)
	}
	// The request itself still succeeds; the browser is what enforces CORS.
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestCORSWildcard(t *testing.T) {
	r := engine(CORS(CORSConfig{AllowedOrigins: []string{"*"}}),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	rec := do(r, http.MethodGet, "/probe", map[string]string{"Origin": "http://anywhere.test"})
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://anywhere.test" {
		t.Errorf("allow-origin = %q, want the requesting origin echoed", got)
	}
}

// A preflight must be answered by the middleware; the route may not even exist
// for OPTIONS.
func TestCORSAnswersPreflight(t *testing.T) {
	r := engine(CORS(CORSConfig{AllowedOrigins: []string{"http://localhost:3000"}}),
		func(c *gin.Context) { t.Error("the handler ran for a preflight"); c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodOptions, "/probe", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", rec.Code)
	}
	for _, header := range []string{"Access-Control-Allow-Methods", "Access-Control-Allow-Headers", "Access-Control-Max-Age"} {
		if rec.Header().Get(header) == "" {
			t.Errorf("preflight response is missing %s", header)
		}
	}
}

func TestCORSDisabledWithNoOrigins(t *testing.T) {
	r := engine(CORS(CORSConfig{}), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	rec := do(r, http.MethodGet, "/probe", map[string]string{"Origin": "http://localhost:3000"})
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("CORS headers were sent with no origins configured")
	}
}

// --- rate limiting ----------------------------------------------------------

// A clock the test controls, because a limiter tested against wall time is
// either slow or flaky.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func TestRateLimitAllowsBurstThenRefuses(t *testing.T) {
	clock := &fakeClock{now: time.Now()}
	r := engine(rateLimitWithClock(RateLimitConfig{RequestsPerMinute: 60, Burst: 3}, clock.Now),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	for i := 0; i < 3; i++ {
		if rec := do(r, http.MethodGet, "/probe", nil); rec.Code != http.StatusOK {
			t.Fatalf("request %d returned %d, want 200 within the burst", i+1, rec.Code)
		}
	}

	rec := do(r, http.MethodGet, "/probe", nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("the fourth request returned %d, want 429", rec.Code)
	}
	// A client that is told to wait should be told how long.
	retry := rec.Header().Get("Retry-After")
	if retry == "" {
		t.Error("no Retry-After was sent")
	}
	if seconds, err := strconv.Atoi(retry); err != nil || seconds < 1 {
		t.Errorf("Retry-After = %q, want a positive number of seconds", retry)
	}
}

// The bucket refills, or a limiter is just a fuse.
func TestRateLimitRefillsOverTime(t *testing.T) {
	clock := &fakeClock{now: time.Now()}
	r := engine(rateLimitWithClock(RateLimitConfig{RequestsPerMinute: 60, Burst: 1}, clock.Now),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	if rec := do(r, http.MethodGet, "/probe", nil); rec.Code != http.StatusOK {
		t.Fatalf("the first request returned %d", rec.Code)
	}
	if rec := do(r, http.MethodGet, "/probe", nil); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("the second request returned %d, want 429", rec.Code)
	}

	// 60 per minute is one per second.
	clock.advance(time.Second)
	if rec := do(r, http.MethodGet, "/probe", nil); rec.Code != http.StatusOK {
		t.Errorf("after a second's refill the request returned %d, want 200", rec.Code)
	}
}

// One noisy client must not exhaust everyone else's budget.
func TestRateLimitIsPerClient(t *testing.T) {
	clock := &fakeClock{now: time.Now()}
	limiter := rateLimitWithClock(RateLimitConfig{RequestsPerMinute: 60, Burst: 1}, clock.Now)

	r := gin.New()
	r.Use(limiter)
	r.GET("/probe", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	call := func(ip string) int {
		req := httptest.NewRequest(http.MethodGet, "/probe", nil)
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}

	if got := call("192.0.2.1"); got != http.StatusOK {
		t.Fatalf("first client returned %d", got)
	}
	if got := call("192.0.2.1"); got != http.StatusTooManyRequests {
		t.Fatalf("first client's second request returned %d, want 429", got)
	}
	if got := call("192.0.2.9"); got != http.StatusOK {
		t.Errorf("a second client was refused (%d) because the first was noisy", got)
	}
}

// Zero means no limit, so a personal deployment need not configure one.
func TestRateLimitDisabledWhenUnconfigured(t *testing.T) {
	r := engine(RateLimit(RateLimitConfig{}), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	for i := 0; i < 50; i++ {
		if rec := do(r, http.MethodGet, "/probe", nil); rec.Code != http.StatusOK {
			t.Fatalf("request %d was limited despite no configuration", i+1)
		}
	}
}

// errDatabaseUnreachable stands in for an unexpected internal failure whose
// text must never reach a client.
var errDatabaseUnreachable = errors.New("mongo: connection() to cluster0.mongodb.net failed")

func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }
