package committees

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestComputeQCWeightThreshold tests computing the HotStuff safety threshold
// for producing a QC.
func TestComputeQCWeightThreshold(t *testing.T) {
	// testing lowest values
	for i := 1; i <= 302; i++ {
		threshold := WeightThresholdToBuildQC(uint64(i))

		boundaryValue := float64(i) * 2.0 / 3.0
		assert.True(t, boundaryValue < float64(threshold))
		assert.False(t, boundaryValue < float64(threshold-1))
	}
}

// TestComputeTOWeightThreshold tests computing the HotStuff safety threshold
// for producing a TO.
func TestComputeTOWeightThreshold(t *testing.T) {
	// testing lowest values
	for totalWeight := uint64(1); totalWeight <= 302; totalWeight++ {
		threshold := WeightThresholdToTimeout(totalWeight)

		assert.Greater(t, threshold*3, totalWeight)         // 3*threshold > totalWeight
		assert.LessOrEqual(t, (threshold-1)*3, totalWeight) // 3*(threshold-1) <= totalWeight
	}
}
