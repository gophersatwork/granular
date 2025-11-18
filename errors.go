package granular

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors
var (
	// ErrCacheMiss is returned when a cache entry is not found.
	ErrCacheMiss = errors.New("cache miss")
)

// ValidationError represents one or more validation errors that occurred
// during key building or write operations.
type ValidationError struct {
	Errors []error
}

// Error implements the error interface.
func (ve *ValidationError) Error() string {
	if len(ve.Errors) == 0 {
		return "validation failed"
	}
	if len(ve.Errors) == 1 {
		return fmt.Sprintf("validation failed: %v", ve.Errors[0])
	}

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("validation failed with %d errors:\n", len(ve.Errors)))
	for i, err := range ve.Errors {
		fmt.Fprintf(&buf, "  %d. %v\n", i+1, err)
	}
	return buf.String()
}

// Unwrap returns the underlying errors for use with errors.Is and errors.As.
// This implements the multi-error unwrap interface introduced in Go 1.20.
func (ve *ValidationError) Unwrap() []error {
	return ve.Errors
}

// newValidationError creates a ValidationError from a slice of errors.
// Returns nil if the slice is empty.
func newValidationError(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return &ValidationError{Errors: errs}
}
