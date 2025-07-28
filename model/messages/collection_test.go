package messages_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestClusterProposal_DeclareStructurallyValid verifies the behavior of the DeclareStructurallyValid converting for untrusted cluster.Proposal.
// It ensures proper handling of both valid and invalid input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedClusterProposal results in a valid cluster.Proposal.
//
// 2. Invalid input with invalid cluster.Block:
//   - Ensures an error is returned when the UnsignedBlock.ParentID is zero.
//
// 3. Invalid input with nil ProposerSigData:
//   - Ensures an error is returned when the ProposerSigData is nil.
//
// 4. Invalid input with empty ProposerSigData:
//   - Ensures an error is returned when the ProposerSigData is an empty byte slice.
func TestClusterProposal_DeclareStructurallyValid(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		untrustedProposal := messages.UntrustedClusterProposal(*unittest.ClusterProposalFixture())
		res, err := untrustedProposal.DeclareStructurallyValid()
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with invalid block", func(t *testing.T) {
		untrustedProposal := messages.UntrustedClusterProposal(*unittest.ClusterProposalFixture())
		untrustedProposal.Block.ParentID = flow.ZeroID

		res, err := untrustedProposal.DeclareStructurallyValid()
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "could not build cluster block")
	})

	t.Run("invalid input with nil ProposerSigData", func(t *testing.T) {
		untrustedProposal := messages.UntrustedClusterProposal(*unittest.ClusterProposalFixture())
		untrustedProposal.ProposerSigData = nil

		res, err := untrustedProposal.DeclareStructurallyValid()
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "proposer signature must not be empty")
	})

	t.Run("invalid input with empty ProposerSigData", func(t *testing.T) {
		untrustedProposal := messages.UntrustedClusterProposal(*unittest.ClusterProposalFixture())
		untrustedProposal.ProposerSigData = []byte{}

		res, err := untrustedProposal.DeclareStructurallyValid()
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "proposer signature must not be empty")
	})
}
