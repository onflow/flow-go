package hotstuff_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/consensus/hotstuff"
)

// TestLevelledForest_AddVertex tests that Vertex can be added twice without problems
func Test_ComputeStakeThreshold(t *testing.T) {
	// testing lowest values
	for i := 1; i <= 302; i++ {
		threshold := hotstuff.ComputeStakeThresholdForBuildingQC(uint64(i))

		boundaryValue := float64(i) * 2.0 / 3.0
		assert.True(t, boundaryValue < float64(threshold))
		assert.False(t, boundaryValue < float64(threshold-1))
	}
}
