package messages_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestProposal_DeclareStructurallyValid verifies the behavior of the DeclareStructurallyValid converting for untrusted Proposal.
// It ensures proper handling of both valid and invalid input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedProposal results in a valid Proposal.
//
// 2. Invalid input with invalid Block:
//   - Ensures an error is returned when the Block.ParentID is zero.
//
// 3. Invalid input with nil ProposerSigData:
//   - Ensures an error is returned when the ProposerSigData is nil.
//
// 4. Invalid input with empty ProposerSigData:
//   - Ensures an error is returned when the ProposerSigData is an empty byte slice.
func TestProposal_DeclareStructurallyValid(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		untrustedProposal := messages.UntrustedProposal(*unittest.ProposalFixture())
		res, err := untrustedProposal.DeclareStructurallyValid()
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with invalid block", func(t *testing.T) {
		untrustedProposal := messages.UntrustedProposal(*unittest.ProposalFixture())
		untrustedProposal.Block.Header.ParentID = flow.ZeroID

		res, err := untrustedProposal.DeclareStructurallyValid()
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "could not build block")
	})

	t.Run("invalid input with nil ProposerSigData", func(t *testing.T) {
		untrustedProposal := messages.UntrustedProposal(*unittest.ProposalFixture())
		untrustedProposal.ProposerSigData = nil

		res, err := untrustedProposal.DeclareStructurallyValid()
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "proposer signature must not be empty")
	})

	t.Run("invalid input with empty ProposerSigData", func(t *testing.T) {
		untrustedProposal := messages.UntrustedProposal(*unittest.ProposalFixture())
		untrustedProposal.ProposerSigData = []byte{}

		res, err := untrustedProposal.DeclareStructurallyValid()
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "proposer signature must not be empty")
	})
}
