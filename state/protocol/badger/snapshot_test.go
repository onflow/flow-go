// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	"github.com/dapperlabs/flow-go/state/protocol/util"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestHead(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		// setup
		header := unittest.BlockHeaderFixture()
		header.Height = 42

		err := db.Update(operation.InsertHeader(header.ID(), &header))
		require.NoError(t, err)

		err = db.Update(operation.IndexBlockHeight(header.Height, header.ID()))
		require.NoError(t, err)

		// add a second, outdated boundary to ensure the latest is taken
		err = db.Update(operation.InsertFinalizedHeight(header.Height - 1))
		require.NoError(t, err)

		err = db.Update(operation.UpdateFinalizedHeight(header.Height))
		require.NoError(t, err)

		t.Run("works with block number", func(t *testing.T) {
			retrieved, err := state.AtHeight(header.Height).Head()
			require.NoError(t, err)
			require.Equal(t, header.ID(), retrieved.ID())
		})

		t.Run("works with block id", func(t *testing.T) {
			retrieved, err := state.AtBlockID(header.ID()).Head()
			require.NoError(t, err)
			require.Equal(t, header.ID(), retrieved.ID())
		})

		t.Run("works with finalized block", func(t *testing.T) {
			retrieved, err := state.Final().Head()
			require.NoError(t, err)
			require.Equal(t, header.ID(), retrieved.ID())
		})
	})
}

func TestIdentity(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		identity := unittest.IdentityFixture()
		blockID := unittest.IdentifierFixture()

		err := db.Update(operation.InsertFinalizedHeight(0))
		require.NoError(t, err)

		err = db.Update(operation.IndexBlockHeight(0, blockID))
		require.NoError(t, err)

		err = db.Update(operation.InsertIdentity(identity.ID(), identity))
		require.NoError(t, err)

		err = db.Update(operation.IndexPayloadIdentities(blockID, []flow.Identifier{identity.NodeID}))

		actual, err := state.Final().Identity(identity.NodeID)
		require.NoError(t, err)
		assert.EqualValues(t, identity, actual)

		_, err = state.Final().Identity(unittest.IdentifierFixture())
		require.Error(t, err)
	})
}

func TestIdentities(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		blockID := unittest.IdentifierFixture()
		identities := unittest.IdentityListFixture(8)

		err := db.Update(operation.InsertFinalizedHeight(0))
		require.NoError(t, err)

		err = db.Update(operation.IndexBlockHeight(0, blockID))
		require.NoError(t, err)

		for _, identity := range identities {
			err = db.Update(operation.InsertIdentity(identity.ID(), identity))
			require.NoError(t, err)
		}

		err = db.Update(operation.IndexPayloadIdentities(blockID, flow.GetIDs(identities)))
		require.NoError(t, err)

		actual, err := state.Final().Identities(filter.Any)
		require.NoError(t, err)
		assert.ElementsMatch(t, identities, actual)
	})
}

func TestClusters(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		blockID := unittest.IdentifierFixture()
		identities := unittest.IdentityListFixture(7, unittest.WithRole(flow.RoleCollection))

		err := db.Update(operation.InsertFinalizedHeight(0))
		require.NoError(t, err)

		err = db.Update(operation.IndexBlockHeight(0, blockID))
		require.NoError(t, err)

		for _, identity := range identities {
			err = db.Update(operation.InsertIdentity(identity.ID(), identity))
			require.NoError(t, err)
		}

		err = db.Update(operation.IndexPayloadIdentities(blockID, flow.GetIDs(identities)))

		actual, err := state.Final().Clusters()
		require.NoError(t, err)

		require.Equal(t, 3, actual.Size())
		assert.Len(t, actual.ByIndex(0), 3)
		assert.Len(t, actual.ByIndex(1), 2)
		assert.Len(t, actual.ByIndex(2), 2)
	}, protocol.SetClusters(3))
}
