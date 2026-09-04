package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// Repository errors used to reach the client as 500s regardless of cause.
func TestRespondRepoErrorMapsStatusCodes(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		want     int
		wantBody string
	}{
		{
			name:     "validation failure is the caller's fault",
			err:      models.Invalid("campaign title is required"),
			want:     http.StatusBadRequest,
			wantBody: "campaign title is required",
		},
		{
			name:     "missing document is a 404",
			err:      models.NotFound("campaign"),
			want:     http.StatusNotFound,
			wantBody: "campaign",
		},
		{
			name:     "wrapped validation failure still maps",
			err:      fmt.Errorf("update failed: %w", models.Invalid("character name is required")),
			want:     http.StatusBadRequest,
			wantBody: "character name is required",
		},
		{
			name:     "anything else is an opaque 500",
			err:      errors.New("connection reset by peer"),
			want:     http.StatusInternalServerError,
			wantBody: "internal server error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

			respondRepoError(c, tc.err)

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
			if !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("body %q does not contain %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
}

// An unexpected error must not leak driver internals to the client.
func TestRespondRepoErrorHidesInternalDetail(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	respondRepoError(c, errors.New("mongo: connection() to cluster0.mongodb.net failed"))

	if strings.Contains(rec.Body.String(), "mongodb.net") {
		t.Errorf("response leaked the underlying error: %s", rec.Body.String())
	}
	if len(c.Errors) == 0 {
		t.Error("error was not recorded on the gin context for logging")
	}
}
