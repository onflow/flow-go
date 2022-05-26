package utils

import (
	"testing"
	"time"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestWorker creates a new worker and test that it runs once and stops.
func TestWorker(t *testing.T) {
	done := make(chan struct{})
	w := NewWorker(0, time.Hour, func(workerID int) { close(done) })
	w.Start()

	unittest.AssertClosesBefore(t, done, 5*time.Second)
	w.Stop()
}
