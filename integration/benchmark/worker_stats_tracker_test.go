package benchmark

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestWorkerStatsTracker creates a new worker stats,
// adds a couple of transactions there and verifies that average tps is correct.
func TestWorkerStatsTracker(t *testing.T) {
	st := NewWorkerStatsTracker()
	st.StartPrinting(time.Second)
	defer st.StopPrinting()

	st.AddWorker()

	startTime := time.Now()
	endTime := startTime.Add(time.Second)
	assert.Equal(t, 0., st.AvgTPSBetween(startTime, endTime))
	st.AddTxSent()
	st.AddTxSent()
	assert.Equal(t, 2., st.AvgTPSBetween(startTime, endTime))
}
