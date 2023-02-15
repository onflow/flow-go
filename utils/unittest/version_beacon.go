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
// finalized by a valid block, meaning having a QC
// This assumes state is bootstrapped with a root block, as it does NOT produce
// results for final block of the state
// Root <- A <- B(result(A(VB))) <- C(seal(B)) <- D <- E(QC(D))
func AddVersionBeacon(t *testing.T, beacon *flow.VersionBeacon, state protocol.MutableState) {

	final, err := state.Final().Head()

	require.NoError(t, err)

	A := BlockWithParentFixture(final)
	A.SetPayload(flow.Payload{})
	addToState(t, state, A, true)

	receiptA := ReceiptForBlockFixture(A)
	receiptA.ExecutionResult.ServiceEvents = []flow.ServiceEvent{beacon.ServiceEvent()}

	B := BlockWithParentFixture(A.Header)
	B.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{receiptA.Meta()},
		Results:  []*flow.ExecutionResult{&receiptA.ExecutionResult},
	})
	addToState(t, state, B, true)

	sealsForB := []*flow.Seal{
		Seal.Fixture(Seal.WithResult(&receiptA.ExecutionResult)),
	}

	C := BlockWithParentFixture(B.Header)
	C.SetPayload(flow.Payload{
		Seals: sealsForB,
	})
	addToState(t, state, C, true)

	D := BlockWithParentFixture(C.Header)
	addToState(t, state, D, true)

	E := BlockWithParentFixture(D.Header)
	addToState(t, state, E, false)
}

func addToState(t *testing.T, state protocol.MutableState, block *flow.Block, finalize bool) {

	err := state.Extend(context.Background(), block)
	require.NoError(t, err)

	//err = state.MarkValid(block.ID())
	//require.NoError(t, err)

	if finalize {
		err = state.Finalize(context.Background(), block.ID())
		require.NoError(t, err)
	}
}
