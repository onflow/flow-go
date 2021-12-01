package util

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithDone(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("cancelling returns context error", func(t *testing.T) {
		done := make(chan struct{})
		doneCtx, doneCancel := WithDone(ctx, done)
		doneCancel()
		select {
		case <-doneCtx.Done():
		default:
			t.Error("expected context to be done")
		}
		assert.Equal(t, context.Canceled, doneCtx.Err())
	})

	t.Run("cancelling parent returns context error", func(t *testing.T) {
		done := make(chan struct{})
		parentCtx, parentCancel := context.WithCancel(ctx)
		doneCtx, _ := WithDone(parentCtx, done)
		parentCancel()
		select {
		case <-doneCtx.Done():
		default:
			t.Error("expected context to be done")
		}
		assert.Equal(t, context.Canceled, doneCtx.Err())
	})

	t.Run("closing done returns ErrChannelClosed", func(t *testing.T) {
		done := make(chan struct{})
		doneCtx, _ := WithDone(ctx, done)
		close(done)
		select {
		case <-doneCtx.Done():
		case <-time.After(100 * time.Millisecond):
			t.Error("expected context to be done")
		}
		assert.Equal(t, ErrChannelClosed, doneCtx.Err())
	})
}
