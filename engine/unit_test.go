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
		time.Sleep(30 * time.Millisecond)
		logs = append(logs, "finish")
	}, 10*time.Millisecond, 0)

	<-time.After((30*3 + 5) * time.Millisecond)
	require.Equal(t, []string{
		"running", "finish",
		"running", "finish",
		"running",
	}, logs)
	<-u.Done()
}
