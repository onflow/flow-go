package protocol_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	recovery "github.com/dapperlabs/flow-go/consensus/recovery/protocol"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	"github.com/dapperlabs/flow-go/state/protocol/util"
	bstorage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// as a consensus follower, when a block is received and saved,
// if it's not finalized yet, this block should be returned by latest
func TestSaveBlockAsReplica(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
		b0, result, seal := unittest.BootstrapFixture(participants)

		err := state.Mutate().Bootstrap(b0, result, seal)
		require.NoError(t, err)

		b1 := unittest.BlockWithParentFixture(b0.Header)
		b1.SetPayload(flow.Payload{})

		err = state.Mutate().Extend(&b1)
		require.NoError(t, err)

		b2 := unittest.BlockWithParentFixture(b1.Header)
		b2.SetPayload(flow.Payload{})

		err = state.Mutate().Extend(&b2)
		require.NoError(t, err)

		b3 := unittest.BlockWithParentFixture(b2.Header)
		b3.SetPayload(flow.Payload{})

		err = state.Mutate().Extend(&b3)
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
