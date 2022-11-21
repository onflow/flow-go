package benchmark

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWorkerStatsTracker creates a new worker stats,
// adds a couple of transactions there and verifies that average tps is correct.
func TestWorkerStatsTracker(t *testing.T) {
	st := NewWorkerStatsTracker(context.Background())
	st.AddWorkers(1)

	stats := st.GetStats()
	assert.Equal(t, 0, stats.TxsSent)
	assert.Equal(t, 1, stats.Workers)

	st.IncTxSent()
	st.IncTxSent()
	st.IncTxExecuted()

	stats = st.GetStats()
	assert.Equal(t, 1, stats.TxsExecuted)
	assert.Equal(t, 2, stats.TxsSent)
	assert.Equal(t, 0., stats.TxsSentMovingAverage)

	st.Stop()
}
