package collections

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

type IndexerSuite struct {
	suite.Suite

	state               *protocolmock.State
	blocks              *storagemock.Blocks
	collections         *storagemock.Collections
	transactions        *storagemock.Transactions
	lastFullBlockCP     *storagemock.ConsumerProgress
	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter
	lockManager         lockctx.Manager

	height uint64
}

func TestIndexer(t *testing.T) {
	suite.Run(t, new(IndexerSuite))
}

func (s *IndexerSuite) SetupTest() {
	s.state = protocolmock.NewState(s.T())
	s.blocks = storagemock.NewBlocks(s.T())
	s.collections = storagemock.NewCollections(s.T())
	s.transactions = storagemock.NewTransactions(s.T())
	s.lastFullBlockCP = storagemock.NewConsumerProgress(s.T())
	s.lockManager = storage.NewTestingLockManager()

	s.height = 100

	var err error
	s.lastFullBlockCP.On("ProcessedIndex").Return(s.height-1, nil)
	s.lastFullBlockHeight, err = counters.NewPersistentStrictMonotonicCounter(s.lastFullBlockCP)
	require.NoError(s.T(), err)
}

func (s *IndexerSuite) createIndexer(t *testing.T) *Indexer {
	indexer, err := NewIndexer(
		unittest.Logger(),
		metrics.NewNoopCollector(),
		s.state,
		s.blocks,
		s.collections,
		s.lastFullBlockHeight,
		s.lockManager,
	)
	require.NoError(t, err)
	return indexer
}

func (s *IndexerSuite) fixture(g *fixtures.GeneratorSuite) (*flow.Block, []*flow.Collection) {
	parentHeader := g.Headers().Fixture(fixtures.Header.WithHeight(s.height))
	return s.fixtureWithParent(g, parentHeader)
}

func (s *IndexerSuite) fixtureWithParent(g *fixtures.GeneratorSuite, parentHeader *flow.Header) (*flow.Block, []*flow.Collection) {
	collections := g.Collections().List(3)
	guarantees := make([]*flow.CollectionGuarantee, len(collections))
	for i, collection := range collections {
		guarantee := g.Guarantees().Fixture(fixtures.Guarantee.WithCollectionID(collection.ID()))
		guarantees[i] = guarantee
	}
	payload := g.Payloads().Fixture(
		fixtures.Payload.WithGuarantees(guarantees...),
		fixtures.Payload.WithReceiptStubs(),
		fixtures.Payload.WithResults(),
		fixtures.Payload.WithSeals(),
	)
	block := g.Blocks().Fixture(
		fixtures.Block.WithParentHeader(parentHeader),
		fixtures.Block.WithPayload(payload),
	)
	return block, collections
}

func (s *IndexerSuite) TestWorkerProcessing_ComponentLifecycle() {
	synctest.Test(s.T(), func(t *testing.T) {
		indexer := s.createIndexer(s.T())

		cm := component.NewComponentManagerBuilder().
			AddWorker(indexer.WorkerLoop).
			Build()

		signalerCtx, cancel := irrecoverable.NewMockSignalerContextWithCancel(s.T(), context.Background())
		defer cancel()

		cm.Start(signalerCtx)

		synctest.Wait()
		unittest.RequireClosed(t, cm.Ready(), "worker should be ready")

		cancel()
		synctest.Wait()
		unittest.RequireClosed(t, cm.Done(), "worker should be done")
	})
}

func (s *IndexerSuite) TestMissingCollectionsAtHeight_HappyPath() {
	g := fixtures.NewGeneratorSuite()
	block, collectionList := s.fixture(g)

	s.blocks.On("ByHeight", block.Height).Return(block, nil)

	s.collections.On("LightByID", collectionList[0].ID()).Return(collectionList[0].Light(), nil).Once()
	s.collections.On("LightByID", mock.Anything).Return(nil, storage.ErrNotFound).Twice()
	// the other 2 collections are missing

	indexer := s.createIndexer(s.T())

	missingCollections, err := indexer.MissingCollectionsAtHeight(block.Height)
	s.Require().NoError(err)

	s.Equal(len(missingCollections), 2)
	s.Equal(collectionList[1].ID(), missingCollections[0].CollectionID)
	s.Equal(collectionList[2].ID(), missingCollections[1].CollectionID)
}

func (s *IndexerSuite) TestMissingCollectionsAtHeight_ErrorCases() {
	g := fixtures.NewGeneratorSuite()
	block, _ := s.fixture(g)

	s.Run("block error", func() {
		height := s.height + 10
		s.blocks.On("ByHeight", height).Return(nil, storage.ErrNotFound).Once()

		indexer := s.createIndexer(s.T())

		missingCollections, err := indexer.MissingCollectionsAtHeight(height)
		s.Require().ErrorIs(err, storage.ErrNotFound)
		s.Require().Empty(missingCollections)
	})

	s.Run("collection error", func() {
		expectedErr := errors.New("expected error")

		s.blocks.On("ByHeight", block.Height).Return(block, nil).Once()
		s.collections.On("LightByID", block.Payload.Guarantees[0].CollectionID).Return(nil, expectedErr).Once()

		indexer := s.createIndexer(s.T())

		missingCollections, err := indexer.MissingCollectionsAtHeight(block.Height)
		s.Require().ErrorIs(err, expectedErr)
		s.Require().Empty(missingCollections)
	})
}

func (s *IndexerSuite) TestIsCollectionInStorage() {
	g := fixtures.NewGeneratorSuite()
	collection := g.Collections().Fixture()

	s.Run("happy path", func() {
		s.collections.On("LightByID", collection.ID()).Return(collection.Light(), nil).Once()

		indexer := s.createIndexer(s.T())

		inStorage, err := indexer.IsCollectionInStorage(collection.ID())
		s.Require().NoError(err)
		s.Require().True(inStorage)
	})

	s.Run("not in storage", func() {
		s.collections.On("LightByID", collection.ID()).Return(nil, storage.ErrNotFound).Once()

		indexer := s.createIndexer(s.T())

		inStorage, err := indexer.IsCollectionInStorage(collection.ID())
		s.Require().NoError(err)
		s.Require().False(inStorage)
	})

	s.Run("unexpected error", func() {
		expectedErr := errors.New("unexpected error")
		s.collections.On("LightByID", collection.ID()).Return(nil, expectedErr).Once()

		indexer := s.createIndexer(s.T())

		inStorage, err := indexer.IsCollectionInStorage(collection.ID())
		s.Require().ErrorIs(err, expectedErr)
		s.Require().False(inStorage)
	})
}

func (s *IndexerSuite) TestUpdateLastFullBlockHeight() {
	g := fixtures.NewGeneratorSuite()

	s.Run("no new complete blocks", func() {
		indexer := s.createIndexer(s.T())
		original := indexer.lastFullBlockHeight.Value()

		// last full block height is the finalized height
		finalizedBlock := g.Blocks().Fixture(fixtures.Block.WithHeight(original))
		finalSnapshot := protocolmock.NewSnapshot(s.T())
		finalSnapshot.On("Head").Return(finalizedBlock.ToHeader(), nil).Once()
		s.state.On("Final").Return(finalSnapshot).Once()

		err := indexer.updateLastFullBlockHeight()
		s.Require().NoError(err)

		updated := indexer.lastFullBlockHeight.Value()
		s.Equal(original, updated)
	})

	s.Run("updates last full block height and stops updating at finalized height", func() {
		indexer := s.createIndexer(s.T())

		lastFullBlockHeight := indexer.lastFullBlockHeight.Value()
		finalizedHeight := lastFullBlockHeight + 3

		// last full block height is the finalized height
		finalizedBlock := g.Blocks().Fixture(fixtures.Block.WithHeight(finalizedHeight))
		finalSnapshot := protocolmock.NewSnapshot(s.T())
		finalSnapshot.On("Head").Return(finalizedBlock.ToHeader(), nil)
		s.state.On("Final").Return(finalSnapshot).Once()

		// Note: it should not check any heights above the finalized height
		for height := lastFullBlockHeight + 1; height <= finalizedHeight; height++ {
			block := g.Blocks().Fixture(fixtures.Block.WithHeight(height))
			s.blocks.On("ByHeight", height).Return(block, nil).Once()
			for _, guarantee := range block.Payload.Guarantees {
				collection := g.Collections().Fixture()
				s.collections.On("LightByID", guarantee.CollectionID).Return(collection.Light(), nil).Once()
			}
		}
		s.lastFullBlockCP.On("SetProcessedIndex", finalizedBlock.Height).Return(nil).Once()

		err := indexer.updateLastFullBlockHeight()
		s.Require().NoError(err)

		updated := indexer.lastFullBlockHeight.Value()
		s.Equal(finalizedBlock.Height, updated)
	})
}

func (s *IndexerSuite) TestOnCollectionReceived() {
	g := fixtures.NewGeneratorSuite()

	collection := g.Collections().Fixture()

	synctest.Test(s.T(), func(t *testing.T) {
		s.collections.On("LightByID", collection.ID()).Return(nil, storage.ErrNotFound).Once()
		s.collections.On("StoreAndIndexByTransaction", mock.Anything, collection).Return(collection.Light(), nil).Once()

		indexer := s.createIndexer(s.T())

		ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(s.T(), context.Background())
		defer cancel()

		ready := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			indexer.WorkerLoop(ctx, func() { close(ready) })
		}()

		synctest.Wait()
		unittest.RequireClosed(t, ready, "worker should be ready")

		// calling OnCollectionReceived stores and indexes the collection when the collection is not in storage.
		indexer.OnCollectionReceived(collection)
		synctest.Wait()

		// calling OnCollectionReceived does skips indexing when the collection is already in storage.
		s.collections.On("LightByID", collection.ID()).Unset()
		s.collections.On("LightByID", collection.ID()).Return(collection.Light(), nil).Once()

		indexer.OnCollectionReceived(collection)
		synctest.Wait()
	})
}

func (s *IndexerSuite) TestWorkerProcessing_ProcessesCollections() {
	g := fixtures.NewGeneratorSuite()
	rootBlock := g.Blocks().Genesis()

	RunWithBlockchain(s.T(), func(bc *blockchain) {
		initializer := store.NewConsumerProgress(bc.db, module.ConsumeProgressLastFullBlockHeight)
		progress, err := initializer.Initialize(rootBlock.Height)
		s.Require().NoError(err)

		lastFullBlockHeight, err := counters.NewPersistentStrictMonotonicCounter(progress)
		s.Require().NoError(err)

		indexer, err := NewIndexer(
			unittest.Logger(),
			metrics.NewNoopCollector(),
			bc.state,
			bc.all.Blocks,
			bc.collections,
			lastFullBlockHeight,
			bc.lockManager,
		)
		s.Require().NoError(err)

		cm := component.NewComponentManagerBuilder().
			AddWorker(indexer.WorkerLoop).
			Build()

		signalerCtx, cancel := irrecoverable.NewMockSignalerContextWithCancel(s.T(), context.Background())
		defer cancel()

		cm.Start(signalerCtx)
		unittest.RequireCloseBefore(s.T(), cm.Ready(), 100*time.Millisecond, "could not start worker")
		defer func() {
			cancel()
			unittest.RequireCloseBefore(s.T(), cm.Done(), 100*time.Millisecond, "could not stop worker")
		}()

		prevHeader := rootBlock.ToHeader()
		for range 10 {
			block, collectionList := s.fixtureWithParent(g, prevHeader)
			bc.finalizeBlock(s.T(), block)
			prevHeader = block.ToHeader()

			for _, collection := range collectionList {
				indexer.OnCollectionReceived(collection)
			}

			// wait until all collections are indexed
			require.Eventually(s.T(), func() bool {
				missingCollections, err := indexer.MissingCollectionsAtHeight(block.Height)
				return err == nil && len(missingCollections) == 0
			}, time.Second, 10*time.Millisecond)

			// check that all collection indices are populated
			// ByID uses all of the indices.
			for _, collection := range collectionList {
				actual, err := bc.collections.ByID(collection.ID())
				s.Require().NoError(err)
				s.Require().Equal(collection, actual)
			}
		}

		// wait until the last full block height is updated
		require.Eventually(s.T(), func() bool {
			return lastFullBlockHeight.Value() == prevHeader.Height
		}, time.Second, 100*time.Millisecond)
	})
}

type blockchain struct {
	db          storage.DB
	lockManager lockctx.Manager

	all          *store.All
	transactions storage.Transactions
	collections  storage.Collections

	state *protocolmock.State
}

func newBlockchain(t *testing.T, pdb *pebble.DB) *blockchain {
	metrics := metrics.NewNoopCollector()

	db := pebbleimpl.ToDB(pdb)
	lockManager := storage.NewTestingLockManager()

	all := store.InitAll(metrics, db)
	transactions := store.NewTransactions(metrics, db)
	collections := store.NewCollections(db, transactions)

	return &blockchain{
		db:           db,
		lockManager:  lockManager,
		all:          all,
		transactions: transactions,
		collections:  collections,
		state:        protocolmock.NewState(t),
	}
}

func (bc *blockchain) finalizeBlock(t *testing.T, block *flow.Block) {
	// Add the target block as finalized.
	err := unittest.WithLocks(t, bc.lockManager, []string{
		storage.LockInsertBlock,
		storage.LockFinalizeBlock,
	}, func(lctx lockctx.Context) error {
		return bc.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			proposal := unittest.ProposalFromBlock(block)
			if err := bc.all.Blocks.BatchStore(lctx, rw, proposal); err != nil {
				return fmt.Errorf("could not store block: %w", err)
			}
			if err := operation.IndexFinalizedBlockByHeight(lctx, rw, block.Height, block.ID()); err != nil {
				return fmt.Errorf("could not index block by height: %w", err)
			}
			return nil
		})
	})
	require.NoError(t, err)

	// final snapshot may not be queried for every block.
	finalSnapshot := protocolmock.NewSnapshot(t)
	finalSnapshot.On("Head").Return(block.ToHeader(), nil).Maybe()
	bc.state.On("Final").Unset()
	bc.state.On("Final").Return(finalSnapshot)
}

func RunWithBlockchain(t *testing.T, fn func(bc *blockchain)) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		bc := newBlockchain(t, pdb)
		fn(bc)
	})
}
