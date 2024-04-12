package benchmark

import (
	"context"
	"testing"
	"time"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestWorkerImmediate tests that first job is executed immeediately
// and stops in the middle of waiting for the interval.
func TestWorkerImmediate(t *testing.T) {
	t.Parallel()
	t.Run("immediate", func(t *testing.T) {
		done := make(chan struct{})
		w := NewWorker(
			context.Background(),
			0,
			time.Hour,
			func(workerID int) { close(done) },
		)
		w.Start()

		unittest.AssertClosesBefore(t, done, 5*time.Second)
		w.Stop()
	})
}

// TestWorker tests that jobs are executed more than once.
func TestWorker(t *testing.T) {
	t.Parallel()
	t.Run("mulpiple runs", func(t *testing.T) {
		i := atomic.NewInt64(0)
		done := make(chan struct{})
		w := NewWorker(
			context.Background(),
			0,
			time.Millisecond,
			func(workerID int) {
				if i.Inc() == 2 {
					close(done)
				}
			},
		)
		w.Start()

		unittest.AssertClosesBefore(t, done, 5*time.Second)
		w.Stop()
	})
}

// TestWorkerStartStop tests that worker can be started and stopped.
func TestWorkerStartStop(t *testing.T) {
	t.Parallel()
	t.Run("stop w/o start", func(t *testing.T) {
		w := NewWorker(
			context.Background(), 0, time.Second, func(workerID int) {})
		w.Stop()
	})
	t.Run("stop and start", func(t *testing.T) {
		w := NewWorker(
			context.Background(), 0, time.Second, func(workerID int) {})
		w.Start()
		w.Stop()
	})
}
