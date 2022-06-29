package unittest

import (
	"context"
	"testing"

	"github.com/onflow/flow-go/module/irrecoverable"
)

// IrrecoverableContextWithCancel returns an irrecoverable context, the cancel
// function for the context, and the error channel for the context.
func IrrecoverableContextWithCancel(ctx context.Context) (irrecoverable.SignalerContext, context.CancelFunc, <-chan error) {
	parent, cancel := context.WithCancel(ctx)
	irrecoverableCtx, errCh := irrecoverable.WithSignaler(parent)
	return irrecoverableCtx, cancel, errCh
}

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
