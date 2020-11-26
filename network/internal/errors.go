package internal

import (
	"fmt"
	"strings"
)

// InvalidEngineError indicates that a non-registered engine is referenced
type InvalidEngineError struct {
	id       string
	senderID string
}

func (e InvalidEngineError) Error() string {
	return fmt.Sprintf("could not find the engine for channel ID: %s", e.id)
}

func NewInvalidEngineError(id string, senderID string) *InvalidEngineError {
	return &InvalidEngineError{
		id:       id,
		senderID: senderID,
	}
}

// IsDialFailureError returns true if the input error contains a wrapped dial failure error
func IsDialFailureError(err error) bool {
	return strings.Contains(err.Error(), "failed to dial")
}
