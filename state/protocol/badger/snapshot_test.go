// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
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
		block.Height = 42

		err := db.Update(procedure.InsertBlock(&block))
		require.NoError(t, err)
		err = db.Update(operation.InsertNumber(block.Height, block.ID()))
		require.NoError(t, err)

		// add a second, outdated boundary to ensure the latest is taken
		err = db.Update(operation.InsertBoundary(block.Height - 1))
		require.NoError(t, err)

		err = db.Update(operation.UpdateBoundary(block.Height))
		require.NoError(t, err)

		state := State{db: db}

		t.Run("works with block number", func(t *testing.T) {
			header, err := state.AtNumber(block.Height).Head()
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
		// setup

		ids := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Role: flow.Role(1), Address: "a1"},
			{NodeID: flow.Identifier{0x02}, Role: flow.Role(2), Address: "a2"},
			{NodeID: flow.Identifier{0x03}, Role: flow.Role(3), Address: "a3"},
		}

		err := db.Update(operation.InsertIdentities(ids))
		require.NoError(t, err)

		genesis := unittest.BlockFixture()
		genesis.Height = 0
		err = db.Update(procedure.InsertBlock(&genesis))
		require.NoError(t, err)
		err = db.Update(operation.InsertNumber(0, genesis.ID()))
		require.NoError(t, err)

		state := State{db: db}

		t.Run("fails for non staked identity", func(t *testing.T) {
			nodeID := unittest.IdentifierFixture()
			_, err := state.AtNumber(0).Identity(nodeID)
			require.EqualError(t, err, fmt.Sprintf("identity not staked (%x)", nodeID))
		})

		t.Run("works for staked identity", func(t *testing.T) {
			actual, err := state.AtNumber(0).Identity(ids[1].NodeID)
			require.NoError(t, err)
			require.EqualValues(t, ids[1], actual)
		})
	})
}

func TestIdentities(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		// setup

		ids := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Role: flow.Role(1), Address: "a1", Stake: 1000},
			{NodeID: flow.Identifier{0x02}, Role: flow.Role(2), Address: "a2", Stake: 0500},
			{NodeID: flow.Identifier{0x03}, Role: flow.Role(3), Address: "a3", Stake: 0000},
		}

		err := db.Update(operation.InsertIdentities(ids))
		require.NoError(t, err)

		genesis := unittest.BlockFixture()
		genesis.Height = 0
		err = db.Update(procedure.InsertBlock(&genesis))
		require.NoError(t, err)
		err = db.Update(operation.InsertNumber(0, genesis.ID()))
		require.NoError(t, err)

		err = db.Update(operation.InsertBoundary(1))
		require.NoError(t, err)

		state := State{db: db}

		t.Run("works", func(t *testing.T) {
			actual, err := state.AtNumber(3).Identity(ids[0].NodeID)
			require.NoError(t, err)
			require.EqualValues(t, ids[0], actual)
		})
	})
}

func TestClusters(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		// setup

		ids := flow.IdentityList{
			{NodeID: flow.Identifier{0x03}, Role: flow.Role(1), Address: "a1", Stake: 200},
			{NodeID: flow.Identifier{0x02}, Role: flow.Role(3), Address: "a3", Stake: 100},
			{NodeID: flow.Identifier{0x01}, Role: flow.Role(1), Address: "a2"},
		}

		err := db.Update(operation.InsertIdentities(ids))
		require.NoError(t, err)

		genesis := unittest.BlockFixture()
		genesis.Height = 0
		err = db.Update(procedure.InsertBlock(&genesis))
		require.NoError(t, err)
		err = db.Update(operation.InsertNumber(0, genesis.ID()))
		require.NoError(t, err)

		err = db.Update(operation.InsertBoundary(1))
		require.NoError(t, err)

		state := State{db: db, clusters: 2}

		actual, err := state.AtNumber(0).Clusters()
		require.NoError(t, err)

		require.Equal(t, 2, actual.Size())
		require.Equal(t, flow.IdentityList{ids[2]}, actual.ByIndex(0))
		require.Equal(t, flow.IdentityList{ids[0]}, actual.ByIndex(1))
	})
}
