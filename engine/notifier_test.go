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
		n.Notify()
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

	var counter sync.WaitGroup
	for range 10 {
		counter.Go(func() {
			notifier.Notify()
		})
	}
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

// TestNotifier_ManyConsumers spans many worker routines and sends just as many notifications.
// We require that all workers eventually get a notification.
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

			// wait until all workers are blocked on the notifier channel
			synctest.Wait()

			for range 100 {
				notifier.Notify()

				// wait for the previous notification to be consumed to ensure that the producer
				// won't drop any notifications.
				// NOTE: this is necessary because golang channels do not provide atomic consistency.
				// Specifically, it means that when we send a notification while workers are waiting
				// it is not guaranteed that a worker will atomically receive that notification. In
				// other words, the channel might behave as if there was no worker waiting and de-
				// duplicating notifications. For details, see
				// https://www.notion.so/flowfoundation/Golang-Channel-Consistency-19a1aee12324817699b1ff162921d8fc
				synctest.Wait()
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

			notifier := NewNotifier()

			producerCount := int32(10) // number of producers
			producerJobs := int32(10)  // number of tasks that each producer will queue up
			pendingWorkQueue := make(chan struct{}, producerCount*producerJobs)
			consumedWork := atomic.NewInt32(0)

			// start the consumers first, otherwise we might finish pushing all jobs, before any of
			// our consumer has started listening to the queue.

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

			// wait for all consumers to block on the notifier channel
			synctest.Wait()

			for range producerCount {
				go func() {
					for range producerJobs {
						pendingWorkQueue <- struct{}{}
						notifier.Notify()
					}
				}()
			}

			// wait for all producers and consumers to block. at this point, all jobs should be consumed.
			synctest.Wait()

			// verify all scheduled jobs were consumed.
			assert.Equal(t, producerCount*producerJobs, consumedWork.Load())

			// shutdown blocked consumers and wait for them to complete
			cancel()
			synctest.Wait()
		})
	}

	for r := range 100 {
		t.Run(fmt.Sprintf("run %d", r), singleTestRun)
	}
}
