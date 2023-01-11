package hotstuff_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/consensus/hotstuff"
)

// Test_ComputeWeightThreshold tests computing the HotStuff safety threshold.
func Test_ComputeWeightThreshold(t *testing.T) {
	// testing lowest values
	for i := 1; i <= 302; i++ {
		threshold := hotstuff.ComputeWeightThresholdForBuildingQC(uint64(i))

		boundaryValue := float64(i) * 2.0 / 3.0
		assert.True(t, boundaryValue < float64(threshold))
		assert.False(t, boundaryValue < float64(threshold-1))
	}
}
