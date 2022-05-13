package committees

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestComputeWeightThreshold tests computing the HotStuff safety threshold.
func TestComputeWeightThreshold(t *testing.T) {
	// testing lowest values
	for i := 1; i <= 302; i++ {
		threshold := WeightThresholdToBuildQC(uint64(i))

		boundaryValue := float64(i) * 2.0 / 3.0
		assert.True(t, boundaryValue < float64(threshold))
		assert.False(t, boundaryValue < float64(threshold-1))
	}
}
