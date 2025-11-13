package engine

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

// TestTrapdoor_NoNotificationsInitialization verifies that Notifier is initialized without notifications
func TestTrapdoor_NoNotificationsInitialization(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		trapdoor := NewTrapdoor()
		defer trapdoor.Close()

		ch := trapdoor.Channel()

		synctest.Wait() // wait for the channel routine to be started and blocked

		assertNotClosed(t, ch, "expected no notification to be available")
	})
}

// TestTrapdoor_Close verifies that Close() behaves as expected by performing the following test:
// - Channels created before and after Close() are never closed.
// - Calling Activate() after Close() panics.
// - Calling Channel() after Close() does not panic.
func TestTrapdoor_Close(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		trapdoor := NewTrapdoor()

		// get channels before and after Close() is called. neither should ever be closed.
		// each call to Channel() returns a new channel
		beforeClose := trapdoor.Channel()
		trapdoor.Close()
		afterClose := trapdoor.Channel() // this should not panic

		synctest.Wait() // wait for channel routines to be started and blocked

		assertNotClosed(t, beforeClose, "beforeClose channel should not be closed")
		assertNotClosed(t, afterClose, "afterClose channel should not be closed")

		// now call Activate(). this should cause a panic, and neither channel should be closed.
		defer func() {
			assert.NotNil(t, recover(), "expected panic")

			synctest.Wait() // make sure any processing has completed (there shouldn't be any)
			assertNotClosed(t, beforeClose, "beforeClose channel should not be closed")
			assertNotClosed(t, afterClose, "afterClose channel should not be closed")
		}()

		trapdoor.Activate()
	})
}

// TestTrapdoor_ActivateMany calls Activate() many times and verifies that
//   - the trapdoor accepts them all without a notification being consumed
//   - only one notification is internally stored and subsequent attempts to read a notification block
func TestTrapdoor_ActivateMany(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		trapdoor := NewTrapdoor()
		defer trapdoor.Close()

		var counter sync.WaitGroup
		for range 10 {
			counter.Go(func() {
				trapdoor.Activate()
			})
		}
		counter.Wait()

		// Note: because the channel is closed within a goroutine that's started after Channel() is called,
		// there is a race between checking the returned channel, and the goroutine internally calling
		// trapdoor.pass() then sending the signal. Therefore, we must use synctest to ensure that the
		// goroutine is started and blocked.

		// attempt to consume first notification:
		// expect that one notification should be available
		ch := trapdoor.Channel()
		synctest.Wait()
		assertClosed(t, ch, "expected one notification to be available")

		// attempt to consume second notification
		// expect that no notification is available
		ch = trapdoor.Channel()
		synctest.Wait()
		assertNotClosed(t, ch, "expected no notification to be available")
	})
}

// TestTrapdoor_ManyBlockedConsumers spawns many blocked worker rountines, then sends Activate() calls
// for each worker. All workers should wake up without any special synchronization.
//
// This test specifically does not use synctest to avoid any potential special behavior introduced
// by the framework. Instead, a small sleep is used to wait for workers to all block before sending
// Activate() calls.
func TestTrapdoor_ManyBlockedConsumers(t *testing.T) {
	t.Parallel()
	singleTestRun := func(t *testing.T) {
		t.Parallel()
		trapdoor := NewTrapdoor()
		defer trapdoor.Close()

		// spawn 100 worker routines to each wait for a notification
		workerCount := int32(100)
		pendingWorkers := atomic.NewInt32(workerCount)

		for range workerCount {
			ready := make(chan struct{})
			go func() {
				close(ready)
				<-trapdoor.Channel()
				pendingWorkers.Dec()
			}()
			<-ready
		}

		// we are specifically not using synctest to avoid any special behavior that may be introduced.
		// Using a sleep here to ensure that all workers have progressed from launching their goroutines
		// to blocking on the channel.
		time.Sleep(10 * time.Millisecond)

		// send Activate() calls as quickly as possible. trapdoor does not require synchronization
		// in this case. each call should wake up one worker.
		for range workerCount {
			trapdoor.Activate()
		}

		// wait for all workers to complete
		require.Eventually(t, func() bool {
			return pendingWorkers.Load() == 0
		}, 2*time.Second, time.Millisecond)
	}

	for r := range 100 {
		t.Run(fmt.Sprintf("run %d", r), singleTestRun)
	}
}

// TestTrapdoor_AllWorkProcessed spawns many routines pushing work and fewer routines consuming work.
// We require that all work is eventually processed.
func TestTrapdoor_AllWorkProcessed(t *testing.T) {
	t.Parallel()
	singleTestRun := func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			trapdoor := NewTrapdoor()
			defer trapdoor.Close()

			producerCount := int32(10)
			producerJobs := int32(10)
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

			for range producerCount / 2 {
				go func() {
					for {
						select {
						case <-ctx.Done():
							return
						case <-trapdoor.Channel():
							processAllPending()
						}
					}
				}()
			}

			// wait for all consumers to block on the channel
			synctest.Wait()

			for range producerCount {
				go func() {
					for range producerJobs {
						pendingWorkQueue <- struct{}{}
						trapdoor.Activate()
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

func assertClosed(t *testing.T, ch <-chan struct{}, errMsg string) {
	select {
	case <-ch:
	default:
		t.Error(errMsg)
	}
}

func assertNotClosed(t *testing.T, ch <-chan struct{}, errMsg string) {
	select {
	case <-ch:
		t.Error(errMsg)
	default:
	}
}
