package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CORSConfig describes which browser origins may call the API.
type CORSConfig struct {
	// AllowedOrigins is an exact list, or ["*"] to reflect whatever asks.
	// Empty disables CORS entirely, which is right for a server with no
	// browser client.
	AllowedOrigins []string

	// AllowedMethods and AllowedHeaders default to what this API uses.
	AllowedMethods []string
	AllowedHeaders []string

	// MaxAge is how long a browser may cache a preflight.
	MaxAge time.Duration

	// AllowCredentials permits cookies. It cannot be combined with a
	// wildcard origin, and the middleware refuses to do so.
	AllowCredentials bool
}

func (c CORSConfig) withDefaults() CORSConfig {
	if len(c.AllowedMethods) == 0 {
		c.AllowedMethods = []string{
			http.MethodGet, http.MethodPost, http.MethodPut,
			http.MethodDelete, http.MethodOptions,
		}
	}
	if len(c.AllowedHeaders) == 0 {
		c.AllowedHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", RequestIDHeader}
	}
	if c.MaxAge == 0 {
		c.MaxAge = 12 * time.Hour
	}
	return c
}

// CORS answers browser preflights and marks responses as shareable.
//
// Without it a UI on another origin -- the usual localhost:3000 against
// localhost:8080 -- has every call blocked by the browser before the server
// ever sees it.
func CORS(config CORSConfig) gin.HandlerFunc {
	config = config.withDefaults()

	allowed := make(map[string]bool, len(config.AllowedOrigins))
	wildcard := false
	for _, origin := range config.AllowedOrigins {
		if origin == "*" {
			wildcard = true
			continue
		}
		allowed[origin] = true
	}
	enabled := wildcard || len(allowed) > 0

	methods := strings.Join(config.AllowedMethods, ", ")
	headers := strings.Join(config.AllowedHeaders, ", ")
	maxAge := strconv.Itoa(int(config.MaxAge.Seconds()))

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if !enabled || origin == "" {
			c.Next()
			return
		}

		if !wildcard && !allowed[origin] {
			// The request is served anyway; withholding the header is what
			// tells the browser to refuse it. Failing the request here would
			// break non-browser clients that send an Origin.
			c.Next()
			return
		}

		// The requesting origin is echoed rather than "*" so credentialed
		// requests remain possible, and Vary keeps a cache from handing one
		// origin's response to another.
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Vary", "Origin")
		c.Header("Access-Control-Expose-Headers", RequestIDHeader)

		if config.AllowCredentials && !wildcard {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", methods)
			c.Header("Access-Control-Allow-Headers", headers)
			c.Header("Access-Control-Max-Age", maxAge)
			// A preflight is answered here: the route may not exist for
			// OPTIONS at all.
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
