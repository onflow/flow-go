package engine

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"go.uber.org/atomic"
)

// TestNotifier_PassByValue verifies that passing Notifier by value is safe
func TestNotifier_PassByValue(t *testing.T) {
	t.Parallel()
	notifier := NewNotifier()

	var sent sync.WaitGroup
	sent.Add(1)
	go func(n Notifier) {
		notifier.Notify()
		sent.Done()
	}(notifier)
	sent.Wait()

	select {
	case <-notifier.Channel(): // expected
	default:
		t.Fail()
	}
}

// TestNotifier_NoNotificationsAtStartup verifies that Notifier is initialized
// without notifications
func TestNotifier_NoNotificationsInitialization(t *testing.T) {
	t.Parallel()
	notifier := NewNotifier()
	select {
	case <-notifier.Channel():
		t.Fail()
	default: // expected
	}
}

// TestNotifier_ManyNotifications sends many notifications to the Notifier
// and verifies that:
//   - the notifier accepts them all without a notification being consumed
//   - only one notification is internally stored and subsequent attempts to
//     read a notification would block
func TestNotifier_ManyNotifications(t *testing.T) {
	t.Parallel()
	notifier := NewNotifier()

	// send 10 notifications
	var counter sync.WaitGroup
	for range 10 {
		counter.Go(func() {
			notifier.Notify()
		})
	}

	// wait for all gorountines to complete
	counter.Wait()

	// attempt to consume first notification:
	// expect that one notification should be available
	c := notifier.Channel()
	select {
	case <-c: // expected
	default:
		t.Error("expected one notification to be available")
	}

	// attempt to consume second notification
	// expect that no notification is available
	select {
	case <-c:
		t.Error("expected only one notification to be available")
	default: // expected
	}
}

// TestNotifier_ManyConsumers spans many worker routines and
// sends just as many notifications with small delays. We require that
// all workers eventually get a notification.
func TestNotifier_ManyConsumers(t *testing.T) {
	singleTestRun := func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			notifier := NewNotifier()
			c := notifier.Channel()

			// spawn 100 worker routines to each wait for a notification
			pendingWorkers := atomic.NewInt32(100)
			for range 100 {
				go func() {
					<-c
					pendingWorkers.Dec()
				}()
			}

			// wait until all workers are blocked on the channel
			synctest.Wait()

			for range 100 {
				notifier.Notify()
			}

			// wait until all workers are done
			synctest.Wait()

			// require that all workers got a notification
			assert.Equal(t, int32(0), pendingWorkers.Load())
		})
	}

	for r := range 100 {
		t.Run(fmt.Sprintf("run %d", r), singleTestRun)
	}
}

// TestNotifier_AllWorkProcessed spans many routines pushing work and fewer
// routines consuming work. We require that all work is eventually processed.
func TestNotifier_AllWorkProcessed(t *testing.T) {
	singleTestRun := func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			notifier := NewNotifier()

			totalWork := int32(100)
			pendingWorkQueue := make(chan struct{}, totalWork)
			scheduledWork := atomic.NewInt32(0)
			consumedWork := atomic.NewInt32(0)

			// starts the consumers first, because if we start the production first instead, then
			// we might finish pushing all jobs, before any of our consumer has started listening
			// to the queue.

			processAllPending := func() {
				for {
					select {
					case <-pendingWorkQueue:
						consumedWork.Inc()
					default:
						return
					}
				}
			}

			// 5 routines consuming work
			for range 5 {
				go func() {
					for {
						select {
						case <-ctx.Done():
							return
						case <-notifier.Channel():
							processAllPending()
						}
					}
				}()
			}

			// wait for all consumer to be ready for new notification.
			synctest.Wait()

			// 10 routines pushing work
			for range 10 {
				go func() {
					for scheduledWork.Load() <= totalWork {
						pendingWorkQueue <- struct{}{}
						scheduledWork.Inc()
						notifier.Notify()
					}
				}()
			}

			// wait for all producers and consumers to block. at this point, all jobs should be completed.
			synctest.Wait()

			// at least the expected number of jobs should have been scheduled, and all jobs that were
			// scheduled should have been consumed.
			assert.GreaterOrEqual(t, scheduledWork.Load(), int32(totalWork))
			assert.Equal(t, scheduledWork.Load(), consumedWork.Load())

			// shutdown blocked consumers and wait for them to complete
			cancel()
			synctest.Wait()
		})
	}

	for r := range 100 {
		t.Run(fmt.Sprintf("run %d", r), singleTestRun)
	}
}
