package cluster_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestClusterBlockMalleability checks that cluster.Block is not malleable: any change in its data
// should result in a different ID.
// Because our NewHeaderBody constructor enforces ParentView < View we use
// WithFieldGenerator to safely pass it.
func TestClusterBlockMalleability(t *testing.T) {
	clusterBlock := unittest.ClusterBlockFixture()
	unittest.RequireEntityNonMalleable(
		t,
		clusterBlock,
		unittest.WithFieldGenerator("HeaderBody.ParentView", func() uint64 {
			return clusterBlock.View - 1 // ParentView must stay below View, so set it to View-1
		}),
		unittest.WithFieldGenerator("Payload.Collection", func() flow.Collection {
			return unittest.CollectionFixture(3)
		}),
	)
}

// TestNewBlock verifies the behavior of the NewBlock constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedBlock results in a valid Block.
//
// 2. Invalid input with invalid HeaderBody:
//   - Ensures an error is returned when the HeaderBody.ParentID is flow.ZeroID.
//
// 3. Invalid input with invalid Payload:
//   - Ensures an error is returned when the Payload contains a Collection with invalid transaction IDs.
func TestNewBlock(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		block := unittest.ClusterBlockFixture()

		res, err := cluster.NewBlock(cluster.UntrustedBlock(*block))
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with invalid header body", func(t *testing.T) {
		block := unittest.ClusterBlockFixture()
		block.ParentID = flow.ZeroID

		res, err := cluster.NewBlock(cluster.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid header body")
	})

	t.Run("invalid input with invalid payload", func(t *testing.T) {
		block := unittest.ClusterBlockFixture()
		collection := unittest.CollectionFixture(5)
		collection.Transactions[2] = nil
		block.Payload.Collection = collection

		res, err := cluster.NewBlock(cluster.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid cluster payload")
	})
}

// TestNewRootBlock verifies the behavior of the NewRootBlock constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedBlock results in a valid root Block.
//
// 2. Invalid input with invalid HeaderBody:
//   - Ensures an error is returned when the HeaderBody.ParentView is not zero.
//
// 3. Invalid input with invalid ParentID:
//   - Ensures an error is returned when the HeaderBody.ParentID is not zero.
//
// 4. Invalid input with invalid Payload:
//   - Ensures an error is returned when the Payload.ReferenceBlockID is not flow.ZeroID.
func TestNewRootBlock(t *testing.T) {
	// validRootBlockFixture returns a new valid root cluster.UntrustedBlock for use in tests.
	validRootBlockFixture := func() cluster.UntrustedBlock {
		return cluster.UntrustedBlock{
			HeaderBody: flow.HeaderBody{
				ChainID:            flow.Emulator,
				ParentID:           flow.ZeroID,
				Height:             10,
				Timestamp:          uint64(time.Now().UnixMilli()),
				View:               0,
				ParentView:         0,
				ParentVoterIndices: []byte{},
				ParentVoterSigData: []byte{},
				ProposerID:         flow.ZeroID,
				LastViewTC:         nil,
			},
			Payload: *cluster.NewEmptyPayload(flow.ZeroID),
		}
	}

	t.Run("valid input", func(t *testing.T) {
		res, err := cluster.NewRootBlock(validRootBlockFixture())
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with invalid header body", func(t *testing.T) {
		block := validRootBlockFixture()
		block.ParentView = 1

		res, err := cluster.NewRootBlock(block)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid root header body")
	})

	t.Run("invalid input with invalid ParentID", func(t *testing.T) {
		block := validRootBlockFixture()
		block.ParentID = unittest.IdentifierFixture()

		res, err := cluster.NewRootBlock(block)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "ParentID must be zero")
	})

	t.Run("invalid input with invalid payload", func(t *testing.T) {
		block := validRootBlockFixture()
		block.Payload.ReferenceBlockID = unittest.IdentifierFixture()

		res, err := cluster.NewRootBlock(block)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid root cluster payload")
	})
}

// TestNewProposal verifies the behavior of the NewProposal constructor.
// It ensures proper handling of both valid and invalid input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedProposal results in a valid Proposal.
//
// 2. Invalid input with invalid Block:
//   - Ensures an error is returned when the Block.ParentID is flow.ZeroID.
//
// 3. Invalid input with nil ProposerSigData:
//   - Ensures an error is returned when the ProposerSigData is nil.
//
// 4. Invalid input with empty ProposerSigData:
//   - Ensures an error is returned when the ProposerSigData is an empty byte slice.
func TestNewProposal(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		res, err := cluster.NewProposal(cluster.UntrustedProposal(*unittest.ClusterProposalFixture()))
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with invalid block", func(t *testing.T) {
		untrustedProposal := cluster.UntrustedProposal(*unittest.ClusterProposalFixture())
		untrustedProposal.Block.ParentID = flow.ZeroID

		res, err := cluster.NewProposal(untrustedProposal)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid block")
	})

	t.Run("invalid input with nil ProposerSigData", func(t *testing.T) {
		untrustedProposal := cluster.UntrustedProposal(*unittest.ClusterProposalFixture())
		untrustedProposal.ProposerSigData = nil

		res, err := cluster.NewProposal(untrustedProposal)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "proposer signature must not be empty")
	})

	t.Run("invalid input with empty ProposerSigData", func(t *testing.T) {
		untrustedProposal := cluster.UntrustedProposal(*unittest.ClusterProposalFixture())
		untrustedProposal.ProposerSigData = []byte{}

		res, err := cluster.NewProposal(untrustedProposal)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "proposer signature must not be empty")
	})
}

// TestNewRootProposal verifies the behavior of the NewRootProposal constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input with nil ProposerSigData:
//   - Verifies that a root proposal with nil ProposerSigData is accepted.
//
// 2. Valid input with empty ProposerSigData:
//   - Verifies that an empty (but non-nil) ProposerSigData is also accepted,
//     since root proposals must not include a signature.
//
// 3. Invalid input with invalid Block:
//   - Ensures an error is returned if the Block.ParentView is non-zero, which is disallowed for root blocks.
//
// 4. Invalid input with non-empty ProposerSigData:
//   - Ensures an error is returned when a ProposerSigData is included, as this is not permitted for root proposals.
func TestNewRootProposal(t *testing.T) {
	// validRootProposalFixture returns a new valid cluster.UntrustedProposal for use in tests.
	validRootProposalFixture := func() cluster.UntrustedProposal {
		block, err := cluster.NewRootBlock(cluster.UntrustedBlock{
			HeaderBody: flow.HeaderBody{
				ChainID:            flow.Emulator,
				ParentID:           flow.ZeroID,
				Height:             10,
				Timestamp:          uint64(time.Now().UnixMilli()),
				View:               0,
				ParentView:         0,
				ParentVoterIndices: []byte{},
				ParentVoterSigData: []byte{},
				ProposerID:         flow.ZeroID,
				LastViewTC:         nil,
			},
			Payload: *cluster.NewEmptyPayload(flow.ZeroID),
		})
		if err != nil {
			panic(err)
		}
		return cluster.UntrustedProposal{
			Block:           *block,
			ProposerSigData: nil,
		}
	}

	t.Run("valid input with nil ProposerSigData", func(t *testing.T) {
		res, err := cluster.NewRootProposal(validRootProposalFixture())

		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("valid input with empty ProposerSigData", func(t *testing.T) {
		untrustedProposal := validRootProposalFixture()
		untrustedProposal.ProposerSigData = []byte{}

		res, err := cluster.NewRootProposal(untrustedProposal)
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with invalid block", func(t *testing.T) {
		untrustedProposal := validRootProposalFixture()
		untrustedProposal.Block.ParentView = 1

		res, err := cluster.NewRootProposal(untrustedProposal)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid root block")
	})

	t.Run("invalid input with non-empty proposer signature", func(t *testing.T) {
		untrustedProposal := validRootProposalFixture()
		untrustedProposal.ProposerSigData = unittest.SignatureFixture()

		res, err := cluster.NewRootProposal(untrustedProposal)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "proposer signature must be empty")
	})
}

// TestBlockEncodingJSON_IDField ensures that the explicit ID field added to the
// block when encoded as JSON is present and accurate.
func TestBlockEncodingJSON_IDField(t *testing.T) {
	block := unittest.ClusterBlockFixture()
	blockID := block.ID()
	data, err := json.Marshal(block)
	require.NoError(t, err)
	var decodedIDField struct{ ID flow.Identifier }
	err = json.Unmarshal(data, &decodedIDField)
	require.NoError(t, err)
	assert.Equal(t, blockID, decodedIDField.ID)
}
