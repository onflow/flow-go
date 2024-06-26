package unicast

import (
	"errors"
	"fmt"
)

// ErrMaxRetries  indicates retries completed with max retries without a successful attempt.
type ErrMaxRetries struct {
	attempts uint64
	err      error
}

func (e ErrMaxRetries) Error() string {
	return fmt.Errorf("retries failed max attempts reached %d: %w", e.attempts, e.err).Error()
}

// NewMaxRetriesErr returns a new ErrMaxRetries.
func NewMaxRetriesErr(attempts uint64, err error) ErrMaxRetries {
	return ErrMaxRetries{attempts: attempts, err: err}
}

// IsErrMaxRetries returns whether an error is ErrMaxRetries.
func IsErrMaxRetries(err error) bool {
	var e ErrMaxRetries
	return errors.As(err, &e)
}
