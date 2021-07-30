package votecollector

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff"
)

// TestVerifyingVoteCollector_ProcessingStatus tests that processing status is expected
func TestVerifyingVoteCollector_ProcessingStatus(t *testing.T) {
	collector := NewVerifyingVoteCollector(BaseVoteCollector{})
	require.Equal(t, hotstuff.VerifyingVotes, collector.ProcessingStatus())
}
