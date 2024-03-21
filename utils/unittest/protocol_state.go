package unittest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
)

// FinalizedProtocolStateWithParticipants returns a protocol state with finalized participants
func FinalizedProtocolStateWithParticipants(participants flow.IdentityList) (
	*flow.Block, *mockprotocol.Snapshot, *mockprotocol.State, *mockprotocol.Snapshot) {
	sealed := BlockFixture()
	block := BlockWithParentFixture(sealed.Header)
	head := block.Header

	// set up protocol snapshot mock
	snapshot := &mockprotocol.Snapshot{}
	snapshot.On("Identities", mock.Anything).Return(
		func(filter flow.IdentityFilter[flow.Identity]) flow.IdentityList {
			return participants.Filter(filter)
		},
		nil,
	)
	snapshot.On("Identity", mock.Anything).Return(func(id flow.Identifier) *flow.Identity {
		for _, n := range participants {
			if n.ID() == id {
				return n
			}
		}
		return nil
	}, nil)
	snapshot.On("Head").Return(
		func() *flow.Header {
			return head
		},
		nil,
	)

	sealedSnapshot := &mockprotocol.Snapshot{}
	sealedSnapshot.On("Head").Return(
		func() *flow.Header {
			return sealed.Header
		},
		nil,
	)

	// set up protocol state mock
	state := &mockprotocol.State{}
	state.On("Final").Return(
		func() protocol.Snapshot {
			return snapshot
		},
	)
	state.On("Sealed").Return(func() protocol.Snapshot {
		return sealedSnapshot
	},
	)
	state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) protocol.Snapshot {
			return snapshot
		},
	)
	return block, snapshot, state, sealedSnapshot
}

// SealBlock seals a block B by building two blocks on it, the first containing
// a receipt for the block (BR), the second (BS) containing a seal for the block.
// B <- BR(Result_B) <- BS(Seal_B)
// Returns the two generated blocks.
func SealBlock(t *testing.T, st protocol.ParticipantState, mutableProtocolState protocol.MutableProtocolState, block *flow.Block, receipt *flow.ExecutionReceipt, seal *flow.Seal) (br *flow.Block, bs *flow.Block) {

	block2 := BlockWithParentFixture(block.Header)
	block2.SetPayload(flow.Payload{
		Receipts:        []*flow.ExecutionReceiptMeta{receipt.Meta()},
		Results:         []*flow.ExecutionResult{&receipt.ExecutionResult},
		ProtocolStateID: block.Payload.ProtocolStateID,
	})
	err := st.Extend(context.Background(), block2)
	require.NoError(t, err)

	block3 := BlockWithParentFixture(block2.Header)
	stateMutator, err := mutableProtocolState.Mutator(block3.Header)
	require.NoError(t, err)
	seals := []*flow.Seal{seal}
	err = stateMutator.ApplyServiceEventsFromValidatedSeals(seals)
	require.NoError(t, err)
	_, _, updatedStateId, _ := stateMutator.Build()
	require.NoError(t, err)

	block3.SetPayload(flow.Payload{
		Seals:           seals,
		ProtocolStateID: updatedStateId,
	})
	err = st.Extend(context.Background(), block3)
	require.NoError(t, err)

	return block2, block3
}

// InsertAndFinalize inserts, then finalizes, the input block.
func InsertAndFinalize(t *testing.T, st protocol.ParticipantState, block *flow.Block) {
	err := st.Extend(context.Background(), block)
	require.NoError(t, err)
	err = st.Finalize(context.Background(), block.ID())
	require.NoError(t, err)
}
