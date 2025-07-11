package messages_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestClusterProposal_DeclareTrusted verifies the behavior of the DeclareTrusted converting for untrusted cluster.Proposal.
// It ensures proper handling of both valid and invalid input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedClusterProposal results in a valid cluster.Proposal.
//
// 2. Invalid input with invalid cluster.Block:
//   - Ensures an error is returned when the Block.ParentID is zero.
//
// 3. Invalid input with nil ProposerSigData:
//   - Ensures an error is returned when the ProposerSigData is nil.
//
// 4. Invalid input with empty ProposerSigData:
//   - Ensures an error is returned when the ProposerSigData is an empty byte slice.
func TestClusterProposal_DeclareTrusted(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		untrustedProposal := messages.UntrustedClusterProposal(*unittest.ClusterProposalFixture())
		res, err := untrustedProposal.DeclareTrusted()
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with invalid block", func(t *testing.T) {
		untrustedProposal := messages.UntrustedClusterProposal(*unittest.ClusterProposalFixture())
		untrustedProposal.Block.Header.ParentID = flow.ZeroID

		res, err := untrustedProposal.DeclareTrusted()
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "could not build cluster block")
	})

	t.Run("invalid input with nil ProposerSigData", func(t *testing.T) {
		untrustedProposal := messages.UntrustedClusterProposal(*unittest.ClusterProposalFixture())
		untrustedProposal.ProposerSigData = nil

		res, err := untrustedProposal.DeclareTrusted()
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "proposer signature must not be empty")
	})

	t.Run("invalid input with empty ProposerSigData", func(t *testing.T) {
		untrustedProposal := messages.UntrustedClusterProposal(*unittest.ClusterProposalFixture())
		untrustedProposal.ProposerSigData = []byte{}

		res, err := untrustedProposal.DeclareTrusted()
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "proposer signature must not be empty")
	})
}
