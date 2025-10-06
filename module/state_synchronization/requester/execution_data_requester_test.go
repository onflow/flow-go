package requester

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	exedatamock "github.com/onflow/flow-go/module/executiondatasync/execution_data/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/state/protocol"
	statemock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

type ExecutionDataRequesterSuite struct {
	suite.Suite

	blobstore   blobs.Blobstore
	datastore   datastore.Batching
	db          *pebble.DB
	downloader  *exedatamock.Downloader
	distributor *ExecutionDataDistributor

	run edTestRun

	mockSnapshot *mockSnapshot
}

func TestExecutionDataRequesterSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ExecutionDataRequesterSuite))
}

func (suite *ExecutionDataRequesterSuite) SetupTest() {
	suite.datastore = dssync.MutexWrap(datastore.NewMapDatastore())
	suite.blobstore = blobs.NewBlobstore(suite.datastore)

	suite.run = edTestRun{
		"",
		100,
		func(_ int) map[uint64]testExecutionDataCallback {
			return map[uint64]testExecutionDataCallback{}
		},
	}
}

type testExecutionDataServiceEntry struct {
	// When set, the response from this call back will be returned for any calls to Get
	// Note: this callback is called twice by mockery, once for the execution data and once for the error
	fn testExecutionDataCallback
	// When set (and fn is unset), this error will be returned for any calls to Get for this ED
	Err error
	// Otherwise, the execution data will be returned directly with no error
	ExecutionData *execution_data.BlockExecutionData
}

type specialBlockGenerator func(int) map[uint64]testExecutionDataCallback
type edTestRun struct {
	name          string
	blockCount    int
	specialBlocks specialBlockGenerator
}

type testExecutionDataCallback func(*execution_data.BlockExecutionData) (*execution_data.BlockExecutionData, error)

func MockDownloader(edStore map[flow.Identifier]*testExecutionDataServiceEntry) *exedatamock.Downloader {
	downloader := new(exedatamock.Downloader)

	get := func(id flow.Identifier) (*execution_data.BlockExecutionData, error) {
		ed, has := edStore[id]

		// return not found
		if !has {
			return nil, execution_data.NewBlobNotFoundError(flow.IdToCid(id))
		}

		// use a callback. this is useful for injecting a pause or custom error behavior
		if ed.fn != nil {
			return ed.fn(ed.ExecutionData)
		}

		// return a custom error
		if ed.Err != nil {
			return nil, ed.Err
		}

		// return the specific execution data
		return ed.ExecutionData, nil
	}

	downloader.On("Get", mock.Anything, mock.AnythingOfType("flow.Identifier")).
		Return(
			func(ctx context.Context, id flow.Identifier) *execution_data.BlockExecutionData {
				ed, _ := get(id)
				return ed
			},
			func(ctx context.Context, id flow.Identifier) error {
				_, err := get(id)
				return err
			},
		).
		Maybe() // Maybe() needed to get call count

	noop := module.NoopReadyDoneAware{}
	downloader.On("Ready").
		Return(func() <-chan struct{} { return noop.Ready() }).
		Maybe() // Maybe() needed to get call count

	return downloader
}

func (suite *ExecutionDataRequesterSuite) mockProtocolState(blocksByHeight map[uint64]*flow.Block) *statemock.State {
	state := new(statemock.State)

	suite.mockSnapshot = new(mockSnapshot)
	suite.mockSnapshot.set(blocksByHeight[0].ToHeader(), nil) // genesis block

	state.On("Sealed").Return(suite.mockSnapshot).Maybe()
	return state
}

// TestRequesterProcessesBlocks tests that the requester processes all blocks and sends notifications
// in order.
func (suite *ExecutionDataRequesterSuite) TestRequesterProcessesBlocks() {

	tests := []edTestRun{
		// Test that blocks are processed in order
		{
			"happy path",
			100,
			func(_ int) map[uint64]testExecutionDataCallback {
				return map[uint64]testExecutionDataCallback{}
			},
		},
		// Tests that blocks that are missed are properly retried and notifications are received in order
		{
			"requests blocks with some missed",
			100,
			generateBlocksWithSomeMissed,
		},
		// Tests that blocks that are missed are properly retried and backfilled
		{
			"requests blocks with some delayed",
			100,
			generateBlocksWithRandomDelays,
		},
	}

	for _, run := range tests {
		suite.Run(run.name, func() {
			unittest.RunWithPebbleDB(suite.T(), func(db *pebble.DB) {
				suite.db = db

				suite.datastore = dssync.MutexWrap(datastore.NewMapDatastore())
				suite.blobstore = blobs.NewBlobstore(suite.datastore)

				testData := generateTestData(suite.T(), suite.blobstore, run.blockCount, run.specialBlocks(run.blockCount))
				edr, fd := suite.prepareRequesterTest(testData)
				fetchedExecutionData := suite.runRequesterTest(edr, fd, testData)

				verifyFetchedExecutionData(suite.T(), fetchedExecutionData, testData)

				suite.T().Log("Shutting down test")
			})
		})
	}
}

// TestRequesterResumesAfterRestart tests that the requester will pick up where it left off after a
// restart, without skipping any blocks
func (suite *ExecutionDataRequesterSuite) TestRequesterResumesAfterRestart() {
	suite.datastore = dssync.MutexWrap(datastore.NewMapDatastore())
	suite.blobstore = blobs.NewBlobstore(suite.datastore)

	testData := generateTestData(suite.T(), suite.blobstore, suite.run.blockCount, suite.run.specialBlocks(suite.run.blockCount))

	test := func(stopHeight, resumeHeight uint64) {
		testData.fetchedExecutionData = nil

		unittest.RunWithPebbleDB(suite.T(), func(db *pebble.DB) {
			suite.db = db

			// Process half of the blocks
			edr, fd := suite.prepareRequesterTest(testData)
			testData.stopHeight = stopHeight
			testData.resumeHeight = 0
			testData.fetchedExecutionData = suite.runRequesterTest(edr, fd, testData)

			// Stand up a new component using the same datastore, and make sure all remaining
			// blocks are processed
			edr, fd = suite.prepareRequesterTest(testData)
			testData.stopHeight = 0
			testData.resumeHeight = resumeHeight
			fetchedExecutionData := suite.runRequesterTest(edr, fd, testData)

			verifyFetchedExecutionData(suite.T(), fetchedExecutionData, testData)

			suite.T().Log("Shutting down test")
		})
	}

	suite.Run("requester resumes processing with no gap", func() {
		stopHeight := testData.startHeight + uint64(suite.run.blockCount)/2
		resumeHeight := stopHeight + 1
		test(stopHeight, resumeHeight)
	})

	suite.Run("requester resumes processing with gap", func() {
		stopHeight := testData.startHeight + uint64(suite.run.blockCount)/2
		resumeHeight := testData.endHeight
		test(stopHeight, resumeHeight)
	})
}

// TestRequesterCatchesUp tests that the requester processes all heights when it starts with a
// backlog of sealed blocks.
func (suite *ExecutionDataRequesterSuite) TestRequesterCatchesUp() {
	unittest.RunWithPebbleDB(suite.T(), func(db *pebble.DB) {
		suite.db = db

		suite.datastore = dssync.MutexWrap(datastore.NewMapDatastore())
		suite.blobstore = blobs.NewBlobstore(suite.datastore)

		testData := generateTestData(suite.T(), suite.blobstore, suite.run.blockCount, suite.run.specialBlocks(suite.run.blockCount))

		// start processing with all seals available
		edr, fd := suite.prepareRequesterTest(testData)
		testData.resumeHeight = testData.endHeight
		fetchedExecutionData := suite.runRequesterTest(edr, fd, testData)

		verifyFetchedExecutionData(suite.T(), fetchedExecutionData, testData)

		suite.T().Log("Shutting down test")
	})
}

// TestRequesterPausesAndResumes tests that the requester pauses when it downloads maxSearchAhead
// blocks beyond the last processed block, and resumes when it catches up.
func (suite *ExecutionDataRequesterSuite) TestRequesterPausesAndResumes() {
	unittest.RunWithPebbleDB(suite.T(), func(db *pebble.DB) {
		suite.db = db

		pauseHeight := uint64(10)
		maxSearchAhead := uint64(5)

		// Downloads will succeed immediately for all blocks except pauseHeight, which will hang
		// until the resume() is called.
		generate, resume := generatePauseResume(pauseHeight)

		testData := generateTestData(suite.T(), suite.blobstore, suite.run.blockCount, generate(suite.run.blockCount))
		testData.maxSearchAhead = maxSearchAhead
		testData.waitTimeout = time.Second * 10

		// calculate the expected number of blocks that should be downloaded before resuming.
		// the test should download all blocks up to pauseHeight, then maxSearchAhead blocks beyond.
		// the pause block itself is excluded.
		expectedDownloads := pauseHeight + maxSearchAhead - 1

		edr, fd := suite.prepareRequesterTest(testData)
		fetchedExecutionData := suite.runRequesterTestPauseResume(edr, fd, testData, int(expectedDownloads), resume)

		verifyFetchedExecutionData(suite.T(), fetchedExecutionData, testData)

		suite.T().Log("Shutting down test")
	})
}

// TestRequesterHalts tests that the requester handles halting correctly when it encounters an
// invalid block
func (suite *ExecutionDataRequesterSuite) TestRequesterHalts() {
	unittest.RunWithPebbleDB(suite.T(), func(db *pebble.DB) {
		suite.db = db

		suite.run.blockCount = 10
		suite.datastore = dssync.MutexWrap(datastore.NewMapDatastore())
		suite.blobstore = blobs.NewBlobstore(suite.datastore)

		// generate a block that will return a malformed blob error. causing the requester to halt
		generate, expectedErr := generateBlocksWithHaltingError(suite.run.blockCount)
		testData := generateTestData(suite.T(), suite.blobstore, suite.run.blockCount, generate(suite.run.blockCount))

		// start processing with all seals available
		edr, followerDistributor := suite.prepareRequesterTest(testData)
		testData.resumeHeight = testData.endHeight
		testData.expectedIrrecoverable = expectedErr
		fetchedExecutionData := suite.runRequesterTestHalts(edr, followerDistributor, testData)
		assert.Less(suite.T(), len(fetchedExecutionData), testData.sealedCount)

		suite.T().Log("Shutting down test")
	})
}

func generateBlocksWithSomeMissed(blockCount int) map[uint64]testExecutionDataCallback {
	missing := map[uint64]testExecutionDataCallback{}

	// every 5th block fails to download n times before succeeding
	for i := uint64(0); i < uint64(blockCount); i++ {
		if i%5 > 0 {
			continue
		}

		failures := rand.Intn(3) + 1
		attempts := 0
		missing[i] = func(ed *execution_data.BlockExecutionData) (*execution_data.BlockExecutionData, error) {
			if attempts < failures*2 { // this func is run twice for every attempt by the mock (once for ExecutionData one for errors)
				attempts++
				// This should fail the first n fetch attempts
				time.Sleep(time.Duration(rand.Intn(25)) * time.Millisecond)
				return nil, &execution_data.BlobNotFoundError{}
			}

			return ed, nil
		}
	}

	return missing
}

func generateBlocksWithRandomDelays(blockCount int) map[uint64]testExecutionDataCallback {
	// delay every third block by a random amount
	delays := map[uint64]testExecutionDataCallback{}
	for i := uint64(0); i < uint64(blockCount); i++ {
		if i%5 > 0 {
			continue
		}

		delays[i] = func(ed *execution_data.BlockExecutionData) (*execution_data.BlockExecutionData, error) {
			time.Sleep(time.Duration(rand.Intn(25)) * time.Millisecond)
			return ed, nil
		}
	}

	return delays
}

func generateBlocksWithHaltingError(blockCount int) (specialBlockGenerator, error) {
	// return a MalformedDataError on the second to last block
	height := uint64(blockCount - 5)
	err := fmt.Errorf("halting error: %w", &execution_data.MalformedDataError{})

	generate := func(int) map[uint64]testExecutionDataCallback {
		return map[uint64]testExecutionDataCallback{
			height: func(ed *execution_data.BlockExecutionData) (*execution_data.BlockExecutionData, error) {
				return nil, err
			},
		}
	}
	return generate, err
}

func generatePauseResume(pauseHeight uint64) (specialBlockGenerator, func()) {
	pause := make(chan struct{})

	blocks := map[uint64]testExecutionDataCallback{}
	blocks[pauseHeight] = func(ed *execution_data.BlockExecutionData) (*execution_data.BlockExecutionData, error) {
		<-pause
		return ed, nil
	}

	generate := func(int) map[uint64]testExecutionDataCallback { return blocks }
	resume := func() { close(pause) }

	return generate, resume
}

func (suite *ExecutionDataRequesterSuite) prepareRequesterTest(cfg *fetchTestRun) (state_synchronization.ExecutionDataRequester, *pubsub.FollowerDistributor) {
	logger := unittest.Logger()
	metricsCollector := metrics.NewNoopCollector()

	headers := synctest.MockBlockHeaderStorage(
		synctest.WithByID(cfg.blocksByID),
		synctest.WithByHeight(cfg.blocksByHeight),
		synctest.WithBlockIDByHeight(cfg.blocksByHeight),
	)
	results := synctest.MockResultsStorage(
		synctest.WithResultByID(cfg.resultsByID),
	)
	seals := synctest.MockSealsStorage(
		synctest.WithSealsByBlockID(cfg.sealsByBlockID),
	)
	state := suite.mockProtocolState(cfg.blocksByHeight)

	suite.downloader = MockDownloader(cfg.executionDataEntries)
	suite.distributor = NewExecutionDataDistributor()

	heroCache := herocache.NewBlockExecutionData(subscription.DefaultCacheSize, logger, metricsCollector)
	edCache := cache.NewExecutionDataCache(suite.downloader, headers, seals, results, heroCache)

	followerDistributor := pubsub.NewFollowerDistributor()
	processedHeight := store.NewConsumerProgress(pebbleimpl.ToDB(suite.db), module.ConsumeProgressExecutionDataRequesterBlockHeight)
	processedNotification := store.NewConsumerProgress(pebbleimpl.ToDB(suite.db), module.ConsumeProgressExecutionDataRequesterNotification)

	edr, err := New(
		logger,
		metricsCollector,
		suite.downloader,
		edCache,
		processedHeight,
		processedNotification,
		state,
		headers,
		ExecutionDataConfig{
			InitialBlockHeight: cfg.startHeight - 1,
			MaxSearchAhead:     cfg.maxSearchAhead,
			FetchTimeout:       cfg.fetchTimeout,
			RetryDelay:         cfg.retryDelay,
			MaxRetryDelay:      cfg.maxRetryDelay,
		},
		suite.distributor,
	)
	require.NoError(suite.T(), err)

	followerDistributor.AddOnBlockFinalizedConsumer(edr.OnBlockFinalized)

	return edr, followerDistributor
}

func (suite *ExecutionDataRequesterSuite) runRequesterTestHalts(edr state_synchronization.ExecutionDataRequester, followerDistributor *pubsub.FollowerDistributor, cfg *fetchTestRun) receivedExecutionData {
	// make sure test helper goroutines are cleaned up
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	signalerCtx := irrecoverable.NewMockSignalerContext(suite.T(), ctx)

	testDone := make(chan struct{})
	fetchedExecutionData := cfg.FetchedExecutionData()

	// collect all execution data notifications
	suite.distributor.AddOnExecutionDataReceivedConsumer(suite.consumeExecutionDataNotifications(cfg, func() { close(testDone) }, fetchedExecutionData))

	edr.Start(signalerCtx)
	unittest.RequireCloseBefore(suite.T(), edr.Ready(), cfg.waitTimeout, "timed out waiting for requester to be ready")

	// Send blocks through followerDistributor
	suite.finalizeBlocks(cfg, followerDistributor)

	// testDone should never close because the requester paused
	unittest.RequireNeverClosedWithin(suite.T(), testDone, 100*time.Millisecond, "finished sending notifications unexpectedly")
	suite.T().Log("All notifications received")

	cancel()
	unittest.RequireCloseBefore(suite.T(), edr.Done(), cfg.waitTimeout, "timed out waiting for requester to shutdown")

	return fetchedExecutionData
}

func (suite *ExecutionDataRequesterSuite) runRequesterTestPauseResume(edr state_synchronization.ExecutionDataRequester, followerDistributor *pubsub.FollowerDistributor, cfg *fetchTestRun, expectedDownloads int, resume func()) receivedExecutionData {
	// make sure test helper goroutines are cleaned up
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(suite.T(), ctx)

	testDone := make(chan struct{})
	fetchedExecutionData := cfg.FetchedExecutionData()

	// collect all execution data notifications
	suite.distributor.AddOnExecutionDataReceivedConsumer(suite.consumeExecutionDataNotifications(cfg, func() { close(testDone) }, fetchedExecutionData))

	edr.Start(signalerCtx)
	unittest.RequireCloseBefore(suite.T(), edr.Ready(), cfg.waitTimeout, "timed out waiting for requester to be ready")

	// Send all blocks through followerDistributor
	suite.finalizeBlocks(cfg, followerDistributor)

	// requester should pause downloads until resume is called, so testDone should not be closed
	unittest.RequireNeverClosedWithin(suite.T(), testDone, 500*time.Millisecond, "finished unexpectedly")

	// confirm the expected number of downloads were attempted
	suite.downloader.AssertNumberOfCalls(suite.T(), "Get", expectedDownloads)

	suite.T().Log("Resuming")
	resume()

	// Pause until we've received all of the expected notifications
	unittest.RequireCloseBefore(suite.T(), testDone, cfg.waitTimeout, "timed out waiting for notifications")
	suite.T().Log("All notifications received")

	cancel()
	unittest.RequireCloseBefore(suite.T(), edr.Done(), cfg.waitTimeout, "timed out waiting for requester to shutdown")

	return fetchedExecutionData
}

func (suite *ExecutionDataRequesterSuite) runRequesterTest(edr state_synchronization.ExecutionDataRequester, followerDistributor *pubsub.FollowerDistributor, cfg *fetchTestRun) receivedExecutionData {
	// make sure test helper goroutines are cleaned up
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(suite.T(), ctx)

	// wait for all notifications
	testDone := make(chan struct{})

	fetchedExecutionData := cfg.FetchedExecutionData()

	// collect all execution data notifications
	suite.distributor.AddOnExecutionDataReceivedConsumer(suite.consumeExecutionDataNotifications(cfg, func() { close(testDone) }, fetchedExecutionData))

	edr.Start(signalerCtx)
	unittest.RequireCloseBefore(suite.T(), edr.Ready(), cfg.waitTimeout, "timed out waiting for requester to be ready")

	// Send blocks through followerDistributor
	suite.finalizeBlocks(cfg, followerDistributor)

	// Pause until we've received all of the expected notifications
	unittest.RequireCloseBefore(suite.T(), testDone, cfg.waitTimeout, "timed out waiting for notifications")
	suite.T().Log("All notifications received")

	cancel()
	unittest.RequireCloseBefore(suite.T(), edr.Done(), cfg.waitTimeout, "timed out waiting for requester to shutdown")

	return fetchedExecutionData
}

func (suite *ExecutionDataRequesterSuite) consumeExecutionDataNotifications(cfg *fetchTestRun, done func(), fetchedExecutionData map[flow.Identifier]*execution_data.BlockExecutionData) func(ed *execution_data.BlockExecutionDataEntity) {
	return func(ed *execution_data.BlockExecutionDataEntity) {
		if _, has := fetchedExecutionData[ed.BlockID]; has {
			suite.T().Errorf("duplicate execution data for block %s", ed.BlockID)
			return
		}

		fetchedExecutionData[ed.BlockID] = ed.BlockExecutionData
		if _, ok := cfg.blocksByID[ed.BlockID]; !ok {
			suite.T().Errorf("unknown execution data for block %s", ed.BlockID)
			return
		}

		suite.T().Logf("notified of execution data for block %v height %d (%d/%d)", ed.BlockID, cfg.blocksByID[ed.BlockID].Height, len(fetchedExecutionData), cfg.sealedCount)

		if cfg.IsLastSeal(ed.BlockID) {
			done()
		}
	}
}

func (suite *ExecutionDataRequesterSuite) finalizeBlocks(cfg *fetchTestRun, followerDistributor *pubsub.FollowerDistributor) {
	for i := cfg.StartHeight(); i <= cfg.endHeight; i++ {
		b := cfg.blocksByHeight[i]

		suite.T().Log(">>>> Finalizing block", b.ID(), b.Height)

		if len(b.Payload.Seals) > 0 {
			seal := b.Payload.Seals[0]
			sealedHeader := cfg.blocksByID[seal.BlockID].ToHeader()

			suite.mockSnapshot.set(sealedHeader, nil)
			suite.T().Log(">>>> Sealing block", sealedHeader.ID(), sealedHeader.Height)
		}

		followerDistributor.OnFinalizedBlock(&model.Block{}) // actual block is unused

		if cfg.stopHeight == i {
			break
		}
	}
}

type receivedExecutionData map[flow.Identifier]*execution_data.BlockExecutionData
type fetchTestRun struct {
	sealedCount              int
	startHeight              uint64
	endHeight                uint64
	blocksByHeight           map[uint64]*flow.Block
	blocksByID               map[flow.Identifier]*flow.Block
	resultsByID              map[flow.Identifier]*flow.ExecutionResult
	resultsByBlockID         map[flow.Identifier]*flow.ExecutionResult
	sealsByBlockID           map[flow.Identifier]*flow.Seal
	executionDataByID        map[flow.Identifier]*execution_data.BlockExecutionData
	executionDataEntries     map[flow.Identifier]*testExecutionDataServiceEntry
	executionDataIDByBlockID map[flow.Identifier]flow.Identifier
	expectedIrrecoverable    error

	stopHeight           uint64
	resumeHeight         uint64
	fetchedExecutionData map[flow.Identifier]*execution_data.BlockExecutionData
	waitTimeout          time.Duration

	maxSearchAhead uint64
	fetchTimeout   time.Duration
	retryDelay     time.Duration
	maxRetryDelay  time.Duration
}

func (r *fetchTestRun) StartHeight() uint64 {
	if r.resumeHeight > 0 {
		return r.resumeHeight
	}
	return r.startHeight
}

func (r *fetchTestRun) StopHeight() uint64 {
	if r.stopHeight > 0 {
		return r.stopHeight
	}
	return r.endHeight
}

func (r *fetchTestRun) FetchedExecutionData() receivedExecutionData {
	if r.fetchedExecutionData == nil {
		return make(receivedExecutionData, r.sealedCount)
	}
	return r.fetchedExecutionData
}

// IsLastSeal returns true if the provided blockID is the last expected sealed block for the test
func (r *fetchTestRun) IsLastSeal(blockID flow.Identifier) bool {
	stopHeight := r.StopHeight()
	lastSeal := r.blocksByHeight[stopHeight].Payload.Seals[0].BlockID
	return lastSeal == r.blocksByID[blockID].ID()
}

func generateTestData(t *testing.T, blobstore blobs.Blobstore, blockCount int, specialHeightFuncs map[uint64]testExecutionDataCallback) *fetchTestRun {
	edsEntries := map[flow.Identifier]*testExecutionDataServiceEntry{}
	blocksByHeight := map[uint64]*flow.Block{}
	blocksByID := map[flow.Identifier]*flow.Block{}
	resultsByID := map[flow.Identifier]*flow.ExecutionResult{}
	resultsByBlockID := map[flow.Identifier]*flow.ExecutionResult{}
	sealsByBlockID := map[flow.Identifier]*flow.Seal{}
	executionDataByID := map[flow.Identifier]*execution_data.BlockExecutionData{}
	executionDataIDByBlockID := map[flow.Identifier]flow.Identifier{}

	sealedCount := blockCount - 4 // seals for blocks 1-96
	firstSeal := blockCount - sealedCount

	// genesis is block 0, we start syncing from block 1
	startHeight := uint64(1)
	endHeight := uint64(blockCount) - 1

	// instantiate ExecutionDataService to generate correct CIDs
	eds := execution_data.NewExecutionDataStore(blobstore, execution_data.DefaultSerializer)

	var previousBlock *flow.Block
	var previousResult *flow.ExecutionResult
	for i := 0; i < blockCount; i++ {
		var seals []*flow.Header

		if i >= firstSeal {
			sealedBlock := blocksByHeight[uint64(i-firstSeal+1)]
			seals = []*flow.Header{
				sealedBlock.ToHeader(), // block 0 doesn't get sealed (it's pre-sealed in the genesis state)
			}

			sealsByBlockID[sealedBlock.ID()] = unittest.Seal.Fixture(
				unittest.Seal.WithBlockID(sealedBlock.ID()),
				unittest.Seal.WithResult(resultsByBlockID[sealedBlock.ID()]),
			)

			t.Logf("block %d has seals for %d", i, seals[0].Height)
		}

		height := uint64(i)
		block := buildBlock(height, previousBlock, seals)

		ed := unittest.BlockExecutionDataFixture(unittest.WithBlockExecutionDataBlockID(block.ID()))

		cid, err := eds.Add(context.Background(), ed)
		require.NoError(t, err)

		result := buildResult(block, cid, previousResult)

		blocksByHeight[height] = block
		blocksByID[block.ID()] = block
		resultsByBlockID[block.ID()] = result
		resultsByID[result.ID()] = result

		// ignore all the data we don't need to verify the test
		if i > 0 && i <= sealedCount {
			executionDataByID[block.ID()] = ed
			edsEntries[cid] = &testExecutionDataServiceEntry{ExecutionData: ed}
			if fn, has := specialHeightFuncs[height]; has {
				edsEntries[cid].fn = fn
			}

			executionDataIDByBlockID[block.ID()] = cid
		}

		previousBlock = block
		previousResult = result
	}

	return &fetchTestRun{
		sealedCount:              sealedCount,
		startHeight:              startHeight,
		endHeight:                endHeight,
		blocksByHeight:           blocksByHeight,
		blocksByID:               blocksByID,
		resultsByBlockID:         resultsByBlockID,
		resultsByID:              resultsByID,
		sealsByBlockID:           sealsByBlockID,
		executionDataByID:        executionDataByID,
		executionDataEntries:     edsEntries,
		executionDataIDByBlockID: executionDataIDByBlockID,
		waitTimeout:              time.Second * 5,

		maxSearchAhead: DefaultMaxSearchAhead,
		fetchTimeout:   DefaultFetchTimeout,
		retryDelay:     1 * time.Millisecond,
		maxRetryDelay:  15 * time.Millisecond,
	}
}

func buildBlock(height uint64, parent *flow.Block, seals []*flow.Header) *flow.Block {
	if parent == nil {
		return unittest.Block.Genesis(flow.Emulator)
	}

	if len(seals) == 0 {
		return unittest.BlockWithParentFixture(parent.ToHeader())
	}

	return unittest.BlockWithParentAndSeals(parent.ToHeader(), seals)
}

func buildResult(block *flow.Block, cid flow.Identifier, previousResult *flow.ExecutionResult) *flow.ExecutionResult {
	opts := []func(result *flow.ExecutionResult){
		unittest.WithBlock(block),
		unittest.WithExecutionDataID(cid),
	}

	if previousResult != nil {
		opts = append(opts, unittest.WithPreviousResult(*previousResult))
	}

	return unittest.ExecutionResultFixture(opts...)
}

func verifyFetchedExecutionData(t *testing.T, actual receivedExecutionData, cfg *fetchTestRun) {
	expected := cfg.executionDataByID
	assert.Len(t, actual, len(expected))

	for i := 0; i < cfg.sealedCount; i++ {
		height := cfg.startHeight + uint64(i)
		block := cfg.blocksByHeight[height]
		blockID := block.ID()

		expectedED := expected[blockID]
		actualED, has := actual[blockID]
		assert.True(t, has, "missing execution data for block %v height %d", blockID, height)
		if has {
			assert.Equal(t, expectedED, actualED, "execution data for block %v doesn't match", blockID)
		}
	}
}

type mockSnapshot struct {
	header *flow.Header
	err    error
	mu     sync.Mutex
}

var _ protocol.Snapshot = &mockSnapshot{}

func (m *mockSnapshot) set(header *flow.Header, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.header = header
	m.err = err
}

func (m *mockSnapshot) Head() (*flow.Header, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.header, m.err
}

// none of these are used in this test
func (m *mockSnapshot) QuorumCertificate() (*flow.QuorumCertificate, error) { return nil, nil }
func (m *mockSnapshot) Identities(selector flow.IdentityFilter[flow.Identity]) (flow.IdentityList, error) {
	return nil, nil
}
func (m *mockSnapshot) Identity(nodeID flow.Identifier) (*flow.Identity, error) { return nil, nil }
func (m *mockSnapshot) SealedResult() (*flow.ExecutionResult, *flow.Seal, error) {
	return nil, nil, nil
}
func (m *mockSnapshot) Commit() (flow.StateCommitment, error)                    { return flow.DummyStateCommitment, nil }
func (m *mockSnapshot) SealingSegment() (*flow.SealingSegment, error)            { return nil, nil }
func (m *mockSnapshot) Descendants() ([]flow.Identifier, error)                  { return nil, nil }
func (m *mockSnapshot) RandomSource() ([]byte, error)                            { return nil, nil }
func (m *mockSnapshot) EpochPhase() (flow.EpochPhase, error)                     { return flow.EpochPhaseUndefined, nil }
func (m *mockSnapshot) Epochs() protocol.EpochQuery                              { return nil }
func (m *mockSnapshot) Params() protocol.GlobalParams                            { return nil }
func (m *mockSnapshot) EpochProtocolState() (protocol.EpochProtocolState, error) { return nil, nil }
func (m *mockSnapshot) ProtocolState() (protocol.KVStoreReader, error)           { return nil, nil }
func (m *mockSnapshot) VersionBeacon() (*flow.SealedVersionBeacon, error)        { return nil, nil }
