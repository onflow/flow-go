package collections

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	collectionsmock "github.com/onflow/flow-go/engine/access/ingestion/collections/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	execdatamock "github.com/onflow/flow-go/module/executiondatasync/execution_data/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/signature"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

type SyncerSuite struct {
	suite.Suite

	state                 *protocolmock.State
	collections           *storagemock.Collections
	lastFullBlockHeightCP *storagemock.ConsumerProgress
	indexer               *collectionsmock.CollectionIndexer
	executionDataCache    *execdatamock.ExecutionDataCache

	requester           *modulemock.Requester
	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter
}

func TestSyncerSuite(t *testing.T) {
	suite.Run(t, new(SyncerSuite))
}

func (s *SyncerSuite) SetupTest() {
	s.state = protocolmock.NewState(s.T())
	s.collections = storagemock.NewCollections(s.T())
	s.lastFullBlockHeightCP = storagemock.NewConsumerProgress(s.T())
	s.indexer = collectionsmock.NewCollectionIndexer(s.T())
	s.requester = modulemock.NewRequester(s.T())
	s.executionDataCache = execdatamock.NewExecutionDataCache(s.T())

	var err error
	s.lastFullBlockHeightCP.On("ProcessedIndex").Return(uint64(100), nil).Once()
	s.lastFullBlockHeight, err = counters.NewPersistentStrictMonotonicCounter(s.lastFullBlockHeightCP)
	s.Require().NoError(err)
}

func (s *SyncerSuite) createSyncer() *Syncer {
	execDataSyncer := NewExecutionDataSyncer(
		s.executionDataCache,
		s.indexer,
	)

	return NewSyncer(
		unittest.Logger(),
		s.requester,
		s.state,
		s.collections,
		s.lastFullBlockHeight,
		s.indexer,
		execDataSyncer,
	)
}

// TestComponentLifecycle tests starting and stopping the syncer's worker loop within a component manager.
func (s *SyncerSuite) TestComponentLifecycle() {
	synctest.Test(s.T(), func(t *testing.T) {
		g := fixtures.NewGeneratorSuite()
		header := g.Headers().Fixture(fixtures.Header.WithHeight(s.lastFullBlockHeight.Value()))

		// finalized block is the same as last full block height, so no catchup is needed
		finalSnapshot := protocolmock.NewSnapshot(s.T())
		finalSnapshot.On("Head").Return(header, nil).Once()
		s.state.On("Final").Return(finalSnapshot, nil).Once()

		syncer := s.createSyncer()

		cm := component.NewComponentManagerBuilder().
			AddWorker(syncer.WorkerLoop).
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

// TestOnCollectionDownloaded tests that the syncer calls OnCollectionReceived on the indexer when
// it receives a collection downloaded notification.
func (s *SyncerSuite) TestOnCollectionDownloaded() {
	g := fixtures.NewGeneratorSuite()
	collection := g.Collections().Fixture()

	s.indexer.On("OnCollectionReceived", collection).Once()

	syncer := s.createSyncer()

	syncer.OnCollectionDownloaded(g.Identifiers().Fixture(), collection)
}

// TestRequestCollectionsForBlock tests calls to RequestCollectionsForBlock submit requests for the
// provided collections for heights > last full block height, and ignores requests for blocks that
// are <= last full block height.
func (s *SyncerSuite) TestRequestCollectionsForBlock() {
	g := fixtures.NewGeneratorSuite()

	guarantors := g.Identities().List(3, fixtures.Identity.WithRole(flow.RoleCollection))
	signerIndices, err := signature.EncodeSignersToIndices(guarantors.NodeIDs(), guarantors.NodeIDs())
	require.NoError(s.T(), err)

	guarantees := g.Guarantees().List(10, fixtures.Guarantee.WithSignerIndices(signerIndices))

	syncer := s.createSyncer()

	s.Run("request for block <= last full block height", func() {
		// EntityByID should not be called
		err := syncer.RequestCollectionsForBlock(s.lastFullBlockHeight.Value(), guarantees)
		s.Require().NoError(err)
	})

	s.Run("request for block above last full block height", func() {
		for _, guarantee := range guarantees {
			s.mockGuarantorsForCollection(guarantee, guarantors.ToSkeleton())
			s.requester.On("EntityByID", guarantee.CollectionID, mock.Anything).Once()
		}

		// called once for the batch
		s.requester.On("Force").Once()

		err := syncer.RequestCollectionsForBlock(s.lastFullBlockHeight.Value()+1, guarantees)
		s.Require().NoError(err)
	})

	s.Run("finding guarantors fails", func() {
		expectedError := errors.New("state lookup failed")
		s.mockGuarantorsForCollectionReturnsError(guarantees[0], expectedError)

		err := syncer.RequestCollectionsForBlock(s.lastFullBlockHeight.Value()+1, guarantees)
		s.Require().ErrorIs(err, expectedError)
	})
}

// TestWorkerLoop_InitialCatchup_CollectionNodesOnly tests the initial collection catchup when configured
// to only use collection nodes to retrieve missing collections.
func (s *SyncerSuite) TestWorkerLoop_InitialCatchup_CollectionNodesOnly() {
	g := fixtures.NewGeneratorSuite()

	guarantors := g.Identities().List(3, fixtures.Identity.WithRole(flow.RoleCollection))
	signerIndices, err := signature.EncodeSignersToIndices(guarantors.NodeIDs(), guarantors.NodeIDs())
	s.Require().NoError(err)

	finalizedHeight := s.lastFullBlockHeight.Value() + 10
	finalSnapshot := protocolmock.NewSnapshot(s.T())
	finalSnapshot.On("Head").Return(g.Headers().Fixture(fixtures.Header.WithHeight(finalizedHeight)), nil).Once()
	s.state.On("Final").Return(finalSnapshot, nil).Once()

	guaranteesByHeight := make(map[uint64][]*flow.CollectionGuarantee)
	for height := s.lastFullBlockHeight.Value() + 1; height <= finalizedHeight; height++ {
		guarantees := g.Guarantees().List(10, fixtures.Guarantee.WithSignerIndices(signerIndices))
		guaranteesByHeight[height] = guarantees

		s.indexer.On("MissingCollectionsAtHeight", height).Return(guarantees, nil).Once()

		for _, guarantee := range guarantees {
			s.mockGuarantorsForCollection(guarantee, guarantors.ToSkeleton())
			s.requester.On("EntityByID", guarantee.CollectionID, mock.Anything).Once()
		}
	}

	// called once for the batch
	s.requester.On("Force").Once()

	for _, guarantees := range guaranteesByHeight {
		for _, guarantee := range guarantees {
			// simulate the collection being missing by randomly returning false. All collections should
			// eventually be found in storage.
			s.indexer.On("IsCollectionInStorage", guarantee.CollectionID).Return(func(collectionID flow.Identifier) (bool, error) {
				return g.Random().Bool(), nil
			})
		}
	}

	syncer := NewSyncer(
		unittest.Logger(),
		s.requester,
		s.state,
		s.collections,
		s.lastFullBlockHeight,
		s.indexer,
		nil, // execution data indexing is disabled
	)

	synctest.Test(s.T(), func(t *testing.T) {
		ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(s.T(), context.Background())

		done := make(chan struct{})
		ready := make(chan struct{})

		go func() {
			defer close(done)
			syncer.WorkerLoop(ctx, func() { close(ready) })
		}()

		iterations := 0
	loop:
		for {
			// note: this sleep advances the synctest bubble's time, but does not actually block
			time.Sleep(syncer.collectionCatchupDBPollInterval + 1)

			synctest.Wait()
			select {
			case <-ready:
				break loop
			default:
			}

			// there are 100 collections, and each collection is marked as downloaded with a 50%
			// probability. Allow up to 100 iterations in pessimistic case, but it should complete
			// much faster.
			iterations++
			if iterations > 100 {
				t.Error("worker never completed initial catchup")
				break
			}
		}

		cancel()
		synctest.Wait()
		unittest.RequireClosed(t, done, "worker should be done")
	})
}

// TestWorkerLoop_InitialCatchup_SplitExecutionDataAndCollectionNodes tests the initial collection
// catchup when configured to use both execution data and collection nodes to retrieve missing collections.
func (s *SyncerSuite) TestWorkerLoop_InitialCatchup_SplitExecutionDataAndCollectionNodes() {
	g := fixtures.NewGeneratorSuite()

	guarantors := g.Identities().List(3, fixtures.Identity.WithRole(flow.RoleCollection))
	signerIndices, err := signature.EncodeSignersToIndices(guarantors.NodeIDs(), guarantors.NodeIDs())
	s.Require().NoError(err)

	finalizedHeight := s.lastFullBlockHeight.Value() + 10
	finalSnapshot := protocolmock.NewSnapshot(s.T())
	finalSnapshot.On("Head").Return(g.Headers().Fixture(fixtures.Header.WithHeight(finalizedHeight)), nil).Once()
	s.state.On("Final").Return(finalSnapshot, nil).Once()

	// simulate that execution data is only available for the first 5 blocks.
	maxExecutionDataHeight := s.lastFullBlockHeight.Value() + 5

	// the syncer should lookup execution data for each of these blocks, and submit the collections
	// to the indexer. It should stop after the first height for which execution data is not available
	// within the cache.
	for height := s.lastFullBlockHeight.Value() + 1; height <= maxExecutionDataHeight; height++ {
		execData, guarantees := executionDataFixture(g)

		s.indexer.On("MissingCollectionsAtHeight", height).Return(guarantees, nil).Once()
		s.executionDataCache.On("ByHeight", mock.Anything, height).Return(execData, nil).Once()

		for _, chunkData := range execData.ChunkExecutionDatas[:len(execData.ChunkExecutionDatas)-1] {
			s.indexer.On("OnCollectionReceived", chunkData.Collection).Once()
		}
	}

	// the syncer should request collections from collection nodes for all remaining heights.
	guaranteesByHeight := make(map[uint64][]*flow.CollectionGuarantee)
	for height := maxExecutionDataHeight + 1; height <= finalizedHeight; height++ {
		guarantees := g.Guarantees().List(10, fixtures.Guarantee.WithSignerIndices(signerIndices))
		guaranteesByHeight[height] = guarantees

		s.indexer.On("MissingCollectionsAtHeight", height).Return(guarantees, nil).Once()
		s.executionDataCache.On("ByHeight", mock.Anything, height).Return(nil, execution_data.NewBlobNotFoundError(cid.Cid{})).Once()

		for _, guarantee := range guarantees {
			s.mockGuarantorsForCollection(guarantee, guarantors.ToSkeleton())
			s.requester.On("EntityByID", guarantee.CollectionID, mock.Anything).Once()
		}
	}

	// called once for the batch
	s.requester.On("Force").Once()

	for _, guarantees := range guaranteesByHeight {
		for _, guarantee := range guarantees {
			// simulate the collection being missing by randomly returning false. All collections should
			// eventually be found in storage.
			s.indexer.On("IsCollectionInStorage", guarantee.CollectionID).Return(func(collectionID flow.Identifier) (bool, error) {
				return g.Random().Bool(), nil
			})
		}
	}

	syncer := s.createSyncer()

	synctest.Test(s.T(), func(t *testing.T) {
		ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(s.T(), context.Background())

		done := make(chan struct{})
		ready := make(chan struct{})

		go func() {
			defer close(done)
			syncer.WorkerLoop(ctx, func() { close(ready) })
		}()

		iterations := 0
	loop:
		for {
			// note: this sleep advances the synctest bubble's time, but does not actually block
			time.Sleep(syncer.collectionCatchupDBPollInterval + 1)

			synctest.Wait()
			select {
			case <-ready:
				break loop
			default:
			}

			// there are 100 collections, and each collection is marked as downloaded with a 50%
			// probability. Allow up to 100 iterations in pessimistic case, but it should complete
			// much faster.
			iterations++
			if iterations > 100 {
				t.Error("worker never completed initial catchup")
				break
			}
		}

		cancel()
		synctest.Wait()
		unittest.RequireClosed(t, done, "worker should be done")
	})
}

// TestWorkerLoop_InitialCatchup_AllAvailableFromExecutionData tests the initial collection catchup
// when configured to use execution data and data is available for all blocks.
func (s *SyncerSuite) TestWorkerLoop_InitialCatchup_AllAvailableFromExecutionData() {
	g := fixtures.NewGeneratorSuite()

	finalizedHeight := s.lastFullBlockHeight.Value() + 10
	finalSnapshot := protocolmock.NewSnapshot(s.T())
	finalSnapshot.On("Head").Return(g.Headers().Fixture(fixtures.Header.WithHeight(finalizedHeight)), nil).Once()
	s.state.On("Final").Return(finalSnapshot, nil).Once()

	// simulate that execution data is available for all blocks.
	maxExecutionDataHeight := finalizedHeight

	// the syncer should lookup execution data for each of these blocks, and submit the collections
	// to the indexer.
	for height := s.lastFullBlockHeight.Value() + 1; height <= maxExecutionDataHeight; height++ {
		execData, guarantees := executionDataFixture(g)

		s.indexer.On("MissingCollectionsAtHeight", height).Return(guarantees, nil).Once()
		s.executionDataCache.On("ByHeight", mock.Anything, height).Return(execData, nil).Once()

		for _, chunkData := range execData.ChunkExecutionDatas[:len(execData.ChunkExecutionDatas)-1] {
			s.indexer.On("OnCollectionReceived", chunkData.Collection).Once()
		}
	}

	syncer := s.createSyncer()

	synctest.Test(s.T(), func(t *testing.T) {
		ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(s.T(), context.Background())

		done := make(chan struct{})
		ready := make(chan struct{})

		go func() {
			defer close(done)
			syncer.WorkerLoop(ctx, func() { close(ready) })
		}()

		synctest.Wait()
		unittest.RequireClosed(t, ready, "worker should be ready")

		cancel()
		synctest.Wait()
		unittest.RequireClosed(t, done, "worker should be done")
	})
}

// TestWorkerLoop_InitialCatchup_Timesout tests the initial collection catchup times out before completing,
// and gracefully continues with normal startup.
func (s *SyncerSuite) TestWorkerLoop_InitialCatchup_Timesout() {
	g := fixtures.NewGeneratorSuite()

	synctest.Test(s.T(), func(t *testing.T) {
		finalizedHeight := s.lastFullBlockHeight.Value() + 10
		finalizedHeader := g.Headers().Fixture(fixtures.Header.WithHeight(finalizedHeight))
		finalSnapshot := protocolmock.NewSnapshot(s.T())
		finalSnapshot.On("Head").Return(finalizedHeader, nil).Once()
		s.state.On("Final").Return(finalSnapshot).Once()

		// block the first call to MissingCollectionsAtHeight that is called during startup until after
		// the timeout. This simulates the catchup logic taking too long.
		unblockStartup := make(chan struct{})
		s.indexer.
			On("MissingCollectionsAtHeight", s.lastFullBlockHeight.Value()+1).
			Return(func(uint64) ([]*flow.CollectionGuarantee, error) {
				// note: this sleep advances the synctest bubble's time, but does not actually block
				time.Sleep(DefaultCollectionCatchupTimeout + time.Second)
				close(unblockStartup)
				return nil, nil
			}).Once()

		syncer := NewSyncer(
			unittest.Logger(),
			s.requester,
			s.state,
			s.collections,
			s.lastFullBlockHeight,
			s.indexer,
			nil, // execution data indexing is disabled
		)

		ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(s.T(), context.Background())

		done := make(chan struct{})
		ready := make(chan struct{})

		go func() {
			defer close(done)
			syncer.WorkerLoop(ctx, func() { close(ready) })
		}()

		// this is only closed after the timeout, so ready should be closed
		<-unblockStartup
		unittest.RequireClosed(t, ready, "worker should be ready")

		cancel()
		synctest.Wait()
		unittest.RequireClosed(t, done, "worker should be done")
	})
}

func (s *SyncerSuite) TestWorkerLoop_RequestsMissingCollections() {
	g := fixtures.NewGeneratorSuite()

	guarantors := g.Identities().List(3, fixtures.Identity.WithRole(flow.RoleCollection))
	signerIndices, err := signature.EncodeSignersToIndices(guarantors.NodeIDs(), guarantors.NodeIDs())
	s.Require().NoError(err)

	lastFullBlockHeight := s.lastFullBlockHeight.Value()
	finalizedHeight := lastFullBlockHeight + 10
	finalizedHeader := g.Headers().Fixture(fixtures.Header.WithHeight(finalizedHeight))
	allGuarantees := make(map[uint64][]*flow.CollectionGuarantee, 0)
	for height := lastFullBlockHeight + 1; height <= finalizedHeight; height++ {
		allGuarantees[height] = g.Guarantees().List(3, fixtures.Guarantee.WithSignerIndices(signerIndices))
	}

	s.Run("no missing collections", func() {
		s.runWorkerLoopMissingCollections(g, nil, func(syncer *Syncer) {
			finalSnapshot := protocolmock.NewSnapshot(s.T())
			finalSnapshot.On("Head").Return(finalizedHeader, nil).Once()
			s.state.On("Final").Return(finalSnapshot).Once()

			for height := lastFullBlockHeight + 1; height <= finalizedHeight; height++ {
				s.indexer.On("MissingCollectionsAtHeight", height).Return(nil, nil).Once()
			}
		})
	})

	s.Run("missing collections - request skipped below thresholds", func() {
		s.runWorkerLoopMissingCollections(g, nil, func(syncer *Syncer) {
			finalSnapshot := protocolmock.NewSnapshot(s.T())
			finalSnapshot.On("Head").Return(finalizedHeader, nil).Once()
			s.state.On("Final").Return(finalSnapshot).Once()

			for height := lastFullBlockHeight + 1; height <= finalizedHeight; height++ {
				s.indexer.On("MissingCollectionsAtHeight", height).Return(allGuarantees[height], nil).Once()
			}
		})
	})

	s.Run("missing collections - request sent when count exceeds missingCollsForBlockThreshold", func() {
		s.runWorkerLoopMissingCollections(g, nil, func(syncer *Syncer) {
			syncer.missingCollsForBlockThreshold = 9

			finalSnapshot := protocolmock.NewSnapshot(s.T())
			finalSnapshot.On("Head").Return(finalizedHeader, nil).Once()
			s.state.On("Final").Return(finalSnapshot).Once()

			for height := lastFullBlockHeight + 1; height <= finalizedHeight; height++ {
				s.indexer.On("MissingCollectionsAtHeight", height).Return(allGuarantees[height], nil).Once()
				for _, guarantee := range allGuarantees[height] {
					s.requester.On("EntityByID", guarantee.CollectionID, mock.Anything).Once()
					s.mockGuarantorsForCollection(guarantee, guarantors.ToSkeleton())
				}
			}
		})
	})

	s.Run("missing collections - request sent when age exceeds missingCollsForAgeThreshold", func() {
		s.runWorkerLoopMissingCollections(g, nil, func(syncer *Syncer) {
			syncer.missingCollsForAgeThreshold = 9

			finalSnapshot := protocolmock.NewSnapshot(s.T())
			finalSnapshot.On("Head").Return(finalizedHeader, nil).Once()
			s.state.On("Final").Return(finalSnapshot).Once()

			for height := s.lastFullBlockHeight.Value() + 1; height <= finalizedHeight; height++ {
				s.indexer.On("MissingCollectionsAtHeight", height).Return(allGuarantees[height], nil).Once()
			}

			for height := s.lastFullBlockHeight.Value() + 1; height <= finalizedHeight; height++ {
				for _, guarantee := range allGuarantees[height] {
					s.requester.On("EntityByID", guarantee.CollectionID, mock.Anything).Once()
					s.mockGuarantorsForCollection(guarantee, guarantors.ToSkeleton())
				}
			}
		})
	})

	s.Run("missing collections - processed from execution data", func() {
		execDataSyncer := NewExecutionDataSyncer(
			s.executionDataCache,
			s.indexer,
		)

		s.runWorkerLoopMissingCollections(g, execDataSyncer, func(syncer *Syncer) {
			finalSnapshot := protocolmock.NewSnapshot(s.T())
			finalSnapshot.On("Head").Return(finalizedHeader, nil).Once()
			s.state.On("Final").Return(finalSnapshot).Once()

			for height := lastFullBlockHeight + 1; height <= finalizedHeight; height++ {
				execData, _ := executionDataFixture(g)
				s.executionDataCache.On("ByHeight", mock.Anything, height).Return(execData, nil).Once()
				for _, chunkData := range execData.ChunkExecutionDatas[:len(execData.ChunkExecutionDatas)-1] {
					s.indexer.On("OnCollectionReceived", chunkData.Collection).Once()
				}
			}
			// syncer continues until it receives a not found error.
			s.executionDataCache.On("ByHeight", mock.Anything, finalizedHeight+1).Return(nil, execution_data.NewBlobNotFoundError(cid.Cid{})).Once()
		})
	})
}

func (s *SyncerSuite) runWorkerLoopMissingCollections(g *fixtures.GeneratorSuite, execDataSyncer *ExecutionDataSyncer, onReady func(*Syncer)) {
	synctest.Test(s.T(), func(t *testing.T) {
		// last full block is latest finalized block, so initial catchup is skipped
		finalizedHeight := s.lastFullBlockHeight.Value()
		finalizedHeader := g.Headers().Fixture(fixtures.Header.WithHeight(finalizedHeight))
		finalSnapshot := protocolmock.NewSnapshot(s.T())
		finalSnapshot.On("Head").Return(finalizedHeader, nil).Once()
		s.state.On("Final").Return(finalSnapshot).Once()

		syncer := NewSyncer(
			unittest.Logger(),
			s.requester,
			s.state,
			s.collections,
			s.lastFullBlockHeight,
			s.indexer,
			execDataSyncer,
		)

		ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(s.T(), context.Background())

		done := make(chan struct{})
		ready := make(chan struct{})

		go func() {
			defer close(done)
			syncer.WorkerLoop(ctx, func() {
				onReady(syncer)
				close(ready)
			})
		}()

		<-ready

		time.Sleep(syncer.missingCollsRequestInterval + 1)
		synctest.Wait()

		cancel()
		synctest.Wait()
		unittest.RequireClosed(t, done, "worker should be done")
	})
}

func executionDataFixture(g *fixtures.GeneratorSuite) (*execution_data.BlockExecutionDataEntity, []*flow.CollectionGuarantee) {
	chunkDatas := g.ChunkExecutionDatas().List(10)
	execData := g.BlockExecutionDataEntities().Fixture(fixtures.BlockExecutionData.WithChunkExecutionDatas(chunkDatas...))
	guarantees := make([]*flow.CollectionGuarantee, len(chunkDatas)-1)
	for i := range len(chunkDatas) - 1 {
		guarantees[i] = g.Guarantees().Fixture(fixtures.Guarantee.WithCollectionID(chunkDatas[i].Collection.ID()))
	}
	return execData, guarantees
}

func (s *SyncerSuite) mockGuarantorsForCollection(guarantee *flow.CollectionGuarantee, members flow.IdentitySkeletonList) {
	cluster := protocolmock.NewCluster(s.T())
	cluster.On("Members").Return(members, nil).Once()

	epoch := protocolmock.NewCommittedEpoch(s.T())
	epoch.On("ClusterByChainID", guarantee.ClusterChainID).Return(cluster, nil).Once()

	query := protocolmock.NewEpochQuery(s.T())
	query.On("Current").Return(epoch, nil).Once()

	snapshot := protocolmock.NewSnapshot(s.T())
	snapshot.On("Epochs").Return(query).Once()

	s.state.On("AtBlockID", guarantee.ReferenceBlockID).Return(snapshot).Once()
}

func (s *SyncerSuite) mockGuarantorsForCollectionReturnsError(guarantee *flow.CollectionGuarantee, expectedError error) {
	query := protocolmock.NewEpochQuery(s.T())
	query.On("Current").Return(nil, expectedError).Once()

	snapshot := protocolmock.NewSnapshot(s.T())
	snapshot.On("Epochs").Return(query).Once()

	s.state.On("AtBlockID", guarantee.ReferenceBlockID).Return(snapshot).Once()
}
