package models

import (
	"errors"
	"fmt"
)

// Repositories return errors wrapping these sentinels so the HTTP layer can map
// them onto status codes without inspecting error strings. Anything that wraps
// neither is an unexpected failure and becomes a 500.
var (
	// ErrNotFound reports that the addressed document does not exist.
	ErrNotFound = errors.New("resource not found")

	// ErrValidation reports that the caller supplied unusable input.
	ErrValidation = errors.New("invalid request")
)

// Invalid builds an ErrValidation with a caller-facing explanation.
func Invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrValidation, fmt.Sprintf(format, args...))
}

// NotFound builds an ErrNotFound naming the missing entity.
func NotFound(entity string) error {
	return fmt.Errorf("%s: %w", entity, ErrNotFound)
}
