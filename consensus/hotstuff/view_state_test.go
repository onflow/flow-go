package hotstuff

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLevelledForest_AddVertex tests that Vertex can be added twice without problems
func Test_ComputeStakeThreshold(t *testing.T) {
	// testing lowest values
	for i := 1; i <= 302; i++ {
		threshold := ComputeStakeThresholdForBuildingQC(uint64(i))

		boundaryValue := float64(i) * 2.0 / 3.0
		assert.True(t, boundaryValue < float64(threshold))
		assert.False(t, boundaryValue < float64(threshold-1))
	}
}
