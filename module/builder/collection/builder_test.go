package collection_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	model "github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/builder/collection"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	cluster "github.com/dapperlabs/flow-go/state/cluster/badger"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestBuilder(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		genesis := model.Genesis()
		chainID := genesis.ChainID

		// create a transaction pool, reset between test runs
		pool, err := stdmap.NewTransactions(1000)
		require.Nil(t, err)

		state, err := cluster.NewState(db, chainID)
		require.Nil(t, err)

		// empty setter for use in BuildOn
		noopSetter := func(*flow.Header) {}

		// helper function for inserting a block
		insert := func(block *model.Block) {
			err := db.Update(func(tx *badger.Txn) error {
				err = procedure.InsertClusterBlock(block)(tx)
				assert.Nil(t, err)
				return nil
			})
			require.Nil(t, err)
		}

		// helper function for bootstrapping chain/mempool state
		bootstrap := func() {
			// bootstrap chain state with genesis block
			err = state.Mutate().Bootstrap(genesis)
			require.Nil(t, err)
			// add some transactions to transaction pool
			for i := 0; i < 3; i++ {
				transaction := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) {
					tx.Nonce = uint64(i)
				})
				err = pool.Add(&transaction)
				require.Nil(t, err)
			}
		}

		// helper function for cleaning up state between tests
		cleanup := func() {
			// reset transaction pool
			pool, err = stdmap.NewTransactions(1000)
			require.Nil(t, err)
			// wipe database
			err = db.DropAll()
			require.Nil(t, err)
		}

		// building on a non-existent parent should throw an error
		t.Run("non-existent parent", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			// use a non-existent parent ID
			parentID := unittest.IdentifierFixture()

			builder := collection.NewBuilder(db, pool, chainID)
			_, err = builder.BuildOn(parentID, noopSetter)
			assert.Error(t, err)
		})

		// should build a block containing all items from mempool
		t.Run("build", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			var expectedView uint64 = 42
			setter := func(h *flow.Header) {
				h.View = expectedView
			}

			builder := collection.NewBuilder(db, pool, chainID)
			header, err := builder.BuildOn(genesis.ID(), setter)
			assert.Nil(t, err)

			// setter should have been run
			assert.Equal(t, expectedView, header.View)

			// should be able to retrieve built block from storage
			var built model.Block
			err = db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
			assert.Nil(t, err)
			builtCollection := built.Collection

			// payload should include only items from mempool
			mempoolTransactions := pool.All()
			assert.Len(t, builtCollection.Transactions, 3)
			assert.True(t, collectionContains(builtCollection, flow.GetIDs(mempoolTransactions)...))
		})

		// with two conflicting forks (tx1 in fork 1, tx2 in fork 2) and tx3 in mempool,
		// if we build on fork 1 our block should contain tx2 and tx3 only.
		t.Run("with forks", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			mempoolTransactions := pool.All()
			tx1 := mempoolTransactions[0] // in fork 1
			tx2 := mempoolTransactions[1] // in fork 2
			tx3 := mempoolTransactions[2] // in no block

			// build first fork on top of genesis
			payload1 := model.PayloadFromTransactions(tx1)
			block1 := unittest.ClusterBlockWithParent(genesis)
			block1.SetPayload(payload1)

			// insert block on fork 1
			insert(&block1)

			// build second fork on top of genesis
			payload2 := model.PayloadFromTransactions(tx2)
			block2 := unittest.ClusterBlockWithParent(genesis)
			block2.SetPayload(payload2)

			// insert block on fork 2
			insert(&block2)

			// build on top of fork 1
			builder := collection.NewBuilder(db, pool, chainID)
			header, err := builder.BuildOn(block1.ID(), noopSetter)
			require.Nil(t, err)

			// should be able to retrieve built block from storage
			var built model.Block
			err = db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
			assert.Nil(t, err)
			builtCollection := built.Collection

			// payload should include ONLY tx2 and tx3
			assert.Len(t, builtCollection.Transactions, 2)
			assert.True(t, collectionContains(builtCollection, tx2.ID(), tx3.ID()))
			assert.False(t, collectionContains(builtCollection, tx1.ID()))
		})

		// when building a block on a chain containing finalized and
		// un-finalized blocks, should avoid conflicts with both
		t.Run("conflicting finalized block", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			mempoolTransactions := pool.All()
			tx1 := mempoolTransactions[0] // in a finalized block
			tx2 := mempoolTransactions[1] // in an un-finalized block
			tx3 := mempoolTransactions[2] // in no blocks

			// build a block containing tx1 on genesis
			finalizedPayload := model.PayloadFromTransactions(tx1)
			finalizedBlock := unittest.ClusterBlockWithParent(genesis)
			finalizedBlock.SetPayload(finalizedPayload)
			insert(&finalizedBlock)

			// build a block containing tx2 on the first block
			unFinalizedPayload := model.PayloadFromTransactions(tx2)
			unFinalizedBlock := unittest.ClusterBlockWithParent(&finalizedBlock)
			unFinalizedBlock.SetPayload(unFinalizedPayload)
			insert(&unFinalizedBlock)

			// finalize first block
			err = db.Update(procedure.FinalizeClusterBlock(finalizedBlock.ID()))
			assert.Nil(t, err)

			// build on the un-finalized block
			builder := collection.NewBuilder(db, pool, chainID)
			header, err := builder.BuildOn(unFinalizedBlock.ID(), noopSetter)
			require.Nil(t, err)

			// retrieve the built block from storage
			var built model.Block
			err = db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
			assert.Nil(t, err)
			builtCollection := built.Collection

			// payload should only contain tx3
			assert.Len(t, builtCollection.Transactions, 1)
			assert.True(t, collectionContains(builtCollection, tx3.ID()))
			assert.False(t, collectionContains(builtCollection, tx1.ID(), tx2.ID()))

			// tx1 should be removed from mempool, as it is in a finalized block
			assert.False(t, pool.Has(tx1.ID()))
		})

		// should include transactions on a conflicting invalidated fork
		t.Run("conflicting invalidated fork", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			mempoolTransactions := pool.All()
			tx1 := mempoolTransactions[0] // in a finalized block
			tx2 := mempoolTransactions[1] // in an invalidated block
			tx3 := mempoolTransactions[2] // in no blocks

			// build a block containing tx1 on genesis - will be finalized
			finalizedPayload := model.PayloadFromTransactions(tx1)
			finalizedBlock := unittest.ClusterBlockWithParent(genesis)
			finalizedBlock.SetPayload(finalizedPayload)
			insert(&finalizedBlock)

			// build a block containing tx2 ALSO on genesis - will be invalidated
			invalidatedPayload := model.PayloadFromTransactions(tx2)
			invalidatedBlock := unittest.ClusterBlockWithParent(genesis)
			invalidatedBlock.SetPayload(invalidatedPayload)
			insert(&invalidatedBlock)

			// finalize first block - this indirectly invalidates the second block
			err = db.Update(procedure.FinalizeClusterBlock(finalizedBlock.ID()))
			assert.Nil(t, err)

			// build on the finalized block
			builder := collection.NewBuilder(db, pool, chainID)
			header, err := builder.BuildOn(finalizedBlock.ID(), noopSetter)
			require.Nil(t, err)

			// retrieve the built block from storage
			var built model.Block
			err = db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
			assert.Nil(t, err)
			builtCollection := built.Collection

			// tx2 and tx3 should be in the built collection
			assert.Len(t, builtCollection.Transactions, 2)
			assert.True(t, collectionContains(builtCollection, tx2.ID(), tx3.ID()))
			assert.False(t, collectionContains(builtCollection, tx1.ID()))

			// tx1 should be removed from mempool, as it is in a finalized block
			assert.False(t, pool.Has(tx1.ID()))
		})

		// should function correctly with large history
		t.Run("large history", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			// seed the RNG
			rand.Seed(time.Now().UnixNano())

			// use a mempool with 1000 transactions, one per block
			pool, err = stdmap.NewTransactions(2000)
			require.Nil(t, err)

			// keep track of the head of the chain
			head := *genesis

			// keep track of invalidated transaction IDs
			var invalidatedTxIds []flow.Identifier

			// create 1000 blocks containing 1000 transactions
			for i := 0; i < 1000; i++ {

				// create a transaction
				tx := unittest.TransactionBodyFixture(func(t *flow.TransactionBody) { t.Nonce = uint64(i) })
				err = pool.Add(&tx)
				assert.Nil(t, err)

				// 1/3 of the time create a conflicting fork that will be invalidated
				// don't do this the first and last few times to ensure we don't
				// try to fork genesis and the the last block is the valid fork.
				conflicting := rand.Intn(3) == 0 && i > 5 && i < 995

				// by default, build on the head - if we are building a
				// conflicting fork, build on the parent of the head
				parent := head
				if conflicting {
					err = db.View(procedure.RetrieveClusterBlock(parent.ParentID, &parent))
					assert.Nil(t, err)
					// add the transaction to the invalidated list
					invalidatedTxIds = append(invalidatedTxIds, tx.ID())
				}

				// create a block containing the transaction
				block := unittest.ClusterBlockWithParent(&head)
				payload := model.PayloadFromTransactions(&tx)
				block.SetPayload(payload)
				insert(&block)

				// reset the valid head if we aren't building a conflicting fork
				if !conflicting {
					head = block
					err = db.Update(procedure.FinalizeClusterBlock(block.ID()))
					assert.Nil(t, err)
				}
			}

			t.Log("conflicting: ", len(invalidatedTxIds))

			// build on the the head block
			builder := collection.NewBuilder(db, pool, chainID)
			header, err := builder.BuildOn(head.ID(), noopSetter)
			require.Nil(t, err)

			// retrieve the built block from storage
			var built model.Block
			err = db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
			require.Nil(t, err)
			builtCollection := built.Collection

			// payload should only contain transactions from invalidated blocks
			assert.Len(t, builtCollection.Transactions, len(invalidatedTxIds))
			assert.True(t, collectionContains(builtCollection, invalidatedTxIds...))
		})

		// TODO specify behaviour for empty mempools
		t.Run("empty mempool", func(t *testing.T) {
			t.Skip()
		})
	})
}

// helper to check whether a collection contains each of the given transactions.
func collectionContains(collection flow.Collection, txIDs ...flow.Identifier) bool {

	lookup := make(map[flow.Identifier]struct{}, len(txIDs))
	for _, tx := range collection.Transactions {
		lookup[tx.ID()] = struct{}{}
	}

	for _, txID := range txIDs {
		_, exists := lookup[txID]
		if !exists {
			return false
		}
	}

	return true
}
