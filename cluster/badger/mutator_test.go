package badger

import (
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestBootstrap(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		chainID := cluster.Genesis().ChainID

		state, err := NewState(db, chainID)
		require.Nil(t, err)
		mutator := state.Mutate()

		// a helper function to wipe the DB to clean up in between tests
		cleanup := func() {
			err := db.DropAll()
			require.Nil(t, err)
		}

		t.Run("invalid chain ID", func(t *testing.T) {
			defer cleanup()
			genesis := cluster.Genesis()
			genesis.ChainID = fmt.Sprintf("%s-invalid", genesis.ChainID)

			err := mutator.Bootstrap(genesis)
			assert.Error(t, err)
		})

		t.Run("invalid number", func(t *testing.T) {
			defer cleanup()
			genesis := cluster.Genesis()
			genesis.Number = 1

			err := mutator.Bootstrap(genesis)
			assert.Error(t, err)
		})

		t.Run("invalid parent hash", func(t *testing.T) {
			defer cleanup()
			genesis := cluster.Genesis()
			genesis.ParentID = unittest.IdentifierFixture()

			err := mutator.Bootstrap(genesis)
			assert.Error(t, err)
		})

		t.Run("payload hash does not match payload", func(t *testing.T) {
			defer cleanup()
			genesis := cluster.Genesis()
			genesis.PayloadHash = unittest.IdentifierFixture()

			err := mutator.Bootstrap(genesis)
			assert.Error(t, err)
		})

		t.Run("invalid payload", func(t *testing.T) {
			defer cleanup()
			genesis := cluster.Genesis()
			genesis.Payload = cluster.Payload{
				Collection: flow.LightCollection{
					Transactions: []flow.Identifier{unittest.IdentifierFixture()},
				},
			}

			err := mutator.Bootstrap(genesis)
			assert.Error(t, err)
		})

		t.Run("bootstrap", func(t *testing.T) {
			defer cleanup()
			genesis := cluster.Genesis()

			err := mutator.Bootstrap(genesis)
			assert.Nil(t, err)

			err = db.View(func(tx *badger.Txn) error {

				// should insert collection
				var collection flow.LightCollection
				err = operation.RetrieveCollection(genesis.Collection.ID(), &collection)(tx)
				assert.Nil(t, err)
				assert.Equal(t, genesis.Collection, collection)

				// should index collection
				var collectionID flow.Identifier
				err = operation.LookupCollection(genesis.PayloadHash, &collectionID)(tx)
				assert.Nil(t, err)
				assert.Equal(t, genesis.Collection.ID(), collectionID)

				// should insert header
				var header flow.Header
				err = operation.RetrieveHeader(genesis.ID(), &header)(tx)
				assert.Nil(t, err)
				assert.Equal(t, genesis.Header.ID(), header.ID())

				// should insert block number -> ID lookup
				var blockID flow.Identifier
				err = operation.RetrieveNumberForCluster(genesis.ChainID, genesis.Number, &blockID)(tx)
				assert.Nil(t, err)
				assert.Equal(t, genesis.ID(), blockID)

				// should insert boundary
				var boundary uint64
				err = operation.RetrieveBoundaryForCluster(genesis.ChainID, &boundary)(tx)
				assert.Nil(t, err)
				assert.Equal(t, genesis.Number, boundary)

				return nil
			})
			assert.Nil(t, err)
		})
	})
}

func TestExtend(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		genesis := cluster.Genesis()
		chainID := genesis.ChainID

		// a helper function to wipe the DB to clean up in between tests
		cleanup := func() {
			err := db.DropAll()
			require.Nil(t, err)
		}

		// set up state and mutator objects, these are safe to share between tests
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
			// first insert the payload
			err = db.Update(operation.AllowDuplicates(operation.InsertCollection(&block.Collection)))
			assert.Nil(t, err)
			// then insert the block
			err = db.Update(procedure.InsertClusterBlock(&block))
			assert.Nil(t, err)
		}

		t.Run("without first bootstrapping", func(t *testing.T) {
			defer cleanup()

			block := unittest.ClusterBlockWithParent(genesis)
			insert(block)

			err = mutator.Extend(block.ID())
			assert.Error(t, err)
		})

		t.Run("non-existent block", func(t *testing.T) {
			defer cleanup()
			bootstrap()

			// ID of a non-existent block
			blockID := unittest.IdentifierFixture()

			err = mutator.Extend(blockID)
			assert.Error(t, err)
		})

		t.Run("non-existent parent", func(t *testing.T) {
			defer cleanup()
			bootstrap()

			block := unittest.ClusterBlockWithParent(genesis)
			// change the parent ID
			block.ParentID = unittest.IdentifierFixture()
			insert(block)

			err = mutator.Extend(block.ID())
			assert.Error(t, err)
		})

		t.Run("wrong chain ID", func(t *testing.T) {
			defer cleanup()
			bootstrap()

			block := unittest.ClusterBlockWithParent(genesis)
			// change the chain ID
			block.ChainID = fmt.Sprintf("%s-invalid", block.ChainID)
			insert(block)

			err = mutator.Extend(block.ID())
			assert.Error(t, err)
		})

		t.Run("invalid block number", func(t *testing.T) {
			defer cleanup()
			bootstrap()

			block := unittest.ClusterBlockWithParent(genesis)
			// change the block number
			block.Number = block.Number - 1
			insert(block)

			err = mutator.Extend(block.ID())
			assert.Error(t, err)
		})

		t.Run("building on parent of finalized block", func(t *testing.T) {
			defer cleanup()
			bootstrap()

			// build one block on top of genesis
			block1 := unittest.ClusterBlockWithParent(genesis)
			insert(block1)
			err = mutator.Extend(block1.ID())
			assert.Nil(t, err)

			// finalize the block
			err = db.Update(procedure.FinalizeClusterBlock(block1.ID()))
			assert.Nil(t, err)

			// insert another block on top of genesis
			// since we have already finalized block 1, this is invalid
			block2 := unittest.ClusterBlockWithParent(genesis)
			insert(block2)

			// try to extend with the invalid block
			err = mutator.Extend(block2.ID())
			assert.Error(t, err)
		})

		t.Run("extend", func(t *testing.T) {
			defer cleanup()
			bootstrap()

			block := unittest.ClusterBlockWithParent(genesis)
			insert(block)

			err = mutator.Extend(block.ID())
			assert.Nil(t, err)
		})

		t.Run("extend with empty collection", func(t *testing.T) {
			defer cleanup()
			bootstrap()

			block := unittest.ClusterBlockWithParent(genesis)
			// set an empty collection as the payload
			block.Collection = flow.LightCollection{}
			block.PayloadHash = block.Payload.Hash()
			insert(block)

			err = mutator.Extend(block.ID())
			assert.Nil(t, err)
		})
	})
}
