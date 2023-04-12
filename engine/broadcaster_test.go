package engine_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestPublish(t *testing.T) {
	t.Parallel()

	t.Run("no subscribers", func(t *testing.T) {
		t.Parallel()
		b := engine.NewBroadcaster()
		unittest.RequireReturnsBefore(t, b.Publish, 100*time.Millisecond, "publish never finished")
	})

	t.Run("all subscribers notified", func(t *testing.T) {
		t.Parallel()
		notifierCount := 10
		recievedCount := atomic.NewInt32(0)

		b := engine.NewBroadcaster()

		// setup subscribers to listen for a notification then return
		subscribers := sync.WaitGroup{}
		subscribers.Add(notifierCount)

		for i := 0; i < notifierCount; i++ {
			notifier := engine.NewNotifier()
			b.Subscribe(notifier)
			go func() {
				defer subscribers.Done()
				<-notifier.Channel()
				recievedCount.Inc()
			}()
		}

		b.Publish()

		unittest.RequireReturnsBefore(t, subscribers.Wait, 100*time.Millisecond, "wait never finished")

		// there should be one notification for each subscriber
		assert.Equal(t, int32(notifierCount), recievedCount.Load())
	})

	t.Run("all subscribers notified at least once", func(t *testing.T) {
		t.Parallel()
		notifierCount := 10
		notifiedCounts := make([]int, notifierCount)

		ctx, cancel := context.WithCancel(context.Background())

		b := engine.NewBroadcaster()

		// setup subscribers to listen for notifications until the context is cancelled
		subscribers := sync.WaitGroup{}
		subscribers.Add(notifierCount)

		for i := 0; i < notifierCount; i++ {
			notifier := engine.NewNotifier()
			b.Subscribe(notifier)

			go func(i int) {
				defer subscribers.Done()

				for {
					select {
					case <-ctx.Done():
						return
					case <-notifier.Channel():
						notifiedCounts[i]++
					}
				}
			}(i)
		}

		// setup publisher to publish notifications concurrently
		publishers := sync.WaitGroup{}
		publishers.Add(20)

		for i := 0; i < 20; i++ {
			go func() {
				defer publishers.Done()
				b.Publish()

				// pause to allow the scheduler to switch to another goroutine
				time.Sleep(time.Millisecond)
			}()
		}

		// wait for publishers to finish, then cancel subscribers' context
		unittest.RequireReturnsBefore(t, publishers.Wait, 100*time.Millisecond, "publishers never finished")
		time.Sleep(100 * time.Millisecond)

		cancel()

		unittest.RequireReturnsBefore(t, subscribers.Wait, 100*time.Millisecond, "receivers never finished")

		// all subscribers should have been notified at least once
		for i, count := range notifiedCounts {
			assert.GreaterOrEqualf(t, count, 1, "notifier %d was not notified", i)
		}
	})
}
