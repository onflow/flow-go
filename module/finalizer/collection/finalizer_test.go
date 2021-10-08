package collection_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	model "github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/finalizer/collection"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/mocknetwork"
	cluster "github.com/onflow/flow-go/state/cluster/badger"
	"github.com/onflow/flow-go/storage/badger/procedure"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestFinalizer(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		// seed the RNG
		rand.Seed(time.Now().UnixNano())

		genesis := model.Genesis()

		metrics := metrics.NewNoopCollector()

		var state *cluster.State

		pool := stdmap.NewTransactions(1000)

		// a helper function to clean up shared state between tests
		cleanup := func() {
			// wipe the DB
			err := db.DropAll()
			require.Nil(t, err)
			// clear the mempool
			for _, tx := range pool.All() {
				pool.Rem(tx.ID())
			}
		}

		// a helper function to bootstrap with the genesis block
		bootstrap := func() {
			stateRoot, err := cluster.NewStateRoot(genesis)
			require.NoError(t, err)
			state, err = cluster.Bootstrap(db, stateRoot)
			require.NoError(t, err)
		}

		// a helper function to insert a block
		insert := func(block model.Block) {
			err := db.Update(procedure.InsertClusterBlock(&block))
			assert.Nil(t, err)
		}

		t.Run("non-existent block", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			prov := new(mocknetwork.Engine)
			prov.On("SubmitLocal", mock.Anything)
			finalizer := collection.NewFinalizer(db, pool, prov, metrics)

			fakeBlockID := unittest.IdentifierFixture()
			err := finalizer.MakeFinal(fakeBlockID)
			assert.Error(t, err)
		})

		t.Run("already finalized block", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			prov := new(mocknetwork.Engine)
			prov.On("SubmitLocal", mock.Anything)
			finalizer := collection.NewFinalizer(db, pool, prov, metrics)

			// tx1 is included in the finalized block
			tx1 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 1 })
			assert.True(t, pool.Add(&tx1))

			// create a new block on genesis
			block := unittest.ClusterBlockWithParent(genesis)
			block.SetPayload(model.PayloadFromTransactions(flow.ZeroID, &tx1))
			insert(block)

			// finalize the block
			err := finalizer.MakeFinal(block.ID())
			assert.Nil(t, err)

			// finalize the block again - this should be a no-op
			err = finalizer.MakeFinal(block.ID())
			assert.Nil(t, err)
		})

		t.Run("unconnected block", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			prov := new(mocknetwork.Engine)
			prov.On("SubmitLocal", mock.Anything)
			finalizer := collection.NewFinalizer(db, pool, prov, metrics)

			// create a new block that isn't connected to a parent
			block := unittest.ClusterBlockWithParent(genesis)
			block.Header.ParentID = unittest.IdentifierFixture()
			insert(block)

			// try to finalize - this should fail
			err := finalizer.MakeFinal(block.ID())
			assert.Error(t, err)
		})

		t.Run("empty collection block", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			prov := new(mocknetwork.Engine)
			finalizer := collection.NewFinalizer(db, pool, prov, metrics)

			// create a block with empty payload on genesis
			block := unittest.ClusterBlockWithParent(genesis)
			block.SetPayload(model.EmptyPayload(flow.ZeroID))
			insert(block)

			// finalize the block
			err := finalizer.MakeFinal(block.ID())
			assert.Nil(t, err)

			// check finalized boundary using cluster state
			final, err := state.Final().Head()
			assert.Nil(t, err)
			assert.Equal(t, block.ID(), final.ID())

			// collection should not have been propagated
			prov.AssertNotCalled(t, "SubmitLocal", mock.Anything)
		})

		t.Run("finalize single block", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			prov := new(mocknetwork.Engine)
			prov.On("SubmitLocal", mock.Anything)
			finalizer := collection.NewFinalizer(db, pool, prov, metrics)

			// tx1 is included in the finalized block and mempool
			tx1 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 1 })
			assert.True(t, pool.Add(&tx1))
			// tx2 is only in the mempool
			tx2 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 2 })
			assert.True(t, pool.Add(&tx2))

			// create a block containing tx1 on top of genesis
			block := unittest.ClusterBlockWithParent(genesis)
			block.SetPayload(model.PayloadFromTransactions(flow.ZeroID, &tx1))
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

			// block should be passed to provider
			prov.AssertNumberOfCalls(t, "SubmitLocal", 1)
			prov.AssertCalled(t, "SubmitLocal", &messages.SubmitCollectionGuarantee{
				Guarantee: flow.CollectionGuarantee{
					CollectionID: block.Payload.Collection.ID(),
					SignerIDs:    block.Header.ParentVoterIDs,
					Signature:    block.Header.ParentVoterSigData,
				},
			})
		})

		// when finalizing a block with un-finalized ancestors, those ancestors
		// should be finalized as well
		t.Run("finalize multiple blocks together", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			prov := new(mocknetwork.Engine)
			prov.On("SubmitLocal", mock.Anything)
			finalizer := collection.NewFinalizer(db, pool, prov, metrics)

			// tx1 is included in the first finalized block and mempool
			tx1 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 1 })
			assert.True(t, pool.Add(&tx1))
			// tx2 is included in the second finalized block and mempool
			tx2 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 2 })
			assert.True(t, pool.Add(&tx2))

			// create a block containing tx1 on top of genesis
			block1 := unittest.ClusterBlockWithParent(genesis)
			block1.SetPayload(model.PayloadFromTransactions(flow.ZeroID, &tx1))
			insert(block1)

			// create a block containing tx2 on top of block1
			block2 := unittest.ClusterBlockWithParent(&block1)
			block2.SetPayload(model.PayloadFromTransactions(flow.ZeroID, &tx2))
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

			// both blocks should be passed to provider
			prov.AssertNumberOfCalls(t, "SubmitLocal", 2)
			prov.AssertCalled(t, "SubmitLocal", &messages.SubmitCollectionGuarantee{
				Guarantee: flow.CollectionGuarantee{
					CollectionID: block1.Payload.Collection.ID(),
					SignerIDs:    block1.Header.ParentVoterIDs,
					Signature:    block1.Header.ParentVoterSigData,
				},
			})
			prov.AssertCalled(t, "SubmitLocal", &messages.SubmitCollectionGuarantee{
				Guarantee: flow.CollectionGuarantee{
					CollectionID: block2.Payload.Collection.ID(),
					SignerIDs:    block2.Header.ParentVoterIDs,
					Signature:    block2.Header.ParentVoterSigData,
				},
			})
		})

		t.Run("finalize with un-finalized child", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			prov := new(mocknetwork.Engine)
			prov.On("SubmitLocal", mock.Anything)
			finalizer := collection.NewFinalizer(db, pool, prov, metrics)

			// tx1 is included in the finalized parent block and mempool
			tx1 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 1 })
			assert.True(t, pool.Add(&tx1))
			// tx2 is included in the un-finalized block and mempool
			tx2 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 2 })
			assert.True(t, pool.Add(&tx2))

			// create a block containing tx1 on top of genesis
			block1 := unittest.ClusterBlockWithParent(genesis)
			block1.SetPayload(model.PayloadFromTransactions(flow.ZeroID, &tx1))
			insert(block1)

			// create a block containing tx2 on top of block1
			block2 := unittest.ClusterBlockWithParent(&block1)
			block2.SetPayload(model.PayloadFromTransactions(flow.ZeroID, &tx2))
			insert(block2)

			// finalize block1 (should NOT finalize block2)
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

			// block should be passed to provider
			prov.AssertNumberOfCalls(t, "SubmitLocal", 1)
			prov.AssertCalled(t, "SubmitLocal", &messages.SubmitCollectionGuarantee{
				Guarantee: flow.CollectionGuarantee{
					CollectionID: block1.Payload.Collection.ID(),
					SignerIDs:    block1.Header.ParentVoterIDs,
					Signature:    block1.Header.ParentVoterSigData,
				},
			})
		})

		// when finalizing a block with a conflicting fork, the fork should
		// not be finalized.
		t.Run("conflicting fork", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			prov := new(mocknetwork.Engine)
			prov.On("SubmitLocal", mock.Anything)
			finalizer := collection.NewFinalizer(db, pool, prov, metrics)

			// tx1 is included in the finalized block and mempool
			tx1 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 1 })
			assert.True(t, pool.Add(&tx1))
			// tx2 is included in the conflicting block and mempool
			tx2 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 2 })
			assert.True(t, pool.Add(&tx2))

			// create a block containing tx1 on top of genesis
			block1 := unittest.ClusterBlockWithParent(genesis)
			block1.SetPayload(model.PayloadFromTransactions(flow.ZeroID, &tx1))
			insert(block1)

			// create a block containing tx2 on top of genesis (conflicting with block1)
			block2 := unittest.ClusterBlockWithParent(genesis)
			block2.SetPayload(model.PayloadFromTransactions(flow.ZeroID, &tx2))
			insert(block2)

			// finalize block2
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

			// block should be passed to provider
			prov.AssertNumberOfCalls(t, "SubmitLocal", 1)
			prov.AssertCalled(t, "SubmitLocal", &messages.SubmitCollectionGuarantee{
				Guarantee: flow.CollectionGuarantee{
					CollectionID: block1.Payload.Collection.ID(),
					SignerIDs:    block1.Header.ParentVoterIDs,
					Signature:    block1.Header.ParentVoterSigData,
				},
			})
		})
	})
}
