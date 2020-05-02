// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestHead(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		// setup
		block := unittest.BlockFixture()
		block.Header.Height = 42

		err := db.Update(procedure.InsertBlock(&block))
		require.NoError(t, err)
		err = db.Update(operation.InsertNumber(block.Header.Height, block.ID()))
		require.NoError(t, err)

		// add a second, outdated boundary to ensure the latest is taken
		err = db.Update(operation.InsertBoundary(block.Header.Height - 1))
		require.NoError(t, err)

		err = db.Update(operation.UpdateBoundary(block.Header.Height))
		require.NoError(t, err)

		state := State{db: db}

		t.Run("works with block number", func(t *testing.T) {
			header, err := state.AtNumber(block.Header.Height).Head()
			require.NoError(t, err)
			require.Equal(t, block.ID(), header.ID())
		})

		t.Run("works with block id", func(t *testing.T) {
			header, err := state.AtBlockID(block.ID()).Head()
			require.NoError(t, err)
			require.Equal(t, block.ID(), header.ID())
		})

		t.Run("works with finalized block", func(t *testing.T) {
			header, err := state.Final().Head()
			require.NoError(t, err)
			require.Equal(t, block.ID(), header.ID())
		})
	})
}

func TestIdentity(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		identity := unittest.IdentityFixture()

		err := db.Update(operation.InsertIdentities(flow.IdentityList{identity}))
		require.NoError(t, err)

		state := State{db: db}

		actual, err := state.Final().Identity(identity.NodeID)
		require.NoError(t, err)
		assert.EqualValues(t, identity, actual)

		_, err = state.Final().Identity(unittest.IdentifierFixture())
		require.Error(t, err)
	})
}

func TestIdentities(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		identities := unittest.IdentityListFixture(8)

		err := db.Update(operation.InsertIdentities(identities))
		require.NoError(t, err)

		state := State{db: db}

		actual, err := state.Final().Identities(filter.Any)
		require.NoError(t, err)
		assert.ElementsMatch(t, identities, actual)
	})
}

func TestClusters(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		identities := unittest.IdentityListFixture(7, unittest.WithRole(flow.RoleCollection))

		err := db.Update(operation.InsertIdentities(identities))
		require.NoError(t, err)

		state := State{db: db, clusters: 3}

		actual, err := state.Final().Clusters()
		require.NoError(t, err)

		require.Equal(t, 3, actual.Size())
		assert.Len(t, actual.ByIndex(0), 3)
		assert.Len(t, actual.ByIndex(1), 2)
		assert.Len(t, actual.ByIndex(2), 2)
	})
}
