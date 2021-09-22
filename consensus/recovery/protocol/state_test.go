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
	b0, err := rootSnapshot.Head()
	require.NoError(t, err)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *protocol.MutableState) {
		b1 := unittest.BlockWithParentFixture(b0)
		b1.SetPayload(flow.Payload{})

		err = state.Extend(context.Background(), &b1)
		require.NoError(t, err)

		b2 := unittest.BlockWithParentFixture(b1.Header)
		b2.SetPayload(flow.Payload{})

		err = state.Extend(context.Background(), &b2)
		require.NoError(t, err)

		b3 := unittest.BlockWithParentFixture(b2.Header)
		b3.SetPayload(flow.Payload{})

		err = state.Extend(context.Background(), &b3)
		require.NoError(t, err)

		metrics := metrics.NewNoopCollector()
		headers := bstorage.NewHeaders(metrics, db)
		finalized, pending, err := recovery.FindLatest(state, headers)
		require.NoError(t, err)
		require.Equal(t, b0.ID(), finalized.ID(), "recover find latest returns inconsistent finalized block")

		// b1,b2,b3 are unfinalized (pending) blocks
		require.Equal(t, []*flow.Header{b1.Header, b2.Header, b3.Header}, pending)
	})
}
