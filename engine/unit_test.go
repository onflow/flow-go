package engine_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestReadyDone(t *testing.T) {
	u := engine.NewUnit()
	<-u.Ready()
	<-u.Done()
}

// Test that if a function is run by LaunchPeriodically and
// takes longer than the interval, the next call will be blocked
func TestLaunchPeriod(t *testing.T) {
	u := engine.NewUnit()
	unittest.RequireCloseBefore(t, u.Ready(), time.Second, "ready did not close")
	logs := make([]string, 0)
	u.LaunchPeriodically(func() {
		logs = append(logs, "running")
		time.Sleep(100 * time.Millisecond)
		logs = append(logs, "finish")
	}, 50*time.Millisecond, 0)

	// 100 * 3 is to ensure enough time for 3 periodic function to finish
	// adding another 30 as buffer to tolerate, and the buffer has to be
	// smaller than 50 in order to avoid another trigger of the periodic function
	<-time.After((100*3 + 30) * time.Millisecond)

	// This can pass
	require.Equal(t, []string{
		"running", "finish",
		"running", "finish",
		"running",
	}, logs)
	unittest.RequireCloseBefore(t, u.Done(), time.Second, "done did not close")

	require.Equal(t, []string{
		"running", "finish",
		"running", "finish",
		"running", "finish",
	}, logs)
}

func TestLaunchPeriod_Delay(t *testing.T) {
	u := engine.NewUnit()
	unittest.RequireCloseBefore(t, u.Ready(), time.Second, "ready did not close")

	// launch f with a large initial delay (30s)
	// the function f should never be invoked, so we use t.Fail
	u.LaunchPeriodically(t.Fail, time.Millisecond, time.Second*30)

	// ensure we can stop the unit quickly (we should not need to wait for initial delay)
	unittest.RequireCloseBefore(t, u.Done(), time.Second, "done did not close")
}
