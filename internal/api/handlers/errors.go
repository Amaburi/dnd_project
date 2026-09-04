package handlers

import (
	"errors"
	"net/http"

	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/gin-gonic/gin"
)

// respondRepoError maps a repository error onto a status code. Validation
// failures are the caller's fault (400) and a missing document is a 404;
// anything else is unexpected and reported as a 500 without leaking the
// underlying driver message.
func respondRepoError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, models.ErrValidation):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, models.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.Error(err) //nolint:errcheck // recorded for the access log
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
