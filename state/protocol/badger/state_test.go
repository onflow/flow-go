package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/util"
	storagebadger "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestBootstrapAndOpen verifies after bootstraping with a state root,
// we should be able to open it and got the same state root
func TestBootstrapAndOpen(t *testing.T) {

	// create a state root and bootstrap the protocol state with it
	participants := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(participants, func(block *flow.Block) {
		block.Header.ParentID = unittest.IdentifierFixture()
	})

	util.RunWithBootstrapState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.State) {

		// protocol state has been bootstrapped, now open a protocol state with
		// the database
		metrics := &metrics.NoopCollector{}
		all := storagebadger.InitAll(metrics, db)
		state, err := bprotocol.OpenState(
			metrics,
			db,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.Setups,
			all.EpochCommits,
			all.Statuses)
		require.NoError(t, err)

		expected, err := rootSnapshot.Head()
		require.NoError(t, err)
		actual, err := state.Final().Head()
		require.NoError(t, err)

		assert.Equal(t, expected, actual)
	})

}
