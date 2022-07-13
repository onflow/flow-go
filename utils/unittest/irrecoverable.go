package unittest

import (
	"testing"
)

// FailOnIrrecoverableError waits for either the done signal, or an error
// to be sent over the error channel. If an error is observed, it is logged
// and the test is failed.
// Must be invoked as a goroutine.
func FailOnIrrecoverableError(t *testing.T, done <-chan struct{}, errCh <-chan error) {
	select {
	case <-done:
		return
	case err := <-errCh:
		t.Log("observed unexpected irrecoverable error: ", err)
		t.Fail()
		return
	}
}
