package protocol_test

import (
	"context"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	protocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/util"
	bstorage "github.com/onflow/flow-go/storage/badger"
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
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.ParticipantState) {
		b1 := unittest.BlockWithParentFixture(b0)
		b1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
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
		headers := bstorage.NewHeaders(metrics, db)
		sigs := bstorage.NewProposalSignatures(metrics, db)
		finalized, pending, err := recovery.FindLatest(state, headers, sigs)
		require.NoError(t, err)
		require.Equal(t, b0.ID(), finalized.ID(), "recover find latest returns inconsistent finalized block")

		// b1,b2,b3 are unfinalized (pending) blocks
		require.Equal(t, []*flow.Proposal{b1p.HeaderProposal(), b2p.HeaderProposal(), b3p.HeaderProposal()}, pending)
	})
}
