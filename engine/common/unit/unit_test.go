package unit_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/common/unit"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestReadyDone(t *testing.T) {
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())

	u := unit.NewUnit()
	u.Start(ctx)
	unittest.RequireCloseBefore(t, u.Ready(), time.Second, "ready did not close")

	cancel()
	unittest.RequireCloseBefore(t, u.Done(), time.Second, "done did not close")
}

func TestPreReadyDone(t *testing.T) {
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())

	counter := atomic.NewInt32(0)
	u := unit.NewUnitWithReadyDone(func() {
		require.True(t, counter.CompareAndSwap(0, 1))
	}, func() {
		require.True(t, counter.CompareAndSwap(2, 3))
	})
	u.Start(ctx)
	unittest.RequireCloseBefore(t, u.Ready(), time.Second, "ready did not close")
	require.True(t, counter.CompareAndSwap(1, 2))

	cancel()
	unittest.RequireCloseBefore(t, u.Done(), time.Second, "done did not close")
	require.True(t, counter.CompareAndSwap(3, 4))
}

// Test that if a function is run by LaunchPeriodically and
// takes longer than the interval, the next call will be blocked
func TestLaunchPeriod(t *testing.T) {
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())
	lock := sync.Mutex{}

	u := unit.NewUnit()
	u.Start(ctx)
	unittest.RequireCloseBefore(t, u.Ready(), time.Second, "ready did not close")

	logs := make([]string, 0)
	u.LaunchPeriodically(func(_ context.Context) {
		lock.Lock()
		logs = append(logs, "running")
		lock.Unlock()

		time.Sleep(100 * time.Millisecond)

		lock.Lock()
		logs = append(logs, "finish")
		lock.Unlock()
	}, 50*time.Millisecond, 0)

	// 100 * 3 is to ensure enough time for 3 periodic function to finish
	// adding another 30 as buffer to tolerate, and the buffer has to be
	// smaller than 50 in order to avoid another trigger of the periodic function
	<-time.After((100*3 + 30) * time.Millisecond)

	// This can pass
	lock.Lock()
	require.Equal(t, []string{
		"running", "finish",
		"running", "finish",
		"running",
	}, logs)
	lock.Unlock()

	cancel()
	unittest.RequireCloseBefore(t, u.Done(), time.Second, "done did not close")

	require.Equal(t, []string{
		"running", "finish",
		"running", "finish",
		"running", "finish",
	}, logs)
}

func TestLaunchPeriod_Delay(t *testing.T) {
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())

	u := unit.NewUnit()
	u.Start(ctx)
	unittest.RequireCloseBefore(t, u.Ready(), time.Second, "ready did not close")

	// launch f with a large initial delay (30s)
	// the function f should never be invoked, so we use t.Fail
	u.LaunchPeriodically(func(_ context.Context) { t.Fail() }, time.Millisecond, time.Second*30)

	cancel()
	// ensure we can stop the unit quickly (we should not need to wait for initial delay)
	unittest.RequireCloseBefore(t, u.Done(), time.Second, "done did not close")
}
