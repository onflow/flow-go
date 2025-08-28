package collection_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	collectionmock "github.com/onflow/flow-go/engine/collection/mock"
	model "github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/finalizer/collection"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	cluster "github.com/onflow/flow-go/state/cluster/badger"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/procedure"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestFinalizer(t *testing.T) {
	// This test has to build on top of badgerdb, because the cleanup method depends
	// on the badgerdb.DropAll method to wipe the database, which pebble does not support.
	unittest.RunWithBadgerDB(t, func(badgerdb *badger.DB) {
		lockManager := storage.NewTestingLockManager()
		db := badgerimpl.ToDB(badgerdb)
		// reference block on the main consensus chain
		refBlock := unittest.ClusterBlockFixture()
		// genesis block for the cluster chain
		genesis, err := unittest.ClusterBlock.Genesis()
		require.NoError(t, err)

		metrics := metrics.NewNoopCollector()
		pool := herocache.NewTransactions(1000, unittest.Logger(), metrics)

		var state *cluster.State

		// a helper function to clean up shared state between tests
		cleanup := func() {
			// wipe the DB
			err := badgerdb.DropAll()
			require.NoError(t, err)
			// clear the mempool
			for _, tx := range pool.All() {
				pool.Remove(tx.ID())
			}
		}

		// a helper function to bootstrap with the genesis block
		bootstrap := func() {
			stateRoot, err := cluster.NewStateRoot(genesis, unittest.QuorumCertificateFixture(), 0)
			require.NoError(t, err)
			state, err = cluster.Bootstrap(db, lockManager, stateRoot)
			require.NoError(t, err)

			lctx := lockManager.NewContext()
			require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertHeader(lctx, rw, refBlock.ID(), refBlock.ToHeader())
			})
			require.NoError(t, err)
			lctx.Release()
		}

		// a helper function to insert a block
		insert := func(db storage.DB, lockManager lockctx.Manager, block *model.Block) {
			lctx := lockManager.NewContext()
			defer lctx.Release()
			require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return procedure.InsertClusterBlock(lctx, rw, unittest.ClusterProposalFromBlock(block))
			})
			assert.NoError(t, err)
		}

		t.Run("non-existent block", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			pusher := collectionmock.NewGuaranteedCollectionPublisher(t)
			finalizer := collection.NewFinalizer(db, lockManager, pool, pusher, metrics)

			fakeBlockID := unittest.IdentifierFixture()
			err := finalizer.MakeFinal(fakeBlockID)
			assert.Error(t, err)
		})

		t.Run("already finalized block", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			pusher := collectionmock.NewGuaranteedCollectionPublisher(t)
			pusher.On("SubmitCollectionGuarantee", mock.Anything).Once()
			finalizer := collection.NewFinalizer(db, lockManager, pool, pusher, metrics)

			// tx1 is included in the finalized block
			tx1 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 1 })
			assert.True(t, pool.Add(tx1.ID(), &tx1))

			// create a new block on genesis
			payload, err := model.NewPayload(
				model.UntrustedPayload{
					ReferenceBlockID: refBlock.ID(),
					Collection:       flow.Collection{Transactions: []*flow.TransactionBody{&tx1}},
				},
			)
			require.NoError(t, err)
			block := unittest.ClusterBlockFixture(
				unittest.ClusterBlock.WithParent(genesis),
				unittest.ClusterBlock.WithPayload(*payload),
			)

			insert(db, lockManager, block)

			// finalize the block
			err = finalizer.MakeFinal(block.ID())
			assert.NoError(t, err)

			// finalize the block again - this should be a no-op
			err = finalizer.MakeFinal(block.ID())
			assert.NoError(t, err)
		})

		t.Run("unconnected block", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			pusher := collectionmock.NewGuaranteedCollectionPublisher(t)
			finalizer := collection.NewFinalizer(db, lockManager, pool, pusher, metrics)

			// create a new block that isn't connected to a parent
			block := unittest.ClusterBlockFixture(
				unittest.ClusterBlock.WithParent(genesis),
				unittest.ClusterBlock.WithPayload(*model.NewEmptyPayload(refBlock.ID())),
			)
			block.ParentID = unittest.IdentifierFixture()

			insert(db, lockManager, block)

			// try to finalize - this should fail
			err := finalizer.MakeFinal(block.ID())
			assert.Error(t, err)
		})

		t.Run("empty collection block", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			pusher := collectionmock.NewGuaranteedCollectionPublisher(t)
			finalizer := collection.NewFinalizer(db, lockManager, pool, pusher, metrics)

			// create a block with empty payload on genesis
			block := unittest.ClusterBlockFixture(
				unittest.ClusterBlock.WithParent(genesis),
				unittest.ClusterBlock.WithPayload(*model.NewEmptyPayload(refBlock.ID())),
			)
			insert(db, lockManager, block)

			// finalize the block
			err := finalizer.MakeFinal(block.ID())
			assert.NoError(t, err)

			// check finalized boundary using cluster state
			final, err := state.Final().Head()
			assert.NoError(t, err)
			assert.Equal(t, block.ToHeader().ID(), final.ID())

			// collection should not have been propagated
		})

		t.Run("finalize single block", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			pusher := collectionmock.NewGuaranteedCollectionPublisher(t)
			finalizer := collection.NewFinalizer(db, lockManager, pool, pusher, metrics)

			// tx1 is included in the finalized block and mempool
			tx1 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 1 })
			assert.True(t, pool.Add(tx1.ID(), &tx1))
			// tx2 is only in the mempool
			tx2 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 2 })
			assert.True(t, pool.Add(tx2.ID(), &tx2))

			// create a block containing tx1 on top of genesis
			payload, err := model.NewPayload(
				model.UntrustedPayload{
					ReferenceBlockID: refBlock.ID(),
					Collection:       flow.Collection{Transactions: []*flow.TransactionBody{&tx1}},
				},
			)
			require.NoError(t, err)
			block := unittest.ClusterBlockFixture(
				unittest.ClusterBlock.WithParent(genesis),
				unittest.ClusterBlock.WithPayload(*payload),
			)
			insert(db, lockManager, block)

			// block should be passed to pusher
			pusher.On("SubmitCollectionGuarantee", &flow.CollectionGuarantee{
				CollectionID:     block.Payload.Collection.ID(),
				ReferenceBlockID: refBlock.ID(),
				ClusterChainID:   block.ChainID,
				SignerIndices:    block.ParentVoterIndices,
				Signature:        nil,
			}).Once()

			// finalize the block
			err = finalizer.MakeFinal(block.ID())
			assert.NoError(t, err)

			// tx1 should have been removed from mempool
			assert.False(t, pool.Has(tx1.ID()))
			// tx2 should still be in mempool
			assert.True(t, pool.Has(tx2.ID()))

			// check finalized boundary using cluster state
			final, err := state.Final().Head()
			assert.NoError(t, err)
			assert.Equal(t, block.ToHeader().ID(), final.ID())
			assertClusterBlocksIndexedByReferenceHeight(t, lockManager, db, refBlock.Height, final.ID())
		})

		// when finalizing a block with un-finalized ancestors, those ancestors should be finalized as well
		t.Run("finalize multiple blocks together", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			pusher := collectionmock.NewGuaranteedCollectionPublisher(t)
			finalizer := collection.NewFinalizer(db, lockManager, pool, pusher, metrics)

			// tx1 is included in the first finalized block and mempool
			tx1 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 1 })
			assert.True(t, pool.Add(tx1.ID(), &tx1))
			// tx2 is included in the second finalized block and mempool
			tx2 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 2 })
			assert.True(t, pool.Add(tx2.ID(), &tx2))

			// create a block containing tx1 on top of genesis
			payload, err := model.NewPayload(
				model.UntrustedPayload{
					ReferenceBlockID: refBlock.ID(),
					Collection:       flow.Collection{Transactions: []*flow.TransactionBody{&tx1}},
				},
			)
			require.NoError(t, err)
			block1 := unittest.ClusterBlockFixture(
				unittest.ClusterBlock.WithParent(genesis),
				unittest.ClusterBlock.WithPayload(*payload),
			)
			insert(db, lockManager, block1)

			// create a block containing tx2 on top of block1
			payload, err = model.NewPayload(
				model.UntrustedPayload{
					ReferenceBlockID: refBlock.ID(),
					Collection:       flow.Collection{Transactions: []*flow.TransactionBody{&tx2}},
				},
			)
			require.NoError(t, err)
			block2 := unittest.ClusterBlockFixture(
				unittest.ClusterBlock.WithParent(block1),
				unittest.ClusterBlock.WithPayload(*payload),
			)
			insert(db, lockManager, block2)

			// both blocks should be passed to pusher
			pusher.On("SubmitCollectionGuarantee", &flow.CollectionGuarantee{
				CollectionID:     block1.Payload.Collection.ID(),
				ReferenceBlockID: refBlock.ID(),
				ClusterChainID:   block1.ChainID,
				SignerIndices:    block1.ParentVoterIndices,
				Signature:        nil,
			}).Once()
			pusher.On("SubmitCollectionGuarantee", &flow.CollectionGuarantee{
				CollectionID:     block2.Payload.Collection.ID(),
				ReferenceBlockID: refBlock.ID(),
				ClusterChainID:   block2.ChainID,
				SignerIndices:    block2.ParentVoterIndices,
				Signature:        nil,
			}).Once()

			// finalize block2 (should indirectly finalize block1 as well)
			err = finalizer.MakeFinal(block2.ID())
			assert.NoError(t, err)

			// tx1 and tx2 should have been removed from mempool
			assert.False(t, pool.Has(tx1.ID()))
			assert.False(t, pool.Has(tx2.ID()))

			// check finalized boundary using cluster state
			final, err := state.Final().Head()
			assert.NoError(t, err)
			assert.Equal(t, block2.ToHeader().ID(), final.ID())
			assertClusterBlocksIndexedByReferenceHeight(t, lockManager, db, refBlock.Height, block1.ID(), block2.ID())
		})

		t.Run("finalize with un-finalized child", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			pusher := collectionmock.NewGuaranteedCollectionPublisher(t)
			finalizer := collection.NewFinalizer(db, lockManager, pool, pusher, metrics)

			// tx1 is included in the finalized parent block and mempool
			tx1 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 1 })
			assert.True(t, pool.Add(tx1.ID(), &tx1))
			// tx2 is included in the un-finalized block and mempool
			tx2 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 2 })
			assert.True(t, pool.Add(tx2.ID(), &tx2))

			// create a block containing tx1 on top of genesis
			payload, err := model.NewPayload(
				model.UntrustedPayload{
					ReferenceBlockID: refBlock.ID(),
					Collection:       flow.Collection{Transactions: []*flow.TransactionBody{&tx1}},
				},
			)
			require.NoError(t, err)
			block1 := unittest.ClusterBlockFixture(
				unittest.ClusterBlock.WithParent(genesis),
				unittest.ClusterBlock.WithPayload(*payload),
			)
			insert(db, lockManager, block1)

			// create a block containing tx2 on top of block1
			payload, err = model.NewPayload(
				model.UntrustedPayload{
					ReferenceBlockID: refBlock.ID(),
					Collection:       flow.Collection{Transactions: []*flow.TransactionBody{&tx2}},
				},
			)
			require.NoError(t, err)
			block2 := unittest.ClusterBlockFixture(
				unittest.ClusterBlock.WithParent(block1),
				unittest.ClusterBlock.WithPayload(*payload),
			)
			insert(db, lockManager, block2)

			// block should be passed to pusher
			pusher.On("SubmitCollectionGuarantee", &flow.CollectionGuarantee{
				CollectionID:     block1.Payload.Collection.ID(),
				ReferenceBlockID: refBlock.ID(),
				ClusterChainID:   block1.ChainID,
				SignerIndices:    block1.ParentVoterIndices,
				Signature:        nil,
			}).Once()

			// finalize block1 (should NOT finalize block2)
			err = finalizer.MakeFinal(block1.ID())
			assert.NoError(t, err)

			// tx1 should have been removed from mempool
			assert.False(t, pool.Has(tx1.ID()))
			// tx2 should NOT have been removed from mempool (since block2 wasn't finalized)
			assert.True(t, pool.Has(tx2.ID()))

			// check finalized boundary using cluster state
			final, err := state.Final().Head()
			assert.NoError(t, err)
			assert.Equal(t, block1.ToHeader().ID(), final.ID())
			assertClusterBlocksIndexedByReferenceHeight(t, lockManager, db, refBlock.Height, block1.ID())
		})

		// when finalizing a block with a conflicting fork, the fork should not be finalized.
		t.Run("conflicting fork", func(t *testing.T) {
			bootstrap()
			defer cleanup()

			pusher := collectionmock.NewGuaranteedCollectionPublisher(t)
			finalizer := collection.NewFinalizer(db, lockManager, pool, pusher, metrics)

			// tx1 is included in the finalized block and mempool
			tx1 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 1 })
			assert.True(t, pool.Add(tx1.ID(), &tx1))
			// tx2 is included in the conflicting block and mempool
			tx2 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) { tx.ProposalKey.SequenceNumber = 2 })
			assert.True(t, pool.Add(tx2.ID(), &tx2))

			// create a block containing tx1 on top of genesis
			payload, err := model.NewPayload(
				model.UntrustedPayload{
					ReferenceBlockID: refBlock.ID(),
					Collection:       flow.Collection{Transactions: []*flow.TransactionBody{&tx1}},
				},
			)
			require.NoError(t, err)
			block1 := unittest.ClusterBlockFixture(
				unittest.ClusterBlock.WithParent(genesis),
				unittest.ClusterBlock.WithPayload(*payload),
			)
			insert(db, lockManager, block1)

			// create a block containing tx2 on top of genesis (conflicting with block1)
			payload, err = model.NewPayload(
				model.UntrustedPayload{
					ReferenceBlockID: refBlock.ID(),
					Collection:       flow.Collection{Transactions: []*flow.TransactionBody{&tx2}},
				},
			)
			require.NoError(t, err)
			block2 := unittest.ClusterBlockFixture(
				unittest.ClusterBlock.WithParent(genesis),
				unittest.ClusterBlock.WithPayload(*payload),
			)
			insert(db, lockManager, block2)

			// block should be passed to pusher
			pusher.On("SubmitCollectionGuarantee", &flow.CollectionGuarantee{
				CollectionID:     block1.Payload.Collection.ID(),
				ReferenceBlockID: refBlock.ID(),
				ClusterChainID:   block1.ChainID,
				SignerIndices:    block1.ParentVoterIndices,
				Signature:        nil,
			}).Once()

			// finalize block1
			err = finalizer.MakeFinal(block1.ID())
			assert.NoError(t, err)

			// tx1 should have been removed from mempool
			assert.False(t, pool.Has(tx1.ID()))
			// tx2 should NOT have been removed from mempool (since block2 wasn't finalized)
			assert.True(t, pool.Has(tx2.ID()))

			// check finalized boundary using cluster state
			final, err := state.Final().Head()
			assert.NoError(t, err)
			assert.Equal(t, block1.ToHeader().ID(), final.ID())
			assertClusterBlocksIndexedByReferenceHeight(t, lockManager, db, refBlock.Height, block1.ID())
		})
	})
}

// assertClusterBlocksIndexedByReferenceHeight checks the given cluster blocks have
// been indexed by the given reference block height, which is expected as part of
// finalization.
func assertClusterBlocksIndexedByReferenceHeight(t *testing.T, lockManager lockctx.Manager, db storage.DB, refHeight uint64, clusterBlockIDs ...flow.Identifier) {
	var ids []flow.Identifier
	lctx := lockManager.NewContext()
	defer lctx.Release()
	require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
	err := operation.LookupClusterBlocksByReferenceHeightRange(lctx, db.Reader(), refHeight, refHeight, &ids)
	require.NoError(t, err)
	assert.ElementsMatch(t, clusterBlockIDs, ids)
}
