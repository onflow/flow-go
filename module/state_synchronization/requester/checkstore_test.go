package requester_test

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	"github.com/onflow/flow-go/module/state_synchronization/requester"
	"github.com/onflow/flow-go/module/state_synchronization/requester/jobs"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/network/compressor"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ExecutionDatastoreCheckerSuite struct {
	suite.Suite

	datastore   datastore.Batching
	blobservice *mocknetwork.BlobService
	eds         *syncmock.ExecutionDataService
	headers     *storagemock.Headers
	results     *storagemock.ExecutionResults
	blockCount  int
	lastHeight  uint64
	cfg         *fetchTestRun
}

func TestExecutionDatastoreCheckerSuite(t *testing.T) {
	t.Parallel()
	rand.Seed(time.Now().UnixMilli())
	suite.Run(t, new(ExecutionDatastoreCheckerSuite))
}

func (suite *ExecutionDatastoreCheckerSuite) SetupTest() {
	suite.blockCount = 100
	suite.datastore = dssync.MutexWrap(datastore.NewMapDatastore())
	suite.blobservice = synctest.MockBlobService(blockstore.NewBlockstore(suite.datastore))
	suite.cfg = suite.generateTestData(suite.blockCount)

	suite.reset()
}

func (suite *ExecutionDatastoreCheckerSuite) reset() {
	suite.lastHeight = suite.cfg.startHeight + uint64(suite.cfg.sealedCount) - 1

	suite.headers = synctest.MockBlockHeaderStorage(synctest.WithByHeight(suite.cfg.blocksByHeight))
	suite.results = synctest.MockResultsStorage(synctest.WithByBlockID(suite.cfg.resultsByID))
}

func (suite *ExecutionDatastoreCheckerSuite) generateTestData(blockCount int) *fetchTestRun {
	edsEntries := map[flow.Identifier]*testExecutionDataServiceEntry{}
	blocksByHeight := map[uint64]*flow.Block{}
	blocksByID := map[flow.Identifier]*flow.Block{}
	resultsByID := map[flow.Identifier]*flow.ExecutionResult{}
	executionDataByID := map[flow.Identifier]*state_synchronization.ExecutionData{}
	executionDataIDByBlockID := map[flow.Identifier]flow.Identifier{}

	sealedCount := blockCount - 4 // seals for blocks 1-96
	firstSeal := blockCount - sealedCount

	// genesis is block 0, we start syncing from block 1
	startHeight := uint64(1)
	endHeight := uint64(blockCount) - 1

	// instantiate ExecutionDataService to generate correct CIDs
	eds := state_synchronization.NewExecutionDataService(
		new(cbor.Codec),
		compressor.NewLz4Compressor(),
		suite.blobservice,
		metrics.NewNoopCollector(),
		zerolog.New(os.Stdout).With().Timestamp().Logger(),
	)

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

		cid, _, err := eds.Add(context.Background(), ed)
		require.NoError(suite.T(), err)

		result := buildResult(block, cid, previousResult)

		blocksByHeight[height] = block
		blocksByID[block.ID()] = block
		resultsByID[block.ID()] = result

		// ignore all the data we don't need to verify the test
		if i > 0 && i <= sealedCount {
			executionDataByID[block.ID()] = ed
			edsEntries[cid] = &testExecutionDataServiceEntry{ExecutionData: ed}
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
		resultsByID:              resultsByID,
		executionDataByID:        executionDataByID,
		executionDataEntries:     edsEntries,
		executionDataIDByBlockID: executionDataIDByBlockID,
		waitTimeout:              time.Second * 5,
	}
}

// TestCheckDatastoreWithLocalEDS tests the happy path using a real ExecutionDataService configured
// to use the local datastore.
func (suite *ExecutionDatastoreCheckerSuite) TestCheckDatastoreWithLocalEDS() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)
	go irrecoverableNotExpected(suite.T(), ctx, errChan)

	downloadCalls := 0
	download := func(ctx irrecoverable.SignalerContext, blockID flow.Identifier, height uint64) error {
		downloadCalls++
		return nil
	}

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	eds := requester.LocalExecutionDataService(signalerCtx, suite.datastore, logger)

	checker := requester.NewDatastoreChecker(
		logger,
		suite.blobservice,
		eds,
		suite.headers,
		suite.results,
		suite.cfg.startHeight,
		download,
	)

	<-eds.Ready()

	run := func() {
		err := checker.Run(signalerCtx, suite.lastHeight)
		assert.NoError(suite.T(), err, "unexpected error from checker")
	}

	unittest.RequireReturnsBefore(suite.T(), run, suite.cfg.waitTimeout, "timed out waiting for requester to shutdown")
	assert.Equal(suite.T(), 0, downloadCalls)

	cancel()
	<-eds.Done()
}

// TestCheckDatastore tests the checker scenarios where the blocks are checked
func (suite *ExecutionDatastoreCheckerSuite) TestCheckDatastore() {
	invalidCids := suite.generateInvalidCids()

	downloadsFunc := func() (map[flow.Identifier]uint64, requester.DownloadExecutionDataFunc) {
		tracker := make(map[flow.Identifier]uint64)
		fun := func(ctx irrecoverable.SignalerContext, blockID flow.Identifier, height uint64) error {
			tracker[blockID] = height
			return nil
		}

		return tracker, fun
	}

	suite.Run("checks valid all block", func() {
		suite.blobservice = new(mocknetwork.BlobService)
		// DeleteBlob should never be called

		suite.eds = new(syncmock.ExecutionDataService)
		suite.eds.On("Check", mock.Anything, mock.AnythingOfType("flow.Identifier")).Return(nil, true).Times(suite.cfg.sealedCount)

		downloads, download := downloadsFunc()

		suite.runTest(download, nil)

		// downloadExecutionData should never be called
		assert.Equal(suite.T(), 0, len(downloads))

		suite.eds.AssertExpectations(suite.T())
	})

	suite.Run("downloads missing blobs", func() {
		suite.blobservice = new(mocknetwork.BlobService)
		// DeleteBlob should never be called

		suite.eds = new(syncmock.ExecutionDataService)
		suite.eds.On("Check", mock.Anything, mock.AnythingOfType("flow.Identifier")).Return(
			mockCheckInvalidCidList(invalidCids, blockservice.ErrNotFound),
			mockCheckError(invalidCids),
		).Times(suite.cfg.sealedCount)

		downloads, download := downloadsFunc()

		suite.runTest(download, nil)

		assert.Equal(suite.T(), len(invalidCids), len(downloads), "expected to download all missing blocks")
		for _, entry := range invalidCids {
			assert.Equal(suite.T(), entry.Height, downloads[entry.BlockID], "expected to download block %s at height %d", entry.BlockID, entry.Height)
		}

		suite.blobservice.AssertExpectations(suite.T())
		suite.eds.AssertExpectations(suite.T())
	})

	suite.Run("deletes and downloads corrupt blobs", func() {
		suite.blobservice = new(mocknetwork.BlobService)
		suite.blobservice.On("DeleteBlob", mock.Anything, mock.AnythingOfType("cid.Cid")).Return(nil).Times(len(invalidCids))

		suite.eds = new(syncmock.ExecutionDataService)
		suite.eds.On("Check", mock.Anything, mock.AnythingOfType("flow.Identifier")).Return(
			mockCheckInvalidCidList(invalidCids, blockstore.ErrHashMismatch),
			mockCheckError(invalidCids),
		).Times(suite.cfg.sealedCount)

		downloads, download := downloadsFunc()

		suite.runTest(download, nil)

		assert.Equal(suite.T(), len(invalidCids), len(downloads), "expected to download all corrupt blocks")
		for _, entry := range invalidCids {
			assert.Equal(suite.T(), entry.Height, downloads[entry.BlockID], "expected to download block %s at height %d", entry.BlockID, entry.Height)
		}

		suite.blobservice.AssertExpectations(suite.T())
		suite.eds.AssertExpectations(suite.T())
	})

	failureOffset := 5
	invalidCids = map[flow.Identifier]*jobs.BlockEntry{}
	suite.addInvalidEntry(invalidCids, suite.cfg.startHeight+uint64(failureOffset)-1)

	suite.Run("handles unexpected errors", func() {
		suite.blobservice = new(mocknetwork.BlobService)
		// DeleteBlob should never be called

		suite.eds = new(syncmock.ExecutionDataService)
		suite.eds.On("Check", mock.Anything, mock.AnythingOfType("flow.Identifier")).Return(
			mockCheckInvalidCidList(invalidCids, errors.New("unexpected error")),
			mockCheckError(invalidCids),
		).Times(failureOffset)

		downloads, download := downloadsFunc()

		suite.runTest(download, errors.New("unexpected error"))

		// downloadExecutionData should never be called
		assert.Equal(suite.T(), 0, len(downloads))

		suite.eds.AssertExpectations(suite.T())
	})

	suite.Run("handles error from download", func() {
		suite.blobservice = new(mocknetwork.BlobService)
		// DeleteBlob should never be called

		suite.eds = new(syncmock.ExecutionDataService)
		suite.eds.On("Check", mock.Anything, mock.AnythingOfType("flow.Identifier")).Return(
			mockCheckInvalidCidList(invalidCids, blockservice.ErrNotFound),
			mockCheckError(invalidCids),
		).Times(failureOffset)

		downloadErr := errors.New("download error")
		downloadCount := 0
		download := func(_ irrecoverable.SignalerContext, _ flow.Identifier, _ uint64) error {
			downloadCount++
			return downloadErr
		}

		suite.runTest(download, downloadErr)

		assert.Equal(suite.T(), 1, downloadCount, "expected 1 download")

		suite.eds.AssertExpectations(suite.T())
	})

	suite.Run("handles error from delete blobs", func() {
		deleteErr := errors.New("delete error")
		suite.blobservice = new(mocknetwork.BlobService)
		suite.blobservice.On("DeleteBlob", mock.Anything, mock.AnythingOfType("cid.Cid")).Return(deleteErr).Times(1)

		suite.eds = new(syncmock.ExecutionDataService)
		suite.eds.On("Check", mock.Anything, mock.AnythingOfType("flow.Identifier")).Return(
			mockCheckInvalidCidList(invalidCids, blockstore.ErrHashMismatch),
			mockCheckError(invalidCids),
		).Times(failureOffset)

		downloads, download := downloadsFunc()

		suite.runTest(download, deleteErr)

		assert.Equal(suite.T(), 0, len(downloads), "expected no downloads")

		suite.blobservice.AssertExpectations(suite.T())
		suite.eds.AssertExpectations(suite.T())
	})
}

// TestCheckDatastoreHandlesErrors tests the scenarios where the checker fails to check the datastore
func (suite *ExecutionDatastoreCheckerSuite) TestCheckDatastoreHandlesErrors() {
	suite.Run("skips running when no blocks in datastore", func() {
		suite.blobservice = new(mocknetwork.BlobService)
		// DeleteBlob should never be called

		suite.eds = new(syncmock.ExecutionDataService)
		// Check should never be called

		downloadCount := 0
		download := func(ctx irrecoverable.SignalerContext, blockID flow.Identifier, height uint64) error {
			downloadCount++
			return nil
		}

		suite.lastHeight = suite.cfg.startHeight
		suite.runTest(download, nil)

		// downloadExecutionData should never be called
		assert.Equal(suite.T(), 0, downloadCount)
	})

	suite.Run("never starts running when context cancelled", func() {
		suite.reset()

		suite.blobservice = new(mocknetwork.BlobService)
		// DeleteBlob should never be called

		suite.eds = new(syncmock.ExecutionDataService)
		// Check should never be called

		downloadCount := 0
		download := func(ctx irrecoverable.SignalerContext, blockID flow.Identifier, height uint64) error {
			downloadCount++
			return nil
		}

		testCtx, testCancel := context.WithCancel(context.Background())
		defer testCancel()

		ctx, cancel := context.WithCancel(testCtx)
		signalerCtx, errChan := irrecoverable.WithSignaler(ctx)
		go irrecoverableNotExpected(suite.T(), testCtx, errChan)

		checker := suite.mockChecker(download)

		// context cancelled before checking, should stop immediately
		run := func() {
			cancel()
			err := checker.Run(signalerCtx, suite.lastHeight)
			assert.NoError(suite.T(), err, "unexpected error from checker")
		}

		unittest.RequireReturnsBefore(suite.T(), run, suite.cfg.waitTimeout, "timed out waiting for requester to shutdown")

		// downloadExecutionData should never be called
		assert.Equal(suite.T(), 0, downloadCount)
	})

	suite.Run("stops running when context cancelled", func() {
		suite.reset()

		suite.blobservice = new(mocknetwork.BlobService)
		// DeleteBlob should never be called

		testCtx, testCancel := context.WithCancel(context.Background())
		defer testCancel()

		ctx, cancel := context.WithCancel(testCtx)

		suite.eds = new(syncmock.ExecutionDataService)
		suite.eds.On("Check", mock.Anything, mock.AnythingOfType("flow.Identifier")).Return(
			func(_ context.Context, _ flow.Identifier) []state_synchronization.InvalidCid {
				// context cancelled after first check, should stop checking
				cancel()
				return nil
			},
			true,
		).Times(1)

		downloadCount := 0
		download := func(ctx irrecoverable.SignalerContext, blockID flow.Identifier, height uint64) error {
			downloadCount++
			return nil
		}

		signalerCtx, errChan := irrecoverable.WithSignaler(ctx)
		go irrecoverableNotExpected(suite.T(), testCtx, errChan)

		checker := suite.mockChecker(download)

		run := func() {
			err := checker.Run(signalerCtx, suite.lastHeight)
			assert.NoError(suite.T(), err, "unexpected error from checker")
		}

		unittest.RequireReturnsBefore(suite.T(), run, suite.cfg.waitTimeout, "timed out waiting for requester to shutdown")

		// downloadExecutionData should never be called
		assert.Equal(suite.T(), 0, downloadCount)

		suite.eds.AssertExpectations(suite.T())
	})

	suite.Run("error getting headers returns immediately", func() {
		suite.reset()

		suite.blobservice = new(mocknetwork.BlobService)
		// DeleteBlob should never be called

		suite.eds = new(syncmock.ExecutionDataService)
		// Check should never be called

		downloadCount := 0
		download := func(ctx irrecoverable.SignalerContext, blockID flow.Identifier, height uint64) error {
			downloadCount++
			return nil
		}

		// headers is empty, so it will return storage.ErrNotFound
		suite.headers = synctest.MockBlockHeaderStorage(synctest.WithByHeight(map[uint64]*flow.Block{}))
		suite.runTest(download, storage.ErrNotFound)

		// downloadExecutionData should never be called
		assert.Equal(suite.T(), 0, downloadCount)
	})

	suite.Run("error getting results returns immediately", func() {
		suite.reset()

		suite.blobservice = new(mocknetwork.BlobService)
		// DeleteBlob should never be called

		suite.eds = new(syncmock.ExecutionDataService)
		// Check should never be called

		downloadCount := 0
		download := func(ctx irrecoverable.SignalerContext, blockID flow.Identifier, height uint64) error {
			downloadCount++
			return nil
		}

		// results is empty, so it will return storage.ErrNotFound
		suite.results = synctest.MockResultsStorage(synctest.WithByBlockID(map[flow.Identifier]*flow.ExecutionResult{}))
		suite.runTest(download, storage.ErrNotFound)

		// downloadExecutionData should never be called
		assert.Equal(suite.T(), 0, downloadCount)
	})
}

func (suite *ExecutionDatastoreCheckerSuite) runTest(download requester.DownloadExecutionDataFunc, expectedErr error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)
	go irrecoverableNotExpected(suite.T(), ctx, errChan)

	checker := suite.mockChecker(download)

	run := func() {
		err := checker.Run(signalerCtx, suite.lastHeight)
		if expectedErr == nil {
			assert.NoError(suite.T(), err, "unexpected error from checker")
		} else {
			assert.Contains(suite.T(), err.Error(), expectedErr.Error(), "expected error from checker")
		}
	}

	unittest.RequireReturnsBefore(suite.T(), run, suite.cfg.waitTimeout, "timed out waiting for requester to shutdown")
}

func (suite *ExecutionDatastoreCheckerSuite) addInvalidEntry(invalidCids map[flow.Identifier]*jobs.BlockEntry, height uint64) {
	block := suite.cfg.blocksByHeight[height]
	rootID := suite.cfg.executionDataIDByBlockID[block.ID()]

	invalidCids[rootID] = &jobs.BlockEntry{
		BlockID: block.ID(),
		Height:  height,
	}
}

func (suite *ExecutionDatastoreCheckerSuite) generateInvalidCids() map[flow.Identifier]*jobs.BlockEntry {
	invalidCids := map[flow.Identifier]*jobs.BlockEntry{}
	for i := suite.cfg.startHeight; i <= suite.cfg.endHeight; i++ {
		if i%5 != 0 {
			continue
		}

		suite.addInvalidEntry(invalidCids, i)
	}
	return invalidCids
}

func (suite *ExecutionDatastoreCheckerSuite) mockChecker(download requester.DownloadExecutionDataFunc) requester.DatastoreChecker {
	return requester.NewDatastoreChecker(
		zerolog.New(os.Stdout).With().Timestamp().Logger(),
		suite.blobservice,
		suite.eds,
		suite.headers,
		suite.results,
		suite.cfg.startHeight,
		download,
	)
}

func mockCheckInvalidCidList(cids map[flow.Identifier]*jobs.BlockEntry, err error) func(context.Context, flow.Identifier) []state_synchronization.InvalidCid {
	return func(_ context.Context, rootID flow.Identifier) []state_synchronization.InvalidCid {
		if _, has := cids[rootID]; !has {
			return nil
		}

		return []state_synchronization.InvalidCid{{
			Cid: flow.IdToCid(rootID),
			Err: err,
		}}
	}
}

func mockCheckError(cids map[flow.Identifier]*jobs.BlockEntry) func(context.Context, flow.Identifier) bool {
	return func(_ context.Context, rootID flow.Identifier) bool {
		if _, has := cids[rootID]; !has {
			return true
		}

		return false
	}
}
