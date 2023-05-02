package events

import (
	"context"
	"testing"
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
)

var noop = func(*model.Block) error { return nil }

// TestFinalizationActor_SubscribeDuringConstruction tests that the FinalizationActor
// subscribes to the provided distributor at construction and can subsequently receive notifications.
func TestFinalizationActor_SubscribeDuringConstruction(t *testing.T) {
	dist := pubsub.NewFinalizationDistributor()
	actor := NewFinalizationActor(dist)

	// to ensure the actor is subscribed, create and start the worker, then register the callback
	done := make(chan struct{})
	worker := actor.CreateWorker(func(_ *model.Block) error {
		close(done)
		return nil
	})
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())
	defer cancel()
	go worker(ctx, func() {})
	dist.OnFinalizedBlock(nil)

	unittest.AssertClosesBefore(t, done, time.Second)
}

// TestFinalizationActor_CreateWorker tests that we can create only one worker.
func TestFinalizationActor_CreateWorker(t *testing.T) {
	actor := NewUnsubscribedFinalizationActor()

	// should be able to create a worker
	_ = actor.CreateWorker(noop)
	// should be unable to create worker twice
	defer unittest.ExpectPanic(t)
	_ = actor.CreateWorker(noop)
}
