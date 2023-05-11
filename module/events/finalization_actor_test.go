package events

import (
	"context"
	"testing"
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestFinalizationActor_SubscribeDuringConstruction tests that the FinalizationActor
// subscribes to the provided distributor at construction and can subsequently receive notifications.
func TestFinalizationActor_SubscribeDuringConstruction(t *testing.T) {

	// to ensure the actor is subscribed, create and start the worker, then register the callback
	done := make(chan struct{})
	actor, worker := NewFinalizationActor(func(_ *model.Block) error {
		close(done)
		return nil
	})
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())
	defer cancel()
	go worker(ctx, func() {})
	actor.OnFinalizedBlock(nil)

	unittest.AssertClosesBefore(t, done, time.Second)
}
