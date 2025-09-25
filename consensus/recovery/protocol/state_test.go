package protocol_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	protocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

// as a consensus follower, when a block is received and saved,
// if it's not finalized yet, this block should be returned by latest
func TestSaveBlockAsReplica(t *testing.T) {
	participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	protocolState, err := rootSnapshot.ProtocolState()
	require.NoError(t, err)
	rootProtocolStateID := protocolState.ID()
	b0, err := rootSnapshot.Head()
	require.NoError(t, err)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db storage.DB, state *protocol.ParticipantState) {
		b1 := unittest.BlockWithParentAndPayload(
			b0,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		b1p := unittest.ProposalFromBlock(b1)
		err = state.Extend(context.Background(), b1p)
		require.NoError(t, err)

		b2 := unittest.BlockWithParentProtocolState(b1)
		b2p := unittest.ProposalFromBlock(b2)
		err = state.Extend(context.Background(), b2p)
		require.NoError(t, err)

		b3 := unittest.BlockWithParentProtocolState(b2)
		b3p := unittest.ProposalFromBlock(b3)
		err = state.Extend(context.Background(), b3p)
		require.NoError(t, err)

		metrics := metrics.NewNoopCollector()
		headers := store.NewHeaders(metrics, db)
		finalized, pending, err := recovery.FindLatest(state, headers)
		require.NoError(t, err)
		require.Equal(t, b0.ID(), finalized.ID(), "recover find latest returns inconsistent finalized block")

		// b1,b2,b3 are unfinalized (pending) blocks
		require.Equal(t, []*flow.ProposalHeader{b1p.ProposalHeader(), b2p.ProposalHeader(), b3p.ProposalHeader()}, pending)
	})
}
