package collection_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cluster "github.com/dapperlabs/flow-go/cluster/badger"
	model "github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/finalizer/collection"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	networkmock "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestFinalizer(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		genesis := model.Genesis()
		chainID := genesis.ChainID

		state, err := cluster.NewState(db, chainID)
		require.NoError(t, err)
		mutator := state.Mutate()

		pool, err := stdmap.NewTransactions()
		require.NoError(t, err)

		prov := new(networkmock.Engine)

		finalizer := collection.NewFinalizer(db, pool, prov, chainID)

		// a helper function to wipe the DB to clean up in between tests
		cleanup := func() {
			err := db.DropAll()
			require.Nil(t, err)
		}

		// a helper function to bootstrap with the genesis block
		bootstrap := func() {
			err = mutator.Bootstrap(genesis)
			assert.Nil(t, err)
		}

		// a helper function to insert a block
		insert := func(block model.Block) {
			// first insert the payload
			err = db.Update(operation.AllowDuplicates(operation.InsertCollection(&block.Collection)))
			assert.Nil(t, err)
			// then insert the block
			err = db.Update(procedure.InsertClusterBlock(&block))
			assert.Nil(t, err)
		}

		t.Run("non-existent block", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			fakeBlockID := unittest.IdentifierFixture()
			err := finalizer.MakeFinal(fakeBlockID)
			assert.Error(t, err)
		})

		t.Run("already finalized block", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			// tx1 is included in the finalized block
			tx1 := unittest.TransactionFixture(func(tx *flow.Transaction) { tx.Nonce = 1 })
			assert.Nil(t, pool.Add(&tx1))

			// create a new block on genesis
			block := unittest.ClusterBlockWithParent(genesis)
			block.SetPayload(model.PayloadFromTransactions([]flow.Identifier{tx1.ID()}))
			insert(block)

			// finalize the block
			err := finalizer.MakeFinal(block.ID())
			assert.Nil(t, err)

			// finalize the block again - this should fail
			err = finalizer.MakeFinal(block.ID())
			assert.Error(t, err)
		})

		t.Run("unconnected block", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			// create a new block that isn't connected to a parent
			block := unittest.ClusterBlockWithParent(genesis)
			block.ParentID = unittest.IdentifierFixture()
			insert(block)

			// try to finalize - this should fail
			err := finalizer.MakeFinal(block.ID())
			assert.Error(t, err)
		})

		t.Run("finalize single block", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			// tx1 is included in the finalized block and mempool
			tx1 := unittest.TransactionFixture(func(tx *flow.Transaction) { tx.Nonce = 1 })
			assert.Nil(t, pool.Add(&tx1))
			// tx2 is only in the mempool
			tx2 := unittest.TransactionFixture(func(tx *flow.Transaction) { tx.Nonce = 2 })
			assert.Nil(t, pool.Add(&tx2))

			// create a block containing tx1 on top of genesis
			block := unittest.ClusterBlockWithParent(genesis)
			block.SetPayload(model.PayloadFromTransactions([]flow.Identifier{tx1.ID()}))
			insert(block)

			// finalize the block
			err := finalizer.MakeFinal(block.ID())
			assert.Nil(t, err)

			// tx1 should have been removed from mempool
			assert.False(t, pool.Has(tx1.ID()))
			// tx2 should still be in mempool
			assert.True(t, pool.Has(tx2.ID()))

			// check finalized boundary using cluster state
			final, err := state.Final().Head()
			assert.Nil(t, err)
			assert.Equal(t, block.ID(), final.ID())
		})

		// when finalizing a block with un-finalized ancestors, those ancestors
		// should be finalized as well
		t.Run("finalize multiple blocks together", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			// tx1 is included in the first finalized block and mempool
			tx1 := unittest.TransactionFixture(func(tx *flow.Transaction) { tx.Nonce = 1 })
			assert.Nil(t, pool.Add(&tx1))
			// tx2 is included in the second finalized block and mempool
			tx2 := unittest.TransactionFixture(func(tx *flow.Transaction) { tx.Nonce = 2 })
			assert.Nil(t, pool.Add(&tx2))

			// create a block containing tx1 on top of genesis
			block1 := unittest.ClusterBlockWithParent(genesis)
			block1.SetPayload(model.PayloadFromTransactions([]flow.Identifier{tx1.ID()}))
			insert(block1)

			// create a block containing tx2 on top of block1
			block2 := unittest.ClusterBlockWithParent(&block1)
			block2.SetPayload(model.PayloadFromTransactions([]flow.Identifier{tx2.ID()}))
			insert(block2)

			// finalize block2 (should indirectly finalize block1 as well)
			err := finalizer.MakeFinal(block2.ID())
			assert.Nil(t, err)

			// tx1 and tx2 should have been removed from mempool
			assert.False(t, pool.Has(tx1.ID()))
			assert.False(t, pool.Has(tx2.ID()))

			// check finalized boundary using cluster state
			final, err := state.Final().Head()
			assert.Nil(t, err)
			assert.Equal(t, block2.ID(), final.ID())
		})

		t.Run("finalize with un-finalized child", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			// tx1 is included in the finalized parent block and mempool
			tx1 := unittest.TransactionFixture(func(tx *flow.Transaction) { tx.Nonce = 1 })
			assert.Nil(t, pool.Add(&tx1))
			// tx2 is included in the un-finalized block and mempool
			tx2 := unittest.TransactionFixture(func(tx *flow.Transaction) { tx.Nonce = 2 })
			assert.Nil(t, pool.Add(&tx2))

			// create a block containing tx1 on top of genesis
			block1 := unittest.ClusterBlockWithParent(genesis)
			block1.SetPayload(model.PayloadFromTransactions([]flow.Identifier{tx1.ID()}))
			insert(block1)

			// create a block containing tx2 on top of block1
			block2 := unittest.ClusterBlockWithParent(&block1)
			block2.SetPayload(model.PayloadFromTransactions([]flow.Identifier{tx2.ID()}))
			insert(block2)

			// finalize block1 (should NOT finalize block1)
			err := finalizer.MakeFinal(block1.ID())
			assert.Nil(t, err)

			// tx1 should have been removed from mempool
			assert.False(t, pool.Has(tx1.ID()))
			// tx2 should NOT have been removed from mempool (since block2 wasn't finalized)
			assert.True(t, pool.Has(tx2.ID()))

			// check finalized boundary using cluster state
			final, err := state.Final().Head()
			assert.Nil(t, err)
			assert.Equal(t, block1.ID(), final.ID())
		})

		// when finalizing a block with a conflicting fork, the fork should
		// not be finalized.
		t.Run("conflicting fork", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			// tx1 is included in the finalized block and mempool
			tx1 := unittest.TransactionFixture(func(tx *flow.Transaction) { tx.Nonce = 1 })
			assert.Nil(t, pool.Add(&tx1))
			// tx2 is included in the conflicting block and mempool
			tx2 := unittest.TransactionFixture(func(tx *flow.Transaction) { tx.Nonce = 2 })
			assert.Nil(t, pool.Add(&tx2))

			// create a block containing tx1 on top of genesis
			block1 := unittest.ClusterBlockWithParent(genesis)
			block1.SetPayload(model.PayloadFromTransactions([]flow.Identifier{tx1.ID()}))
			insert(block1)

			// create a block containing tx2 on top of genesis (conflicting with block1)
			block2 := unittest.ClusterBlockWithParent(genesis)
			block2.SetPayload(model.PayloadFromTransactions([]flow.Identifier{tx2.ID()}))
			insert(block2)

			// finalize block2
			err := finalizer.MakeFinal(block1.ID())
			assert.Nil(t, err)

			// tx1 should have been removed from mempool
			assert.False(t, pool.Has(tx1.ID()))
			// tx2 should NOT have been removed from mempool (since block2 wasn't finalized)
			assert.True(t, pool.Has(tx2.ID()))

			// check finalized boundary using cluster state
			t.Log("1: ", block1.ID())
			t.Log("2: ", block2.ID())
			final, err := state.Final().Head()
			assert.Nil(t, err)
			assert.Equal(t, block1.ID(), final.ID())
		})
	})
}
