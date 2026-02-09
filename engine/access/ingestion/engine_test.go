package ingestion

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	hotmodel "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/engine/access/ingestion/collections"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
	downloadermock "github.com/onflow/flow-go/module/executiondatasync/execution_data/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/network/channels"
	mocknetwork "github.com/onflow/flow-go/network/mock"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

type Suite struct {
	suite.Suite

	// protocol state
	proto struct {
		state    *protocolmock.FollowerState
		snapshot *protocolmock.Snapshot
		params   *protocolmock.Params
	}

	me           *modulemock.Local
	net          *mocknetwork.EngineRegistry
	request      *modulemock.Requester
	obsIdentity  *flow.Identity
	provider     *mocknetwork.Engine
	blocks       *storagemock.Blocks
	headers      *storagemock.Headers
	collections  *storagemock.Collections
	transactions *storagemock.Transactions
	receipts     *storagemock.ExecutionReceipts
	results      *storagemock.ExecutionResults
	seals        *storagemock.Seals

	conduit        *mocknetwork.Conduit
	downloader     *downloadermock.Downloader
	sealedBlock    *flow.Header
	finalizedBlock *flow.Header
	log            zerolog.Logger
	blockMap       map[uint64]*flow.Block
	distributor    *pubsub.FollowerDistributor
	rootBlock      *flow.Block

	collectionExecutedMetric *indexer.CollectionExecutedMetricImpl

	ctx    context.Context
	cancel context.CancelFunc

	db                  storage.DB
	dbDir               string
	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter
	lockManager         lockctx.Manager
}

func TestIngestEngine(t *testing.T) {
	suite.Run(t, new(Suite))
}

// TearDownTest stops the engine and cleans up the db
func (s *Suite) TearDownTest() {
	s.cancel()
	err := os.RemoveAll(s.dbDir)
	s.Require().NoError(err)
}

func (s *Suite) SetupTest() {
	s.log = unittest.Logger()
	s.ctx, s.cancel = context.WithCancel(context.Background())
	db, dbDir := unittest.TempPebbleDB(s.T())
	s.db = pebbleimpl.ToDB(db)
	s.dbDir = dbDir
	s.lockManager = storage.NewTestingLockManager()

	s.obsIdentity = unittest.IdentityFixture(unittest.WithRole(flow.RoleAccess))

	s.blocks = storagemock.NewBlocks(s.T())
	// mock out protocol state
	s.proto.state = new(protocolmock.FollowerState)
	s.proto.snapshot = new(protocolmock.Snapshot)
	s.proto.params = new(protocolmock.Params)
	s.finalizedBlock = unittest.BlockHeaderFixture(unittest.WithHeaderHeight(0))
	s.proto.state.On("Identity").Return(s.obsIdentity, nil)
	s.proto.state.On("Params").Return(s.proto.params)

	s.me = modulemock.NewLocal(s.T())
	s.me.On("NodeID").Return(s.obsIdentity.NodeID).Maybe()
	s.net = mocknetwork.NewEngineRegistry(s.T())
	conduit := mocknetwork.NewConduit(s.T())
	s.net.On("Register", channels.ReceiveReceipts, mock.Anything).
		Return(conduit, nil).
		Once()
	s.request = modulemock.NewRequester(s.T())
	s.provider = mocknetwork.NewEngine(s.T())
	s.blocks = storagemock.NewBlocks(s.T())
	s.headers = storagemock.NewHeaders(s.T())
	s.collections = new(storagemock.Collections)
	s.receipts = new(storagemock.ExecutionReceipts)
	s.transactions = new(storagemock.Transactions)
	s.results = new(storagemock.ExecutionResults)
	s.results.On("BatchIndex", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	collectionsToMarkFinalized := stdmap.NewTimes(100)
	collectionsToMarkExecuted := stdmap.NewTimes(100)
	blocksToMarkExecuted := stdmap.NewTimes(100)
	blockTransactions := stdmap.NewIdentifierMap(100)

	s.proto.state.On("Identity").Return(s.obsIdentity, nil)
	s.proto.state.On("Params").Return(s.proto.params)

	blockCount := 5
	s.blockMap = make(map[uint64]*flow.Block, blockCount)
	s.rootBlock = unittest.Block.Genesis(flow.Emulator)
	parent := s.rootBlock.ToHeader()

	for range blockCount {
		block := unittest.BlockWithParentFixture(parent)
		s.blockMap[block.Height] = block
		// update for next iteration
		parent = block.ToHeader()
	}
	s.finalizedBlock = parent

	s.blocks.On("ByHeight", mock.AnythingOfType("uint64")).Return(
		mocks.ConvertStorageOutput(
			mocks.StorageMapGetter(s.blockMap),
			func(block *flow.Block) *flow.Block { return block },
		),
	).Maybe()

	// Mock the finalized root block header with height 0.
	header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(0))
	s.proto.params.On("FinalizedRoot").Return(header, nil)

	var err error
	s.collectionExecutedMetric, err = indexer.NewCollectionExecutedMetricImpl(
		s.log,
		metrics.NewNoopCollector(),
		collectionsToMarkFinalized,
		collectionsToMarkExecuted,
		blocksToMarkExecuted,
		s.collections,
		s.blocks,
		blockTransactions,
	)
	require.NoError(s.T(), err)
}

// initEngineAndSyncer create new instance of ingestion engine and collection syncer.
// It waits until the ingestion engine starts.
func (s *Suite) initEngineAndSyncer() (*Engine, *collections.Syncer, *collections.Indexer) {
	processedHeight, err := store.NewConsumerProgress(s.db, module.ConsumeProgressIngestionEngineBlockHeight).Initialize(s.finalizedBlock.Height)
	require.NoError(s.T(), err)
	lastFullBlockHeight, err := store.NewConsumerProgress(s.db, module.ConsumeProgressLastFullBlockHeight).Initialize(s.finalizedBlock.Height)
	require.NoError(s.T(), err)

	s.lastFullBlockHeight, err = counters.NewPersistentStrictMonotonicCounter(lastFullBlockHeight)
	require.NoError(s.T(), err)

	indexer, err := collections.NewIndexer(
		s.log,
		s.db,
		s.collectionExecutedMetric,
		s.proto.state,
		s.blocks,
		s.collections,
		s.lastFullBlockHeight,
		s.lockManager,
	)
	require.NoError(s.T(), err)

	syncer := collections.NewSyncer(
		s.log,
		s.request,
		s.proto.state,
		s.collections,
		s.lastFullBlockHeight,
		indexer,
		nil,
	)

	s.distributor = pubsub.NewFollowerDistributor()
	eng, err := New(
		s.log,
		s.net,
		s.proto.state,
		s.me,
		s.lockManager,
		s.db,
		s.blocks,
		s.results,
		s.receipts,
		processedHeight,
		syncer,
		indexer,
		s.collectionExecutedMetric,
		metrics.NewNoopCollector(),
		nil,
		s.distributor,
	)
	require.NoError(s.T(), err)

	return eng, syncer, indexer
}

// mockCollectionsForBlock mocks collections for block
func (s *Suite) mockCollectionsForBlock(block *flow.Block) {
	// we should query the block once and index the guarantee payload once
	for _, g := range block.Payload.Guarantees {
		collection := unittest.CollectionFixture(1)
		light := collection.Light()
		s.collections.On("LightByID", g.CollectionID).Return(light, nil).Twice()
	}
}

// generateBlock prepares block with payload and specified guarantee.SignerIndices
func (s *Suite) generateBlock(clusterCommittee flow.IdentitySkeletonList, snap *protocolmock.Snapshot) *flow.Block {
	block := unittest.BlockFixture(
		unittest.Block.WithPayload(unittest.PayloadFixture(
			unittest.WithGuarantees(unittest.CollectionGuaranteesFixture(4)...),
			unittest.WithExecutionResults(unittest.ExecutionResultFixture()),
			unittest.WithSeals(unittest.Seal.Fixture()),
		)),
	)

	refBlockID := unittest.IdentifierFixture()
	for _, guarantee := range block.Payload.Guarantees {
		guarantee.ReferenceBlockID = refBlockID
		// guarantee signers must be cluster committee members, so that access will fetch collection from
		// the signers that are specified by guarantee.SignerIndices
		indices, err := signature.EncodeSignersToIndices(clusterCommittee.NodeIDs(), clusterCommittee.NodeIDs())
		require.NoError(s.T(), err)
		guarantee.SignerIndices = indices
	}

	s.proto.state.On("AtBlockID", refBlockID).Return(snap)

	return block
}

// TestOnFinalizedBlock checks that when a block is received, a request for each individual collection is made
func (s *Suite) TestOnFinalizedBlockSingle() {
	cluster := new(protocolmock.Cluster)
	epoch := new(protocolmock.CommittedEpoch)
	epochs := new(protocolmock.EpochQuery)
	snap := new(protocolmock.Snapshot)

	finalSnapshot := protocolmock.NewSnapshot(s.T())
	finalSnapshot.On("Head").Return(s.finalizedBlock, nil).Once()
	s.proto.state.On("Final").Return(finalSnapshot, nil).Once()

	epoch.On("ClusterByChainID", mock.Anything).Return(cluster, nil)
	epochs.On("Current").Return(epoch, nil)
	snap.On("Epochs").Return(epochs)

	// prepare cluster committee members
	clusterCommittee := unittest.IdentityListFixture(32 * 4).Filter(filter.HasRole[flow.Identity](flow.RoleCollection)).ToSkeleton()
	cluster.On("Members").Return(clusterCommittee, nil)

	eng, _, _ := s.initEngineAndSyncer()

	irrecoverableCtx, cancel := irrecoverable.NewMockSignalerContextWithCancel(s.T(), s.ctx)
	eng.ComponentManager.Start(irrecoverableCtx)
	unittest.RequireCloseBefore(s.T(), eng.Ready(), 100*time.Millisecond, "could not start worker")
	defer func() {
		cancel()
		unittest.RequireCloseBefore(s.T(), eng.Done(), 100*time.Millisecond, "could not stop worker")
	}()

	block := s.generateBlock(clusterCommittee, snap)
	block.Height = s.finalizedBlock.Height + 1
	s.blockMap[block.Height] = block
	s.mockCollectionsForBlock(block)
	s.finalizedBlock = block.ToHeader()

	hotstuffBlock := hotmodel.Block{
		BlockID: block.ID(),
	}

	// expect that the block storage is indexed with each of the collection guarantee
	s.blocks.On("BatchIndexBlockContainingCollectionGuarantees", mock.Anything, mock.Anything, block.ID(), []flow.Identifier(flow.GetIDs(block.Payload.Guarantees))).Return(nil).Once()

	missingCollectionCount := 4
	wg := sync.WaitGroup{}
	wg.Add(missingCollectionCount)

	for _, cg := range block.Payload.Guarantees {
		s.request.On("EntityByID", cg.CollectionID, mock.Anything).Return().Run(func(args mock.Arguments) {
			// Ensure the test does not complete its work faster than necessary
			wg.Done()
		}).Once()
	}

	// force should be called once
	s.request.On("Force").Return().Once()

	// process the block through the finalized callback
	s.distributor.OnFinalizedBlock(&hotstuffBlock)

	unittest.RequireReturnsBefore(s.T(), wg.Wait, 100*time.Millisecond, "expect to process new block before timeout")

	// assert that the block was retrieved and all collections were requested
	s.headers.AssertExpectations(s.T())
	s.request.AssertNumberOfCalls(s.T(), "EntityByID", len(block.Payload.Guarantees))
}

// TestOnFinalizedBlockSeveralBlocksAhead checks OnFinalizedBlock with a block several blocks newer than the last block processed
func (s *Suite) TestOnFinalizedBlockSeveralBlocksAhead() {
	cluster := new(protocolmock.Cluster)
	epoch := new(protocolmock.CommittedEpoch)
	epochs := new(protocolmock.EpochQuery)
	snap := new(protocolmock.Snapshot)

	finalSnapshot := protocolmock.NewSnapshot(s.T())
	finalSnapshot.On("Head").Return(s.finalizedBlock, nil).Once()
	s.proto.state.On("Final").Return(finalSnapshot, nil).Once()

	epoch.On("ClusterByChainID", mock.Anything).Return(cluster, nil)
	epochs.On("Current").Return(epoch, nil)
	snap.On("Epochs").Return(epochs)

	// prepare cluster committee members
	clusterCommittee := unittest.IdentityListFixture(32 * 4).Filter(filter.HasRole[flow.Identity](flow.RoleCollection)).ToSkeleton()
	cluster.On("Members").Return(clusterCommittee, nil)

	eng, _, _ := s.initEngineAndSyncer()

	irrecoverableCtx, cancel := irrecoverable.NewMockSignalerContextWithCancel(s.T(), s.ctx)
	eng.ComponentManager.Start(irrecoverableCtx)
	unittest.RequireCloseBefore(s.T(), eng.Ready(), 100*time.Millisecond, "could not start worker")
	defer func() {
		cancel()
		unittest.RequireCloseBefore(s.T(), eng.Done(), 100*time.Millisecond, "could not stop worker")
	}()

	newBlocksCount := 3
	startHeight := s.finalizedBlock.Height + 1
	blocks := make([]*flow.Block, newBlocksCount)

	// generate the test blocks, cgs and collections
	for i := 0; i < newBlocksCount; i++ {
		block := s.generateBlock(clusterCommittee, snap)
		block.Height = startHeight + uint64(i)
		s.blockMap[block.Height] = block
		blocks[i] = block
		s.mockCollectionsForBlock(block)
		s.finalizedBlock = block.ToHeader()
	}

	// latest of all the new blocks which are newer than the last block processed
	latestBlock := blocks[2]

	// block several blocks newer than the last block processed
	hotstuffBlock := hotmodel.Block{
		BlockID: latestBlock.ID(),
	}

	missingCollectionCountPerBlock := 4
	wg := sync.WaitGroup{}
	wg.Add(missingCollectionCountPerBlock * newBlocksCount)

	// expected all new blocks after last block processed
	for _, block := range blocks {
		s.blocks.On("BatchIndexBlockContainingCollectionGuarantees", mock.Anything, mock.Anything, block.ID(), []flow.Identifier(flow.GetIDs(block.Payload.Guarantees))).Return(nil).Once()

		for _, cg := range block.Payload.Guarantees {
			s.request.On("EntityByID", cg.CollectionID, mock.Anything).Return().Run(func(args mock.Arguments) {
				// Ensure the test does not complete its work faster than necessary, so we can check all expected results
				wg.Done()
			}).Once()
		}
		// force should be called once
		s.request.On("Force").Return().Once()
	}

	s.distributor.OnFinalizedBlock(&hotstuffBlock)

	unittest.RequireReturnsBefore(s.T(), wg.Wait, 100*time.Millisecond, "expect to process all blocks before timeout")

	expectedEntityByIDCalls := 0
	for _, block := range blocks {
		expectedEntityByIDCalls += len(block.Payload.Guarantees)
	}

	s.headers.AssertExpectations(s.T())
	s.blocks.AssertNumberOfCalls(s.T(), "BatchIndexBlockContainingCollectionGuarantees", newBlocksCount)
	s.request.AssertNumberOfCalls(s.T(), "EntityByID", expectedEntityByIDCalls)
}

// TestExecutionReceiptsAreIndexed checks that execution receipts are properly indexed
func (s *Suite) TestExecutionReceiptsAreIndexed() {
	eng, _, _ := s.initEngineAndSyncer()

	originID := unittest.IdentifierFixture()
	collection := unittest.CollectionFixture(5)
	light := collection.Light()

	// we should store the collection and index its transactions
	s.collections.On("StoreAndIndexByTransaction", &collection).Return(light, nil).Once()
	block := unittest.BlockFixture(
		unittest.Block.WithHeight(0),
		unittest.Block.WithPayload(
			unittest.PayloadFixture(unittest.WithGuarantees([]*flow.CollectionGuarantee{}...)),
		),
	)
	s.blocks.On("ByID", mock.Anything).Return(block, nil)

	// for each transaction in the collection, we should store it
	needed := make(map[flow.Identifier]struct{})
	for _, txID := range light.Transactions {
		needed[txID] = struct{}{}
	}
	s.transactions.On("Store", mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			tx := args.Get(0).(*flow.TransactionBody)
			_, pending := needed[tx.ID()]
			s.Assert().True(pending, "tx not pending (%x)", tx.ID())
		},
	)
	er1 := unittest.ExecutionReceiptFixture()
	er2 := unittest.ExecutionReceiptFixture()

	s.receipts.On("Store", mock.Anything).Return(nil)
	s.blocks.On("ByID", er1.ExecutionResult.BlockID).Return(nil, storage.ErrNotFound)

	s.receipts.On("Store", mock.Anything).Return(nil)
	s.blocks.On("ByID", er2.ExecutionResult.BlockID).Return(nil, storage.ErrNotFound)

	err := eng.handleExecutionReceipt(originID, er1)
	require.NoError(s.T(), err)

	err = eng.handleExecutionReceipt(originID, er2)
	require.NoError(s.T(), err)

	s.receipts.AssertExpectations(s.T())
	s.results.AssertExpectations(s.T())
	s.receipts.AssertExpectations(s.T())
}

// TestCollectionSyncing tests the happy path of syncing collections for finalized blocks.
// It performs syncs for a single block passed via the OnFinalizedBlock callback, and verifies that
// the finalized block processing logic submits the request for each collection in the finalized block,
// the indexer indexes all collections received from the network, and the last full block height is
// updated after the collections are indexed.
func (s *Suite) TestCollectionSyncing() {
	g := fixtures.NewGeneratorSuite()

	guarantors := g.Identities().List(3, fixtures.Identity.WithRole(flow.RoleCollection))
	signerIndices, err := signature.EncodeSignersToIndices(guarantors.NodeIDs(), guarantors.NodeIDs())
	require.NoError(s.T(), err)

	collections := g.Collections().List(10)
	guarantees := make([]*flow.CollectionGuarantee, len(collections))
	for i, collection := range collections {
		guarantee := g.Guarantees().Fixture(
			fixtures.Guarantee.WithCollectionID(collection.ID()),
			fixtures.Guarantee.WithSignerIndices(signerIndices),
		)
		guarantees[i] = guarantee
	}
	payload := g.Payloads().Fixture(
		fixtures.Payload.WithGuarantees(guarantees...),
		fixtures.Payload.WithSeals(g.Seals().List(10)...),
	)
	block := g.Blocks().Fixture(
		fixtures.Block.WithPayload(payload),
		fixtures.Block.WithParentHeader(s.finalizedBlock), // replace the finalized block
	)
	s.blockMap[block.Height] = block
	hotstuffBlock := hotmodel.Block{
		BlockID: block.ID(),
	}

	for _, guarantee := range block.Payload.Guarantees {
		// initially, all collections should be missing from storage
		s.collections.On("LightByID", guarantee.CollectionID).Return(nil, storage.ErrNotFound)

		// setup requester engine requests
		s.mockGuarantorsForCollection(guarantee, guarantors.ToSkeleton())
		s.request.On("EntityByID", guarantee.CollectionID, mock.Anything).Once()
	}
	s.request.On("Force").Once()

	// setup finalized block indexer mocks
	guaranteeIDs := []flow.Identifier(flow.GetIDs(block.Payload.Guarantees))
	s.blocks.On("BatchIndexBlockContainingCollectionGuarantees", mock.Anything, mock.Anything, block.ID(), guaranteeIDs).Return(nil).Once()
	for _, seal := range payload.Seals {
		s.results.On("BatchIndex", mock.Anything, mock.Anything, seal.BlockID, seal.ResultID).Return(nil).Once()
	}

	// initialize the engine using the initial finalized block
	initialFinalSnapshot := protocolmock.NewSnapshot(s.T())
	initialFinalSnapshot.On("Head").Return(s.finalizedBlock, nil)
	s.proto.state.On("Final").Return(initialFinalSnapshot, nil)

	eng, syncer, _ := s.initEngineAndSyncer()

	irrecoverableCtx, cancel := irrecoverable.NewMockSignalerContextWithCancel(s.T(), s.ctx)
	eng.ComponentManager.Start(irrecoverableCtx)
	unittest.RequireCloseBefore(s.T(), eng.Ready(), 1*time.Second, "could not start worker")
	defer func() {
		cancel()
		unittest.RequireCloseBefore(s.T(), eng.Done(), 100*time.Millisecond, "could not stop worker")
	}()

	// progress the finalized block, and submit the new block to the engine
	newFinalSnapshot := protocolmock.NewSnapshot(s.T())
	newFinalSnapshot.On("Head").Return(block.ToHeader(), nil)
	s.proto.state.On("Final").Unset()
	s.proto.state.On("Final").Return(newFinalSnapshot, nil)

	s.distributor.OnFinalizedBlock(&hotstuffBlock)

	// wait until the finalized block jobqueue completes processing the block
	require.Eventually(s.T(), func() bool {
		return eng.finalizedBlockConsumer.LastProcessedIndex() == block.Height
	}, 2*time.Second, 10*time.Millisecond, "finalized block processor never processed block")

	// all requests should be sent after the finalized block processor completes processing the block.
	// The requester engine calls the syncer's OnCollectionDownloaded callback for each response.
	// simulate receiving the collection responses from the network.
	for _, collection := range collections {
		collectionID := collection.ID()
		light := collection.Light()
		s.collections.On("StoreAndIndexByTransaction", mock.Anything, collection).Return(light, nil).Once()

		// the collections are now available in storage.
		s.collections.On("LightByID", collectionID).Unset()
		s.collections.On("LightByID", collectionID).Return(light, nil)

		syncer.OnCollectionDownloaded(g.Identifiers().Fixture(), collection)
	}

	// make sure that the collection indexer updates the last full block height
	require.Eventually(s.T(), func() bool {
		return s.lastFullBlockHeight.Value() == block.Height
	}, 2*time.Second, 100*time.Millisecond, "last full block height never updated")
}

// TestOnFinalizedBlockAlreadyIndexed checks that when a block has already been indexed
// (storage.ErrAlreadyExists), the engine logs a warning and continues processing by
// requesting collections and updating metrics. This can happen when the job queue processed
// index hasn't been updated yet after a previous indexing operation.
func (s *Suite) TestOnFinalizedBlockAlreadyIndexed() {
	cluster := new(protocolmock.Cluster)
	epoch := new(protocolmock.CommittedEpoch)
	epochs := new(protocolmock.EpochQuery)
	snap := new(protocolmock.Snapshot)

	finalSnapshot := protocolmock.NewSnapshot(s.T())
	finalSnapshot.On("Head").Return(s.finalizedBlock, nil).Once()
	s.proto.state.On("Final").Return(finalSnapshot, nil).Once()

	epoch.On("ClusterByChainID", mock.Anything).Return(cluster, nil)
	epochs.On("Current").Return(epoch, nil)
	snap.On("Epochs").Return(epochs)

	// prepare cluster committee members
	clusterCommittee := unittest.IdentityListFixture(32 * 4).Filter(filter.HasRole[flow.Identity](flow.RoleCollection)).ToSkeleton()
	cluster.On("Members").Return(clusterCommittee, nil)

	eng, _, _ := s.initEngineAndSyncer()

	irrecoverableCtx, cancel := irrecoverable.NewMockSignalerContextWithCancel(s.T(), s.ctx)
	eng.ComponentManager.Start(irrecoverableCtx)
	unittest.RequireCloseBefore(s.T(), eng.Ready(), 100*time.Millisecond, "could not start worker")
	defer func() {
		cancel()
		unittest.RequireCloseBefore(s.T(), eng.Done(), 100*time.Millisecond, "could not stop worker")
	}()

	block := s.generateBlock(clusterCommittee, snap)
	block.Height = s.finalizedBlock.Height + 1
	s.blockMap[block.Height] = block
	s.mockCollectionsForBlock(block)
	s.finalizedBlock = block.ToHeader()

	hotstuffBlock := hotmodel.Block{
		BlockID: block.ID(),
	}

	// simulate that the block has already been indexed (e.g., by a previous job queue run)
	// by returning storage.ErrAlreadyExists
	s.blocks.On("BatchIndexBlockContainingCollectionGuarantees", mock.Anything, mock.Anything, block.ID(), []flow.Identifier(flow.GetIDs(block.Payload.Guarantees))).
		Return(storage.ErrAlreadyExists).Once()

	missingCollectionCount := 4
	wg := sync.WaitGroup{}
	wg.Add(missingCollectionCount)

	// even though block is already indexed, collections should still be requested
	for _, cg := range block.Payload.Guarantees {
		s.request.On("EntityByID", cg.CollectionID, mock.Anything).Return().Run(func(args mock.Arguments) {
			wg.Done()
		}).Once()
	}

	// force should be called once
	s.request.On("Force").Return().Once()

	// process the block through the finalized callback
	s.distributor.OnFinalizedBlock(&hotstuffBlock)

	unittest.RequireReturnsBefore(s.T(), wg.Wait, 100*time.Millisecond, "expect to process new block before timeout")

	// assert that collections were still requested despite the block being already indexed
	s.headers.AssertExpectations(s.T())
	s.request.AssertNumberOfCalls(s.T(), "EntityByID", len(block.Payload.Guarantees))
}

func (s *Suite) mockGuarantorsForCollection(guarantee *flow.CollectionGuarantee, members flow.IdentitySkeletonList) {
	cluster := protocolmock.NewCluster(s.T())
	cluster.On("Members").Return(members, nil).Once()

	epoch := protocolmock.NewCommittedEpoch(s.T())
	epoch.On("ClusterByChainID", guarantee.ClusterChainID).Return(cluster, nil).Once()

	query := protocolmock.NewEpochQuery(s.T())
	query.On("Current").Return(epoch, nil).Once()

	snapshot := protocolmock.NewSnapshot(s.T())
	snapshot.On("Epochs").Return(query).Once()

	s.proto.state.On("AtBlockID", guarantee.ReferenceBlockID).Return(snapshot).Once()
}
