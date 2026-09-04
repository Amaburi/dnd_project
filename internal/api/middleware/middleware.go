// Package middleware holds the cross-cutting HTTP concerns: request
// identification, structured logging, panic recovery, CORS and rate limiting.
//
// They are middleware rather than helpers each handler remembers to call,
// because the ones that matter most are exactly the ones a handler forgets:
// nothing logs a panic that never reached a handler, and nothing rate-limits a
// route somebody added last week.
package middleware

import (
	crand "crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// RequestIDHeader carries the id in and out.
const RequestIDHeader = "X-Request-ID"

// requestIDKey is where the id lives on the context.
const requestIDKey = "request_id"

// RequestID gives every request an id and returns it to the client.
//
// An error a user reports is only findable in the logs if they can quote
// something. An id that arrives on the request is kept, so one call keeps one
// id across a proxy or a front end that already assigned one.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if id == "" {
			id = newRequestID()
		}

		c.Set(requestIDKey, id)
		c.Header(RequestIDHeader, id)
		c.Next()
	}
}

// RequestIDFrom returns the id assigned to this request, or "".
func RequestIDFrom(c *gin.Context) string {
	if id, ok := c.Get(requestIDKey); ok {
		if text, ok := id.(string); ok {
			return text
		}
	}
	return ""
}

func newRequestID() string {
	var buf [12]byte
	if _, err := crand.Read(buf[:]); err != nil {
		// A duplicate id is a worse outcome than an ugly one, so fall back to
		// the clock rather than returning empty.
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf[:])
}

// Logger writes one structured line per request.
//
// It replaces gin's default logger so the output is JSON like the rest of the
// application's, and carries the request id so a line can be matched to a
// client's complaint.
func Logger(logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		event := logger.Info()
		if status := c.Writer.Status(); status >= http.StatusInternalServerError {
			event = logger.Error()
		} else if status >= http.StatusBadRequest {
			event = logger.Warn()
		}

		event.
			Str("request_id", RequestIDFrom(c)).
			Str("method", c.Request.Method).
			Str("path", path).
			Str("query", query).
			Int("status", c.Writer.Status()).
			Dur("took", time.Since(started)).
			Str("client_ip", c.ClientIP()).
			Msg("request")
	}
}

// Recovery turns a panic into a JSON 500 and keeps the server serving.
//
// gin's own recovery writes an empty body, which a JSON client cannot read and
// which tells the user nothing to quote. The panic itself is logged in full
// and never sent: a stack trace in a response is an information leak.
func Recovery(logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error().
					Str("request_id", RequestIDFrom(c)).
					Str("method", c.Request.Method).
					Str("path", c.Request.URL.Path).
					Interface("panic", recovered).
					Bytes("stack", stack()).
					Msg("panic recovered")

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":      "internal server error",
					"request_id": RequestIDFrom(c),
				})
			}
		}()

		c.Next()
	}
}

// ErrorHandler answers a request whose handler recorded an error but wrote
// nothing.
//
// Handlers report unexpected failures with c.Error and usually answer as well;
// this catches the ones that do not, which would otherwise return 200 with an
// empty body. The error text is logged, never sent: it carries connection
// strings and internal detail.
func ErrorHandler(logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		for _, err := range c.Errors {
			logger.Error().
				Str("request_id", RequestIDFrom(c)).
				Str("method", c.Request.Method).
				Str("path", c.Request.URL.Path).
				Int("status", c.Writer.Status()).
				Err(err.Err).
				Msg("request failed")
		}

		// A handler that already answered keeps its own response: it knew
		// more about the failure than this does.
		if c.Writer.Written() {
			return
		}

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":      "internal server error",
			"request_id": RequestIDFrom(c),
		})
	}
}
