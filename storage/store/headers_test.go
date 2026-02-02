package store_test

import (
	"testing"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestHeaderStoreRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		all, err := store.InitAll(metrics, db, flow.Emulator)
		require.NoError(t, err)
		headers := all.Headers
		blocks := all.Blocks

		proposal := unittest.ProposalFixture()
		block := proposal.Block

		// store block which will also store header
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, proposal)
			})
		})
		require.NoError(t, err)

		// index the header
		err = unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx2 lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexFinalizedBlockByHeight(lctx2, rw, block.Height, block.ID())
			})
		})
		require.NoError(t, err)

		// retrieve header by height
		actual, err := headers.ByHeight(block.Height)
		require.NoError(t, err)
		require.Equal(t, block.ToHeader(), actual)
		// retrieve by ID
		actual, err = headers.ByBlockID(block.ID())
		require.NoError(t, err)
		require.Equal(t, block.ToHeader(), actual)
		// retrieve with proposer signature
		headerProp, err := headers.ProposalByBlockID(block.ID())
		require.NoError(t, err)
		require.Equal(t, proposal.ProposalHeader(), headerProp)
	})
}

func TestHeaderIndexByViewAndRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		all, err := store.InitAll(metrics, db, flow.Emulator)
		require.NoError(t, err)
		headers := all.Headers
		blocks := all.Blocks

		proposal := unittest.ProposalFixture()
		block := proposal.Block

		// store block which will also store header
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, proposal)
			})
		})
		require.NoError(t, err)

		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				// index the header
				return operation.IndexCertifiedBlockByView(lctx, rw, block.View, block.ID())
			})
		})
		require.NoError(t, err)

		// retrieve header by view
		actual, err := headers.ByView(block.View)
		require.NoError(t, err)
		require.Equal(t, block.ToHeader(), actual)

		// verify error sentinel of cluster Headers ByView
		clusterChainID := cluster.CanonicalClusterID(0, unittest.IdentifierListFixture(1))
		clusterHeaders, err := store.NewClusterHeaders(metrics, db, clusterChainID)
		require.NoError(t, err)
		_, err = clusterHeaders.ByView(block.View)
		require.ErrorIs(t, err, storage.ErrNotAvailableForClusterConsensus)
		_, err = clusterHeaders.ByView(block.View + 1)
		require.ErrorIs(t, err, storage.ErrNotAvailableForClusterConsensus)
	})
}

func TestHeaderRetrieveWithoutStore(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		headers, err := store.NewHeaders(metrics, db, flow.Emulator)
		require.NoError(t, err)

		header := unittest.BlockHeaderFixture()

		// retrieve header by height, should err as not store before height
		_, err = headers.ByHeight(header.Height)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

// TestHeadersByParentID tests method [Headers.ByParentID] for:
//  1. a known parent with no children should return an empty list;
//  2. a known parent with 3 children should return the headers of those children;
//  3. an unknown parent should return [storage.ErrNotFound].
//  4. a known parent on a different chain should return [storage.ErrWrongChain].
func TestHeadersByParentID(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		all, err := store.InitAll(metrics, db, flow.Emulator)
		require.NoError(t, err)
		headers := all.Headers
		blocks := all.Blocks

		// Create a parent block
		parentProposal := unittest.ProposalFixture()
		parentBlock := parentProposal.Block

		// Store parent block
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, parentProposal)
			})
		})
		require.NoError(t, err)

		// Test case 1: Parent with no children should return empty list
		children, err := headers.ByParentID(parentBlock.ID())
		require.NoError(t, err)
		require.Empty(t, children)

		// Test case 2: Parent with 3 children
		var childProposals []*flow.Proposal
		for i := 0; i < 3; i++ {
			childProposal := unittest.ProposalFromBlock(unittest.BlockWithParentFixture(parentBlock.ToHeader()))
			childProposals = append(childProposals, childProposal)

			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					// Store the block
					err := blocks.BatchStore(lctx, rw, childProposal)
					if err != nil {
						return err
					}
					// Index the parent-child relationship
					return operation.IndexNewBlock(lctx, rw, childProposal.Block.ID(), childProposal.Block.ParentID)
				})
			})
			require.NoError(t, err)
		}

		// confirm correct behaviour for test case 2: we should retrieve the headers of the 3 children
		children, err = headers.ByParentID(parentBlock.ID())
		require.NoError(t, err)
		require.ElementsMatch(t,
			children,
			[]*flow.Header{childProposals[0].Block.ToHeader(), childProposals[1].Block.ToHeader(), childProposals[2].Block.ToHeader()})

		// Test case 3: Non-existent parent should return ErrNotFound
		nonExistentParent := unittest.IdentifierFixture()
		_, err = headers.ByParentID(nonExistentParent)
		require.ErrorIs(t, err, storage.ErrNotFound)

		// Test case 4: parent on a different chain should return ErrWrongChain
		clusterBlock := unittest.ClusterBlockFixture()
		err = unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertClusterBlock(lctx, rw, unittest.ClusterProposalFromBlock(clusterBlock))
			})
		})
		require.NoError(t, err)
		_, err = headers.ByParentID(clusterBlock.ID())
		require.ErrorIs(t, err, storage.ErrWrongChain)
	})
}

// TestHeadersByParentIDChainStructure tests method [Headers.ByParentID] for a tree of blocks with
// deeper ancestry (children and grandchildren). Specifically, we use the following fork structure,
// which blocks denoted in square brackets:
//
//	                    ↙ [grandchild1]
//	[parent] ← [child]
//	                    ↖ [grandchild2]
func TestHeadersByParentIDChainStructure(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		all, err := store.InitAll(metrics, db, flow.Emulator)
		require.NoError(t, err)
		headers := all.Headers
		blocks := all.Blocks

		// Create child structure: parent -> child -> grandchild1, grandchild2
		parent := unittest.BlockFixture()
		child := unittest.BlockWithParentFixture(parent.ToHeader())
		grandchild1 := unittest.BlockWithParentFixture(child.ToHeader())
		grandchild2 := unittest.BlockWithParentFixture(child.ToHeader())

		// Store all blocks
		for _, b := range []*flow.Block{parent, child, grandchild1, grandchild2} {
			proposal := unittest.ProposalFromBlock(b)

			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					// Store the block
					err := blocks.BatchStore(lctx, rw, proposal)
					if err != nil {
						return err
					}
					// Index the parent-child relationship (skip for root block)
					if proposal.Block.ParentID != flow.ZeroID {
						return operation.IndexNewBlock(lctx, rw, proposal.Block.ID(), proposal.Block.ParentID)
					}
					return nil
				})
			})
			require.NoError(t, err)
		}

		// Test that parent only returns direct children (child1)
		children, err := headers.ByParentID(parent.ID())
		require.NoError(t, err)
		require.Len(t, children, 1)
		require.Equal(t, child.ToHeader(), children[0])

		// Test that child returns its direct children (grandchild1, grandchild2)
		grandchildren, err := headers.ByParentID(child.ID())
		require.NoError(t, err)
		require.ElementsMatch(t, grandchildren,
			[]*flow.Header{grandchild1.ToHeader(), grandchild2.ToHeader()})

		// Test that grandchildren have no children
		children, err = headers.ByParentID(grandchild1.ID())
		require.NoError(t, err)
		require.Empty(t, children)

		children, err = headers.ByParentID(grandchild2.ID())
		require.NoError(t, err)
		require.Empty(t, children)
	})
}

// TestHeadersStoreWrongChainID tests that attempting to store a block with a different chainID than
// expected returns the appropriate sentinel error.
func TestHeadersStoreWrongChainID(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		all, err := store.InitAll(metrics, db, flow.Emulator)
		require.NoError(t, err)
		blocks := all.Blocks
		// the underlying Headers has chainID flow.Emulator

		clusterChain := cluster.CanonicalClusterID(0, unittest.IdentifierListFixture(1))
		clusterChain2 := cluster.CanonicalClusterID(0, unittest.IdentifierListFixture(1))

		// A [flow.Proposal] with a mismatched chain should not be stored
		for _, invalidChainID := range []flow.ChainID{clusterChain, clusterChain2, flow.Localnet, flow.Testnet, flow.ChainID("invalid-chain")} {
			invalidBlock := unittest.BlockFixture()
			invalidBlock.ChainID = invalidChainID
			invalidProposal := unittest.ProposalFromBlock(invalidBlock)
			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return blocks.BatchStore(lctx, rw, invalidProposal)
				})
			})
			require.ErrorIs(t, err, storage.ErrWrongChain)
		}
	})
}

// TestHeadersRetrieveWrongChainID tests that methods of Headers throw an appropriate sentinel error
// when attempting to retrieve data that does not match the expected chain.
func TestHeadersRetrieveWrongChainID(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		all, err := store.InitAll(metrics, db, flow.Emulator)
		require.NoError(t, err)
		headers := all.Headers
		blocks := all.Blocks

		clusterChain := cluster.CanonicalClusterID(0, unittest.IdentifierListFixture(1))
		clusterChain2 := cluster.CanonicalClusterID(0, unittest.IdentifierListFixture(1))
		clusterHeaders, err := store.NewClusterHeaders(metrics, db, clusterChain)
		require.NoError(t, err)

		// Cluster Headers should not be able to retrieve a stored header for a different chain.
		// 1. store and index a block on main consensus chain
		proposal := unittest.ProposalFixture()
		block := proposal.Block
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, proposal)
			})
		})
		require.NoError(t, err)
		err = unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexFinalizedBlockByHeight(lctx, rw, block.Height, block.ID())
			})
		})
		require.NoError(t, err)

		// 2. check we can retrieve header by height and ID using the correct header storage instance
		actual, err := headers.ByHeight(block.Height)
		require.NoError(t, err)
		require.Equal(t, block.ToHeader(), actual)
		actual, err = headers.ByBlockID(block.ID())
		require.NoError(t, err)
		require.Equal(t, block.ToHeader(), actual)
		headerProp, err := headers.ProposalByBlockID(block.ID())
		require.NoError(t, err)
		require.Equal(t, proposal.ProposalHeader(), headerProp)

		// 3. clusterHeaders should not be able to retrieve that block by height or ID
		_, err = clusterHeaders.ByHeight(block.Height)
		require.ErrorIs(t, err, storage.ErrNotFound) // there are no finalized cluster blocks at any height
		_, err = clusterHeaders.ByBlockID(block.ID())
		require.ErrorIs(t, err, storage.ErrWrongChain)
		_, err = clusterHeaders.ProposalByBlockID(block.ID())
		require.ErrorIs(t, err, storage.ErrWrongChain)

		// 4. Store a block on a different cluster chain
		differentClusterBlock := unittest.ClusterBlockFixture()
		differentClusterBlock.ChainID = clusterChain2
		err = unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertClusterBlock(lctx, rw, unittest.ClusterProposalFromBlock(differentClusterBlock))
			})
		})
		require.NoError(t, err)

		// 5. clusterHeaders should not be able to retrieve it, as it is for a different cluster chain
		_, err = clusterHeaders.ByBlockID(differentClusterBlock.ID())
		require.ErrorIs(t, err, storage.ErrWrongChain)
		_, err = clusterHeaders.ProposalByBlockID(differentClusterBlock.ID())
		require.ErrorIs(t, err, storage.ErrWrongChain)

		// 6. main consensus chain Headers should also not be able to retrieve the cluster header
		_, err = headers.ByBlockID(differentClusterBlock.ID())
		require.ErrorIs(t, err, storage.ErrWrongChain)
		_, err = headers.ProposalByBlockID(differentClusterBlock.ID())
		require.ErrorIs(t, err, storage.ErrWrongChain)
	})
}
