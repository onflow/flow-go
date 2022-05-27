package utils

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

	assert.Equal(t, st.AvgTPSBetween(time.Now().Add(-time.Hour), time.Now()), 0.)
	st.AddTxSent()
	st.AddTxSent()
	assert.Greater(t, st.AvgTPSBetween(time.Now().Add(-time.Hour), time.Now()), 0.)
}
