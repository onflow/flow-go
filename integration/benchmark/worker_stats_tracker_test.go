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

	stats := st.getStats()
	assert.Equal(t, 0, stats.txsSent)
	assert.Equal(t, 1, stats.workers)

	st.IncTxSent()
	st.IncTxSent()
	st.IncTxExecuted()

	stats = st.getStats()
	assert.Equal(t, 1, stats.txsExecuted)
	assert.Equal(t, 2, stats.txsSent)
	assert.Equal(t, 0., stats.txsSentMovingAverage)

	st.Stop()
}
