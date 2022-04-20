package requester_test

import (
	"context"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	"github.com/onflow/flow-go/module/state_synchronization/requester"
	"github.com/onflow/flow-go/module/state_synchronization/requester/status"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/state/protocol"
	statemock "github.com/onflow/flow-go/state/protocol/mock"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

type ExecutionDataRequesterSuite struct {
	suite.Suite

	blobservice *mocknetwork.BlobService
	datastore   datastore.Batching
	db          *badger.DB

	eds *syncmock.ExecutionDataService

	run edTestRun

	mockSnapshot *mockSnapshot
}

func TestExecutionDataRequesterSuite(t *testing.T) {
	t.Parallel()
	rand.Seed(time.Now().UnixMilli())
	suite.Run(t, new(ExecutionDataRequesterSuite))
}

func (suite *ExecutionDataRequesterSuite) SetupTest() {
	suite.datastore = dssync.MutexWrap(datastore.NewMapDatastore())
	suite.blobservice = synctest.MockBlobService(blockstore.NewBlockstore(suite.datastore))

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
	ExecutionData *state_synchronization.ExecutionData
}

type specialBlockGenerator func(int) map[uint64]testExecutionDataCallback
type edTestRun struct {
	name          string
	blockCount    int
	specialBlocks specialBlockGenerator
}

type testExecutionDataCallback func(*state_synchronization.ExecutionData) (*state_synchronization.ExecutionData, error)

func mockExecutionDataService(edStore map[flow.Identifier]*testExecutionDataServiceEntry) *syncmock.ExecutionDataService {
	eds := new(syncmock.ExecutionDataService)

	get := func(ctx context.Context, id flow.Identifier) (*state_synchronization.ExecutionData, error) {
		ed, has := edStore[id]

		// return not found
		if !has {
			return nil, &state_synchronization.BlobNotFoundError{}
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

	eds.On("Get", mock.Anything, mock.AnythingOfType("flow.Identifier")).Return(
		func(ctx context.Context, id flow.Identifier) *state_synchronization.ExecutionData {
			ed, _ := get(ctx, id)
			return ed
		},
		func(ctx context.Context, id flow.Identifier) error {
			_, err := get(ctx, id)
			return err
		},
	).Maybe() // Maybe() needed to get call count

	eds.On("Add", mock.Anything, mock.AnythingOfType("*state_synchronization.ExecutionData")).
		Return(flow.ZeroID, nil, nil)

	noop := module.NoopReadyDoneAware{}
	eds.On("Ready").Return(func() <-chan struct{} { return noop.Ready() })

	return eds
}

func (suite *ExecutionDataRequesterSuite) mockProtocolState(blocksByHeight map[uint64]*flow.Block) *statemock.State {
	state := new(statemock.State)

	suite.mockSnapshot = new(mockSnapshot)
	suite.mockSnapshot.set(blocksByHeight[0].Header, nil) // genesis block

	state.On("Sealed").Return(suite.mockSnapshot).Maybe()
	return state
}

// Test cases:
// * bootstrap with halted db does not start
// * halts are handled gracefully
// * pause resume works as expected

// func (suite *ExecutionDataRequesterSuite) TestBootstrapFromExistingStateMidSpork() {
// 	// this is testing the case where a node is stopped for a period of time mid spork

// 	// create status object. update fields. save. then start the node with the existing datastore
// 	ctx := context.Background()
// 	startHeight := uint64(100)

// 	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

// 	// Create the status using a mid-spork start height
// 	s := status.New(suite.datastore, log, startHeight, DefaultMaxCachedEntries, DefaultMaxCachedEntries)

// 	// Add a mock entry, and mark it notified so the state is updated
// 	s.Fetched(&status.BlockEntry{
// 		BlockID: unittest.IdentifierFixture(),
// 		Height:  startHeight,
// 	})
// 	_, ok := s.NextNotification(ctx)
// 	assert.True(suite.T(), ok)

// 	testData := suite.generateTestData(200, map[uint64]testExecutionDataCallback{})
// 	testData.startHeight = startHeight

// 	edr, fd := suite.prepareRequesterTest(testData)
// 	suite.runRequesterTest(edr, fd, testData)
// }

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
			unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {
				suite.db = db

				suite.datastore = dssync.MutexWrap(datastore.NewMapDatastore())
				suite.blobservice = synctest.MockBlobService(blockstore.NewBlockstore(suite.datastore))

				testData := suite.generateTestData(run.blockCount, run.specialBlocks(run.blockCount))
				edr, fd := suite.prepareRequesterTest(testData)
				fetchedExecutionData := suite.runRequesterTest(edr, fd, testData)

				verifyFetchedExecutionData(suite.T(), fetchedExecutionData, testData)

				suite.T().Log("Shutting down test")
			})
		})
	}
}

func (suite *ExecutionDataRequesterSuite) TestRequesterResumesAfterRestart() {
	suite.datastore = dssync.MutexWrap(datastore.NewMapDatastore())
	suite.blobservice = synctest.MockBlobService(blockstore.NewBlockstore(suite.datastore))

	testData := suite.generateTestData(suite.run.blockCount, suite.run.specialBlocks(suite.run.blockCount))

	test := func(stopHeight, resumeHeight uint64) {
		testData.stopHeight = 0
		testData.resumeHeight = 0
		testData.fetchedExecutionData = nil

		unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {
			suite.db = db

			// Process half of the blocks
			edr, fd := suite.prepareRequesterTest(testData)
			testData.stopHeight = testData.startHeight + uint64(suite.run.blockCount)/2
			testData.fetchedExecutionData = suite.runRequesterTest(edr, fd, testData)

			// Stand up a new component using the same datastore, and make sure all remaining
			// blocks are processed
			edr, fd = suite.prepareRequesterTest(testData)
			testData.resumeHeight = testData.stopHeight + 1
			testData.stopHeight = 0
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

func (suite *ExecutionDataRequesterSuite) TestRequesterCatchesUp() {
	unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {
		suite.db = db

		suite.datastore = dssync.MutexWrap(datastore.NewMapDatastore())
		suite.blobservice = synctest.MockBlobService(blockstore.NewBlockstore(suite.datastore))

		testData := suite.generateTestData(5, suite.run.specialBlocks(5))

		// start processing with all seals available
		edr, fd := suite.prepareRequesterTest(testData)
		testData.resumeHeight = testData.endHeight
		fetchedExecutionData := suite.runRequesterTest(edr, fd, testData)

		verifyFetchedExecutionData(suite.T(), fetchedExecutionData, testData)

		suite.T().Log("Shutting down test")
	})
}

func (suite *ExecutionDataRequesterSuite) TestRequesterHalts() {
	unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {
		suite.db = db

		suite.datastore = dssync.MutexWrap(datastore.NewMapDatastore())
		suite.blobservice = synctest.MockBlobService(blockstore.NewBlockstore(suite.datastore))

		testData := suite.generateTestData(suite.run.blockCount, suite.run.specialBlocks(suite.run.blockCount))

		processedNotification := storage.NewConsumerProgress(suite.db, module.ConsumeProgressExecutionDataRequesterNotification)
		processedNotification.InitProcessedIndex(testData.startHeight - 1)

		s := status.New(
			zerolog.New(os.Stdout).With().Timestamp().Logger(),
			requester.DefaultMaxCachedEntries,
			processedNotification,
		)
		err := s.Load()
		require.NoError(suite.T(), err)

		haltErr := &status.RequesterHaltedError{
			ExecutionDataID: unittest.IdentifierFixture(),
			BlockID:         unittest.IdentifierFixture(),
			Height:          testData.startHeight,
			Err:             err,
		}

		s.Halt(haltErr)

		// start processing with all seals available
		edr, _ := suite.prepareRequesterTest(testData)
		testData.resumeHeight = testData.endHeight
		suite.runRequesterTestHalted(edr, testData)

		suite.T().Log("Shutting down test")
	})
}

// TestRequesterPausesAndResumes tests that the requester pauses when it downloads maxSearchAhead
// blocks beyond the last processed block, and resumes when it catches up.
func (suite *ExecutionDataRequesterSuite) TestRequesterPausesAndResumes() {
	unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {
		suite.db = db

		pauseHeight := uint64(10)
		maxSearchAhead := uint64(50)

		// Downloads will succeed immediately for all blocks except pauseHeight, which will hang
		// until the pause channel is closed.
		pause, generate := generatePauseResume(pauseHeight)

		testData := suite.generateTestData(suite.run.blockCount, generate(suite.run.blockCount))
		testData.maxSearchAhead = maxSearchAhead
		testData.waitTimeout = time.Second * 10

		// calculate the expected number of blocks that should be downloaded before resuming
		expectedDownloads := maxSearchAhead + pauseHeight - 1

		edr, fd := suite.prepareRequesterTest(testData)
		fetchedExecutionData := suite.runRequesterTestPauseResume(edr, fd, testData, int(expectedDownloads), func() { close(pause) })

		verifyFetchedExecutionData(suite.T(), fetchedExecutionData, testData)

		suite.T().Log("Shutting down test")
	})
}

func generateBlocksWithSomeMissed(blockCount int) map[uint64]testExecutionDataCallback {
	// every third block fails to download 3 times before succeeding
	missing := map[uint64]testExecutionDataCallback{}

	for i := uint64(0); i < uint64(blockCount); i++ {
		if i%5 > 0 {
			continue
		}

		failures := rand.Intn(5)
		attempts := 0
		missing[i] = func(ed *state_synchronization.ExecutionData) (*state_synchronization.ExecutionData, error) {
			if attempts < failures { // this func is run twice for every attempt by the mock (once for ExecutionData one for errors)
				attempts++
				// This should fail the first n fetch attempts
				time.Sleep(time.Duration(rand.Intn(25)) * time.Millisecond)
				return nil, &state_synchronization.BlobNotFoundError{}
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

		delays[i] = func(ed *state_synchronization.ExecutionData) (*state_synchronization.ExecutionData, error) {
			time.Sleep(time.Duration(rand.Intn(25)) * time.Millisecond)
			return ed, nil
		}
	}

	return delays
}

func generatePauseResume(pauseHeight uint64) (chan struct{}, specialBlockGenerator) {
	pause := make(chan struct{})

	blocks := map[uint64]testExecutionDataCallback{}
	blocks[pauseHeight] = func(ed *state_synchronization.ExecutionData) (*state_synchronization.ExecutionData, error) {
		<-pause
		return ed, nil
	}

	return pause, func(int) map[uint64]testExecutionDataCallback {
		return blocks
	}
}

func (suite *ExecutionDataRequesterSuite) runRequesterTestHalted(edr state_synchronization.ExecutionDataRequester, cfg *fetchTestRun) receivedExecutionData {
	// make sure test helper goroutines are cleaned up
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)
	go irrecoverableNotExpected(suite.T(), ctx, errChan)

	edr.AddOnExecutionDataFetchedConsumer(func(ed *state_synchronization.ExecutionData) {
		assert.Fail(suite.T(), "should not have received any execution data")
	})

	edr.Start(signalerCtx)

	// requester should never become ready
	unittest.RequireNeverClosedWithin(suite.T(), edr.Ready(), 100*time.Millisecond, "requester shutdown unexpectedly")

	cancel()
	<-edr.Done()

	return nil
}

func (suite *ExecutionDataRequesterSuite) runRequesterTestPauseResume(edr state_synchronization.ExecutionDataRequester, finalizationDistributor *pubsub.FinalizationDistributor, cfg *fetchTestRun, expectedDownloads int, resume func()) receivedExecutionData {
	// make sure test helper goroutines are cleaned up
	ctx, cancel := context.WithCancel(context.Background())

	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)
	go irrecoverableNotExpected(suite.T(), ctx, errChan)

	testDone := make(chan struct{})
	fetchedExecutionData := cfg.FetchedExecutionData()

	// collect all execution data notifications
	edr.AddOnExecutionDataFetchedConsumer(suite.consumeExecutionDataNotifications(cfg, func() { close(testDone) }, fetchedExecutionData))

	edr.Start(signalerCtx)
	<-edr.Ready()

	// Send all blocks through finalizationDistributor
	suite.finalizeBlocks(cfg, finalizationDistributor)

	// requester should pause downloads until resume is called, so testDone should not be closed
	unittest.RequireNeverClosedWithin(suite.T(), testDone, 500*time.Millisecond, "finished unexpectedly")

	// confirm the expected number of downloads were attempted
	suite.eds.AssertNumberOfCalls(suite.T(), "Get", expectedDownloads)

	suite.T().Log("Resuming")
	resume()

	// Pause until we've received all of the expected notifications
	unittest.RequireCloseBefore(suite.T(), testDone, cfg.waitTimeout, "timed out waiting for notifications")
	suite.T().Log("All notifications received")

	cancel()
	<-edr.Done()

	return fetchedExecutionData
}

func (suite *ExecutionDataRequesterSuite) prepareRequesterTest(cfg *fetchTestRun) (state_synchronization.ExecutionDataRequester, *pubsub.FinalizationDistributor) {
	headers := synctest.MockBlockHeaderStorage(synctest.WithByID(cfg.blocksByID), synctest.WithByHeight(cfg.blocksByHeight))
	results := synctest.MockResultsStorage(synctest.WithByBlockID(cfg.resultsByID))
	state := suite.mockProtocolState(cfg.blocksByHeight)

	suite.eds = mockExecutionDataService(cfg.executionDataEntries)

	finalizationDistributor := pubsub.NewFinalizationDistributor()
	processedHeight := storage.NewConsumerProgress(suite.db, module.ConsumeProgressExecutionDataRequesterBlockHeight)
	processedNotification := storage.NewConsumerProgress(suite.db, module.ConsumeProgressExecutionDataRequesterNotification)

	edr, err := requester.New(
		zerolog.New(os.Stdout).With().Timestamp().Logger(),
		metrics.NewNoopCollector(),
		suite.datastore,
		suite.blobservice,
		suite.eds,
		processedHeight,
		processedNotification,
		state,
		headers,
		results,
		requester.ExecutionDataConfig{
			StartBlockHeight: cfg.startHeight,
			MaxCachedEntries: cfg.maxCachedEntries,
			MaxSearchAhead:   cfg.maxSearchAhead,
			FetchTimeout:     cfg.fetchTimeout,
			RetryDelay:       cfg.retryDelay,
			MaxRetryDelay:    cfg.maxRetryDelay,
			CheckEnabled:     cfg.checkEnabled,
		},
	)
	assert.NoError(suite.T(), err)

	return edr, finalizationDistributor
}

func (suite *ExecutionDataRequesterSuite) runRequesterTest(edr state_synchronization.ExecutionDataRequester, finalizationDistributor *pubsub.FinalizationDistributor, cfg *fetchTestRun) receivedExecutionData {
	// make sure test helper goroutines are cleaned up
	ctx, cancel := context.WithCancel(context.Background())

	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)
	go irrecoverableNotExpected(suite.T(), ctx, errChan)

	// wait for all notifications
	testDone := make(chan struct{})

	fetchedExecutionData := cfg.FetchedExecutionData()

	// collect all execution data notifications
	edr.AddOnExecutionDataFetchedConsumer(suite.consumeExecutionDataNotifications(cfg, func() { close(testDone) }, fetchedExecutionData))

	edr.Start(signalerCtx)
	<-edr.Ready()

	// Send blocks through finalizationDistributor
	suite.finalizeBlocks(cfg, finalizationDistributor)

	// Pause until we've received all of the expected notifications
	unittest.RequireCloseBefore(suite.T(), testDone, cfg.waitTimeout, "timed out waiting for notifications")
	suite.T().Log("All notifications received")

	cancel()
	<-edr.Done()

	return fetchedExecutionData
}

func (suite *ExecutionDataRequesterSuite) consumeExecutionDataNotifications(cfg *fetchTestRun, done func(), fetchedExecutionData map[flow.Identifier]*state_synchronization.ExecutionData) func(ed *state_synchronization.ExecutionData) {
	return func(ed *state_synchronization.ExecutionData) {
		if _, has := fetchedExecutionData[ed.BlockID]; has {
			suite.T().Errorf("duplicate execution data for block %s", ed.BlockID)
			return
		}

		fetchedExecutionData[ed.BlockID] = ed
		suite.T().Logf("notified of execution data for block %v height %d (%d/%d)", ed.BlockID, cfg.blocksByID[ed.BlockID].Header.Height, len(fetchedExecutionData), cfg.sealedCount)

		if cfg.IsLastSeal(ed.BlockID) {
			done()
		}
	}
}

func (suite *ExecutionDataRequesterSuite) finalizeBlocks(cfg *fetchTestRun, finalizationDistributor *pubsub.FinalizationDistributor) {
	for i := cfg.StartHeight(); i <= cfg.endHeight; i++ {
		b := cfg.blocksByHeight[i]

		suite.T().Log(">>>> Finalizing block", b.ID(), b.Header.Height)

		if len(b.Payload.Seals) > 0 {
			seal := b.Payload.Seals[0]
			sealedHeader := cfg.blocksByID[seal.BlockID].Header

			suite.mockSnapshot.set(sealedHeader, nil)
			suite.T().Log(">>>> Sealing block", sealedHeader.ID(), sealedHeader.Height)
		}

		finalizationDistributor.OnFinalizedBlock(&model.Block{}) // actual block is unused

		if cfg.stopHeight == i {
			break
		}
	}
}

type receivedExecutionData map[flow.Identifier]*state_synchronization.ExecutionData
type fetchTestRun struct {
	sealedCount           int
	startHeight           uint64
	endHeight             uint64
	blocksByHeight        map[uint64]*flow.Block
	blocksByID            map[flow.Identifier]*flow.Block
	resultsByID           map[flow.Identifier]*flow.ExecutionResult
	executionDataByID     map[flow.Identifier]*state_synchronization.ExecutionData
	executionDataEntries  map[flow.Identifier]*testExecutionDataServiceEntry
	expectedIrrecoverable error

	stopHeight           uint64
	resumeHeight         uint64
	fetchedExecutionData map[flow.Identifier]*state_synchronization.ExecutionData
	waitTimeout          time.Duration

	maxCachedEntries uint64
	maxSearchAhead   uint64
	fetchTimeout     time.Duration
	retryDelay       time.Duration
	maxRetryDelay    time.Duration
	checkEnabled     bool
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

func (suite *ExecutionDataRequesterSuite) generateTestData(blockCount int, missingHeightFuncs map[uint64]testExecutionDataCallback) *fetchTestRun {
	edsEntries := map[flow.Identifier]*testExecutionDataServiceEntry{}
	blocksByHeight := map[uint64]*flow.Block{}
	blocksByID := map[flow.Identifier]*flow.Block{}
	resultsByID := map[flow.Identifier]*flow.ExecutionResult{}
	executionDataByID := map[flow.Identifier]*state_synchronization.ExecutionData{}

	sealedCount := blockCount - 4 // seals for blocks 1-96
	firstSeal := blockCount - sealedCount

	// genesis is block 0, we start syncing from block 1
	startHeight := uint64(1)
	endHeight := uint64(blockCount) - 1

	var previousBlock *flow.Block
	var previousResult *flow.ExecutionResult
	for i := 0; i < blockCount; i++ {
		var seals []*flow.Header

		if i >= firstSeal {
			seals = []*flow.Header{
				blocksByHeight[uint64(i-firstSeal+1)].Header, // block 0 doesn't get sealed (it's pre-sealed in the genesis state)
			}
			suite.T().Logf("block %d has seals for %d", i, seals[0].Height)
		}

		height := uint64(i)
		block := buildBlock(height, previousBlock, seals)

		ed := synctest.ExecutionDataFixture(block.ID())
		cid := unittest.IdentifierFixture()

		result := buildResult(block, cid, previousResult)

		blocksByHeight[height] = block
		blocksByID[block.ID()] = block
		resultsByID[block.ID()] = result

		// ignore all the data we don't need to verify the test
		if i > 0 && i <= sealedCount {
			executionDataByID[block.ID()] = ed
			edsEntries[cid] = &testExecutionDataServiceEntry{ExecutionData: ed}
			if fn, missing := missingHeightFuncs[height]; missing {
				edsEntries[cid].fn = fn
			}
		}

		previousBlock = block
		previousResult = result
	}

	return &fetchTestRun{
		sealedCount:          sealedCount,
		startHeight:          startHeight,
		endHeight:            endHeight,
		blocksByHeight:       blocksByHeight,
		blocksByID:           blocksByID,
		resultsByID:          resultsByID,
		executionDataByID:    executionDataByID,
		executionDataEntries: edsEntries,
		waitTimeout:          time.Second * 5,

		maxCachedEntries: requester.DefaultMaxCachedEntries,
		maxSearchAhead:   requester.DefaultMaxSearchAhead,
		fetchTimeout:     requester.DefaultFetchTimeout,
		retryDelay:       1 * time.Millisecond,
		maxRetryDelay:    15 * time.Millisecond,
		checkEnabled:     false,
	}
}

func buildBlock(height uint64, parent *flow.Block, seals []*flow.Header) *flow.Block {
	if parent == nil {
		return unittest.GenesisFixture()
	}

	if len(seals) == 0 {
		return unittest.BlockWithParentFixture(parent.Header)
	}

	return unittest.BlockWithParentAndSeals(parent.Header, seals)
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

func irrecoverableExpected(t *testing.T, ctx context.Context, errChan <-chan error, expectedErr error) {
	select {
	case <-ctx.Done():
		t.Errorf("expected irrecoverable error, but got none")
	case err := <-errChan:
		assert.ErrorIs(t, err, expectedErr)
	}
}

func irrecoverableNotExpected(t *testing.T, ctx context.Context, errChan <-chan error) {
	select {
	case <-ctx.Done():
		return
	case err := <-errChan:
		assert.NoError(t, err, "unexpected irrecoverable error")
	}
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
func (m *mockSnapshot) Identities(selector flow.IdentityFilter) (flow.IdentityList, error) {
	return nil, nil
}
func (m *mockSnapshot) Identity(nodeID flow.Identifier) (*flow.Identity, error) { return nil, nil }
func (m *mockSnapshot) SealedResult() (*flow.ExecutionResult, *flow.Seal, error) {
	return nil, nil, nil
}
func (m *mockSnapshot) Commit() (flow.StateCommitment, error)         { return flow.DummyStateCommitment, nil }
func (m *mockSnapshot) SealingSegment() (*flow.SealingSegment, error) { return nil, nil }
func (m *mockSnapshot) Descendants() ([]flow.Identifier, error)       { return nil, nil }
func (m *mockSnapshot) ValidDescendants() ([]flow.Identifier, error)  { return nil, nil }
func (m *mockSnapshot) RandomSource() ([]byte, error)                 { return nil, nil }
func (m *mockSnapshot) Phase() (flow.EpochPhase, error)               { return flow.EpochPhaseUndefined, nil }
func (m *mockSnapshot) Epochs() protocol.EpochQuery                   { return nil }
func (m *mockSnapshot) Params() protocol.GlobalParams                 { return nil }
