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

func TestUnsubscribe(t *testing.T) {
	t.Parallel()

	t.Run("unsubscribe removes subscriber", func(t *testing.T) {
		t.Parallel()
		b := engine.NewBroadcaster()

		notifier1 := engine.NewNotifier()
		notifier2 := engine.NewNotifier()
		notifier3 := engine.NewNotifier()

		b.Subscribe(notifier1)
		b.Subscribe(notifier2)
		b.Subscribe(notifier3)

		assert.Equal(t, 3, b.SubscriberCount())

		b.Unsubscribe(notifier2)
		assert.Equal(t, 2, b.SubscriberCount())

		// Verify remaining subscribers still receive notifications
		received1 := make(chan struct{}, 1)
		received3 := make(chan struct{}, 1)

		go func() {
			<-notifier1.Channel()
			received1 <- struct{}{}
		}()
		go func() {
			<-notifier3.Channel()
			received3 <- struct{}{}
		}()

		b.Publish()

		unittest.RequireReturnsBefore(t, func() { <-received1 }, 100*time.Millisecond, "notifier1 should receive")
		unittest.RequireReturnsBefore(t, func() { <-received3 }, 100*time.Millisecond, "notifier3 should receive")
	})

	t.Run("unsubscribe non-existent subscriber is no-op", func(t *testing.T) {
		t.Parallel()
		b := engine.NewBroadcaster()

		notifier1 := engine.NewNotifier()
		notifier2 := engine.NewNotifier()

		b.Subscribe(notifier1)
		assert.Equal(t, 1, b.SubscriberCount())

		// Unsubscribe a notifier that was never subscribed
		b.Unsubscribe(notifier2)
		assert.Equal(t, 1, b.SubscriberCount())
	})

	t.Run("unsubscribe same subscriber twice is no-op", func(t *testing.T) {
		t.Parallel()
		b := engine.NewBroadcaster()

		notifier := engine.NewNotifier()

		b.Subscribe(notifier)
		assert.Equal(t, 1, b.SubscriberCount())

		b.Unsubscribe(notifier)
		assert.Equal(t, 0, b.SubscriberCount())

		// Unsubscribe again should be a no-op
		b.Unsubscribe(notifier)
		assert.Equal(t, 0, b.SubscriberCount())
	})

	t.Run("concurrent subscribe and unsubscribe", func(t *testing.T) {
		t.Parallel()
		b := engine.NewBroadcaster()

		const numOperations = 100
		notifiers := make([]engine.Notifier, numOperations)
		for i := range numOperations {
			notifiers[i] = engine.NewNotifier()
		}

		// Subscribe all notifiers concurrently
		var wg sync.WaitGroup
		wg.Add(numOperations)
		for i := range numOperations {
			go func(n engine.Notifier) {
				defer wg.Done()
				b.Subscribe(n)
			}(notifiers[i])
		}
		wg.Wait()

		assert.Equal(t, numOperations, b.SubscriberCount())

		// Unsubscribe all notifiers concurrently
		wg.Add(numOperations)
		for i := range numOperations {
			go func(n engine.Notifier) {
				defer wg.Done()
				b.Unsubscribe(n)
			}(notifiers[i])
		}
		wg.Wait()

		assert.Equal(t, 0, b.SubscriberCount())
	})
}

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

		for range notifierCount {
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

		for i := range notifierCount {
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

		for range 20 {
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
