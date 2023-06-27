package unittest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// FailOnIrrecoverableError waits for either the done signal, or an error
// to be sent over the error channel. If an error is observed, it is logged
// and the test is failed.
// Must be invoked as a goroutine.
func FailOnIrrecoverableError(t *testing.T, done <-chan struct{}, errCh <-chan error) {
	select {
	case <-done:
	case err := <-errCh:
		assert.NoError(t, err, "observed unexpected irrecoverable error")
	}
}
