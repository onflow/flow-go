package engine_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine"
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
	<-u.Ready()
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
	<-u.Done()

	require.Equal(t, []string{
		"running", "finish",
		"running", "finish",
		"running", "finish",
	}, logs)
}
