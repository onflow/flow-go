package unittest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// AddVersionBeacon adds blocks sequence with given VersionBeacon so this
// service events takes effect in Flow protocol.
// This means execution result where event was emitted is sealed, and the seal is
// finalized by a valid block.
// This assumes state is bootstrapped with a root block, as it does NOT produce
// results for final block of the state
// Root <- A <- B(result(A(VB))) <- C(seal(B))
func AddVersionBeacon(t *testing.T, beacon *flow.VersionBeacon, state protocol.FollowerState) {

	final, err := state.Final().Head()
	require.NoError(t, err)

	protocolState, err := state.Final().EpochProtocolState()
	require.NoError(t, err)
	protocolStateID := protocolState.Entry().ID()

	A := BlockWithParentFixture(final)
	A.SetPayload(PayloadFixture(WithProtocolStateID(protocolStateID)))
	addToState(t, state, A, true)

	receiptA := ReceiptForBlockFixture(A)
	receiptA.ExecutionResult.ServiceEvents = []flow.ServiceEvent{beacon.ServiceEvent()}

	B := BlockWithParentFixture(A.Header)
	B.SetPayload(flow.Payload{
		Receipts:        []*flow.ExecutionReceiptMeta{receiptA.Meta()},
		Results:         []*flow.ExecutionResult{&receiptA.ExecutionResult},
		ProtocolStateID: protocolStateID,
	})
	addToState(t, state, B, true)

	sealsForB := []*flow.Seal{
		Seal.Fixture(Seal.WithResult(&receiptA.ExecutionResult)),
	}

	C := BlockWithParentFixture(B.Header)
	C.SetPayload(flow.Payload{
		Seals:           sealsForB,
		ProtocolStateID: protocolStateID,
	})
	addToState(t, state, C, true)
}

func addToState(t *testing.T, state protocol.FollowerState, block *flow.Block, finalize bool) {

	err := state.ExtendCertified(context.Background(), block, CertifyBlock(block.Header))
	require.NoError(t, err)

	if finalize {
		err = state.Finalize(context.Background(), block.ID())
		require.NoError(t, err)
	}
}
