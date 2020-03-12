package collection

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cluster "github.com/dapperlabs/flow-go/cluster/badger"
	model "github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
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
				err = operation.InsertCollection(&block.Collection)(tx)
				assert.Nil(t, err)
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
				transaction := unittest.TransactionFixture(func(tx *flow.Transaction) {
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

			builder := NewBuilder(db, pool, chainID)
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

			builder := NewBuilder(db, pool, chainID)
			header, err := builder.BuildOn(genesis.ID(), setter)
			assert.Nil(t, err)

			// setter should have been run
			assert.Equal(t, expectedView, header.View)

			// should be able to retrieve built block from storage
			var block model.Block
			err = db.View(procedure.RetrieveClusterBlock(header.ID(), &block))
			assert.Nil(t, err)

			// payload should include only items from mempool
			mempoolTransactions := pool.All()
			assert.Equal(t, len(mempoolTransactions), len(block.Collection.Transactions))

			// create a lookup for all the transactions in the payload
			txLookup := make(map[flow.Identifier]struct{})
			for _, txID := range block.Collection.Transactions {
				txLookup[txID] = struct{}{}
			}

			// every transaction in the mempool should be in the payload
			for _, tx := range mempoolTransactions {
				_, exists := txLookup[tx.ID()]
				assert.True(t, exists)
			}
		})

		// with two conflicting forks (tx1 in fork 1, tx2 in fork 2) and tx3 in mempool,
		// if we build on fork 1 our block should contain tx2 and tx3 only.
		t.Run("with forks", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			mempoolTransactions := pool.All()
			// the first transaction in the pool will be in fork 1
			tx1 := mempoolTransactions[0]
			// the second transaction in the pool will be in fork 2
			tx2 := mempoolTransactions[1]

			// build first fork on top of genesis
			payload1 := model.PayloadFromTransactions([]flow.Identifier{tx1.ID()})
			block1 := unittest.ClusterBlockWithParent(genesis)
			block1.SetPayload(payload1)

			// insert block on fork 1
			insert(&block1)

			// build second fork on top of genesis
			payload2 := model.PayloadFromTransactions([]flow.Identifier{tx2.ID()})
			block2 := unittest.ClusterBlockWithParent(genesis)
			block2.SetPayload(payload2)

			// insert block on fork 2
			insert(&block2)

			// build on top of fork 1
			builder := NewBuilder(db, pool, chainID)
			header, err := builder.BuildOn(block1.ID(), noopSetter)
			require.Nil(t, err)

			// should be able to retrieve built block from storage
			var block model.Block
			err = db.View(procedure.RetrieveClusterBlock(header.ID(), &block))
			assert.Nil(t, err)

			// payload should include only items from mempool
			// it should have one fewer item (should not include tx1)
			assert.Equal(t, len(mempoolTransactions)-1, len(block.Collection.Transactions))

			// create a lookup for all the transactions in the payload
			txLookup := make(map[flow.Identifier]struct{})
			for _, txID := range block.Collection.Transactions {
				txLookup[txID] = struct{}{}
			}

			// every transaction in the mempool (except tx1) should be in the payload
			for _, tx := range mempoolTransactions {
				_, exists := txLookup[tx.ID()]
				if tx.ID() == tx1.ID() {
					assert.False(t, exists)
				} else {
					assert.True(t, exists)
				}
			}
		})

		t.Run("empty mempool", func(t *testing.T) {
			t.Skip()
		})
	})
}
