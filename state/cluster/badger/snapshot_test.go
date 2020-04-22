package badger

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestSnapshot(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		genesis := cluster.Genesis()
		chainID := genesis.ChainID

		// a helper function to wipe the DB to clean up in between tests
		cleanup := func() {
			err := db.DropAll()
			require.Nil(t, err)
		}

		state, err := NewState(db, chainID)
		require.Nil(t, err)
		mutator := state.Mutate()

		// a helper function to bootstrap with the genesis block
		bootstrap := func() {
			err = mutator.Bootstrap(genesis)
			assert.Nil(t, err)
		}

		// a helper function to insert a block
		insert := func(block cluster.Block) {
			err = db.Update(procedure.InsertClusterBlock(&block))
			assert.Nil(t, err)
		}

		t.Run("nonexistent block", func(t *testing.T) {
			defer cleanup()
			bootstrap()

			nonexistentBlockID := unittest.IdentifierFixture()
			snapshot := state.AtBlockID(nonexistentBlockID)

			_, err = snapshot.Collection()
			assert.Error(t, err)

			_, err = snapshot.Head()
			assert.Error(t, err)
		})

		t.Run("at block ID", func(t *testing.T) {
			defer cleanup()
			bootstrap()

			snapshot := state.AtBlockID(genesis.ID())

			// ensure collection is correct
			coll, err := snapshot.Collection()
			assert.Nil(t, err)
			assert.Equal(t, &genesis.Collection, coll)

			// ensure head is correct
			head, err := snapshot.Head()
			assert.Nil(t, err)
			assert.Equal(t, genesis.ID(), head.ID())
		})

		t.Run("with empty collection", func(t *testing.T) {
			defer cleanup()
			bootstrap()

			// create a block with an empty collection
			block := unittest.ClusterBlockWithParent(genesis)
			block.SetPayload(cluster.EmptyPayload())
			insert(block)

			snapshot := state.AtBlockID(block.ID())

			// ensure collection is correct
			coll, err := snapshot.Collection()
			assert.Nil(t, err)
			assert.Equal(t, &block.Collection, coll)
		})

		t.Run("finalized block", func(t *testing.T) {
			defer cleanup()
			bootstrap()

			// create a new finalized block on genesis (height=1)
			finalizedBlock1 := unittest.ClusterBlockWithParent(genesis)
			insert(finalizedBlock1)
			err = mutator.Extend(finalizedBlock1.ID())
			assert.Nil(t, err)

			// create an un-finalized block on genesis (height=1)
			unFinalizedBlock1 := unittest.ClusterBlockWithParent(genesis)
			insert(unFinalizedBlock1)
			err = mutator.Extend(unFinalizedBlock1.ID())
			assert.Nil(t, err)

			// create a second un-finalized on top of the finalized block (height=2)
			unFinalizedBlock2 := unittest.ClusterBlockWithParent(&finalizedBlock1)
			insert(unFinalizedBlock2)
			err = mutator.Extend(unFinalizedBlock2.ID())
			assert.Nil(t, err)

			// finalize the block
			err = db.Update(procedure.FinalizeClusterBlock(finalizedBlock1.ID()))
			assert.Nil(t, err)

			// get the final snapshot, should map to finalizedBlock1
			snapshot := state.Final()

			// ensure collection is correct
			coll, err := snapshot.Collection()
			assert.Nil(t, err)
			assert.Equal(t, &finalizedBlock1.Collection, coll)

			// ensure head is correct
			head, err := snapshot.Head()
			assert.Nil(t, err)
			assert.Equal(t, finalizedBlock1.ID(), head.ID())
		})
	})
}
