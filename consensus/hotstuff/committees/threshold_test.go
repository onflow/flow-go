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
	for i := 1; i <= 302; i++ {
		threshold := WeightThresholdToTimeout(uint64(i))

		boundaryValue := float64(i) * 1.0 / 3.0
		assert.True(t, boundaryValue < float64(threshold))
		assert.False(t, boundaryValue < float64(threshold-1))
	}
}
