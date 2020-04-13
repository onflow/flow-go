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

		for _, id := range ids {
			err := db.Update(operation.InsertIdentity(id))
			require.NoError(t, err)
		}

		vectors := []struct {
			Number uint64
			Deltas map[int]int64
		}{
			{
				Number: 0,
				Deltas: map[int]int64{
					0: 100,
					1: 500,
				},
			},
			{
				Number: 1,
				Deltas: map[int]int64{
					0: -100,
					2: 200,
				},
			},
			{
				Number: 4,
				Deltas: map[int]int64{
					2: -100,
				},
			},
		}

		for _, v := range vectors {
			for i, delta := range v.Deltas {
				id := ids[i]
				err := db.Update(operation.InsertDelta(v.Number, id.Role, id.NodeID, delta))
				require.NoError(t, err)
			}
		}

		genesis := unittest.BlockFixture()
		genesis.Height = 0
		err := db.Update(procedure.InsertBlock(&genesis))
		require.NoError(t, err)
		err = db.Update(operation.InsertNumber(0, genesis.ID()))
		require.NoError(t, err)

		final := unittest.BlockFixture()
		final.Height = 1
		final.ParentID = genesis.ID()
		err = db.Update(procedure.InsertBlock(&final))
		require.NoError(t, err)
		err = db.Update(operation.InsertNumber(1, final.ID()))
		require.NoError(t, err)
		err = db.Update(operation.InsertBoundary(1))
		require.NoError(t, err)

		block2 := unittest.BlockFixture()
		block2.Height = 2
		block2.ParentID = final.ID()
		block2.Identities = nil
		block2.PayloadHash = block2.Payload.Hash()
		err = db.Update(procedure.InsertBlock(&block2))
		require.NoError(t, err)
		err = db.Update(operation.InsertNumber(2, block2.ID()))
		require.NoError(t, err)

		block3 := unittest.BlockFixture()
		block3.Height = 3
		block3.ParentID = block2.ID()
		err = db.Update(procedure.InsertBlock(&block3))
		require.NoError(t, err)
		err = db.Update(operation.InsertNumber(3, block3.ID()))
		require.NoError(t, err)

		state := State{db: db}

		t.Run("works before finalized state", func(t *testing.T) {
			actual, err := state.AtNumber(0).Identities(filter.Any)
			require.NoError(t, err)
			expected := flow.IdentityList{ids[0], ids[1]}
			ids[0].Stake = 100
			ids[1].Stake = 500
			require.EqualValues(t, expected, actual)
		})

		t.Run("works after finalized state", func(t *testing.T) {
			actual, err := state.AtNumber(3).Identities(filter.Any)
			require.NoError(t, err)
			expected := flow.IdentityList{ids[0], ids[1], ids[2]}
			ids[0].Stake = 0 // hot stuff keeps identities with 0 stake in identities list
			ids[1].Stake = 500
			ids[2].Stake = 200
			require.EqualValues(t, expected, actual)
		})
	})
}

func TestIdentities(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		// setup

		ids := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Role: flow.Role(1), Address: "a1"},
			{NodeID: flow.Identifier{0x02}, Role: flow.Role(2), Address: "a2"},
			{NodeID: flow.Identifier{0x03}, Role: flow.Role(3), Address: "a3"},
		}

		for _, id := range ids {
			err := db.Update(operation.InsertIdentity(id))
			require.NoError(t, err)
		}

		vectors := []struct {
			Number uint64
			Deltas map[int]int64
		}{
			{
				Number: 0,
				Deltas: map[int]int64{
					0: 100,
					1: 500,
				},
			},
		}

		for _, v := range vectors {
			for i, delta := range v.Deltas {
				id := ids[i]
				err := db.Update(operation.InsertDelta(v.Number, id.Role, id.NodeID, delta))
				require.NoError(t, err)
			}
		}

		genesis := unittest.BlockFixture()
		genesis.Height = 0
		err := db.Update(procedure.InsertBlock(&genesis))
		require.NoError(t, err)
		err = db.Update(operation.InsertNumber(0, genesis.ID()))
		require.NoError(t, err)

		err = db.Update(operation.InsertBoundary(1))
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
			ids[1].Stake = 500
			require.EqualValues(t, ids[1], actual)
		})
	})
}

func TestClusters(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		// setup

		ids := flow.IdentityList{
			{NodeID: flow.Identifier{0x03}, Role: flow.Role(1), Address: "a1"},
			{NodeID: flow.Identifier{0x02}, Role: flow.Role(3), Address: "a3"},
			{NodeID: flow.Identifier{0x01}, Role: flow.Role(1), Address: "a2"},
		}

		for _, id := range ids {
			err := db.Update(operation.InsertIdentity(id))
			require.NoError(t, err)
		}

		vectors := []struct {
			Number uint64
			Deltas map[int]int64
		}{
			{
				Number: 0,
				Deltas: map[int]int64{
					0: 100,
					1: 500,
					2: 200,
				},
			},
		}

		for _, v := range vectors {
			for i, delta := range v.Deltas {
				id := ids[i]
				err := db.Update(operation.InsertDelta(v.Number, id.Role, id.NodeID, delta))
				require.NoError(t, err)
			}
		}

		genesis := unittest.BlockFixture()
		genesis.Height = 0
		err := db.Update(procedure.InsertBlock(&genesis))
		require.NoError(t, err)
		err = db.Update(operation.InsertNumber(0, genesis.ID()))
		require.NoError(t, err)

		err = db.Update(operation.InsertBoundary(1))
		require.NoError(t, err)

		state := State{db: db, clusters: 2}

		actual, err := state.AtNumber(0).Clusters()
		require.NoError(t, err)

		ids[0].Stake = 100
		ids[2].Stake = 200

		require.Equal(t, 2, actual.Size())
		require.Equal(t, flow.IdentityList{ids[2]}, actual.ByIndex(0))
		require.Equal(t, flow.IdentityList{ids[0]}, actual.ByIndex(1))
	})
}
