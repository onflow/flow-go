package requester

// putting tests directly into requester package so the suite can inject data directly onto the
// requesters queues

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/network/mocknetwork"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type ExecutionDataRequesterSuite struct {
	suite.Suite

	blobservice *mocknetwork.BlobService
	datastore   datastore.Batching

	cleanup func()
}

func TestExecutionDataRequesterSuite(t *testing.T) {
	t.Parallel()
	rand.Seed(time.Now().UnixMilli())
	suite.Run(t, new(ExecutionDataRequesterSuite))
}

func (suite *ExecutionDataRequesterSuite) SetupTest() {
	suite.datastore = dssync.MutexWrap(datastore.NewMapDatastore())
	suite.blobservice = mockBlobService(blockstore.NewBlockstore(suite.datastore))
}

func (suite *ExecutionDataRequesterSuite) TearDownTest() {
	if suite.cleanup != nil {
		suite.cleanup()
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
	)

	eds.On("Add", mock.Anything, mock.AnythingOfType("*state_synchronization.ExecutionData")).
		Return(flow.ZeroID, nil, nil)

	noop := module.NoopReadyDoneAware{}
	eds.On("Ready").Return(func() <-chan struct{} { return noop.Ready() })

	return eds
}

func mockBlobService(bs blockstore.Blockstore) *mocknetwork.BlobService {
	bex := new(mocknetwork.BlobService)

	bex.On("GetBlobs", mock.Anything, mock.AnythingOfType("[]cid.Cid")).
		Return(func(ctx context.Context, cids []cid.Cid) <-chan blobs.Blob {
			ch := make(chan blobs.Blob)

			var wg sync.WaitGroup
			wg.Add(len(cids))

			for _, c := range cids {
				c := c
				go func() {
					defer wg.Done()

					blob, err := bs.Get(ctx, c)

					if err != nil {
						// In the real implementation, Bitswap would keep trying to get the blob from
						// the network indefinitely, sending requests to more and more peers until it
						// eventually finds the blob, or the context is canceled. Here, we know that
						// if the blob is not already in the blobstore, then we will never appear, so
						// we just wait for the context to be canceled.
						<-ctx.Done()

						return
					}

					ch <- blob
				}()
			}

			go func() {
				wg.Wait()
				close(ch)
			}()

			return ch
		})

	bex.On("AddBlobs", mock.Anything, mock.AnythingOfType("[]blocks.Block")).Return(bs.PutMany)

	noop := module.NoopReadyDoneAware{}
	bex.On("Ready").Return(func() <-chan struct{} { return noop.Ready() })

	return bex
}

func mockBlocksStorage(blocksByID map[flow.Identifier]*flow.Block, blocksByHeight map[uint64]*flow.Block) *storagemock.Blocks {
	blocks := new(storagemock.Blocks)

	blocks.On("ByID", mock.AnythingOfType("flow.Identifier")).Return(
		func(blockID flow.Identifier) *flow.Block {
			return blocksByID[blockID]
		},
		func(blockID flow.Identifier) error {
			if _, has := blocksByID[blockID]; !has {
				return fmt.Errorf("block %s not found", blockID)
			}
			return nil
		},
	)

	blocks.On("ByHeight", mock.AnythingOfType("uint64")).Return(
		func(height uint64) *flow.Block {
			return blocksByHeight[height]
		},
		func(height uint64) error {
			if _, has := blocksByHeight[height]; !has {
				return fmt.Errorf("block %d not found", height)
			}
			return nil
		},
	)

	return blocks
}

func mockResultsStorage(resultsByID map[flow.Identifier]*flow.ExecutionResult) *storagemock.ExecutionResults {
	results := new(storagemock.ExecutionResults)

	results.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(
		func(blockID flow.Identifier) *flow.ExecutionResult {
			return resultsByID[blockID]
		},
		func(blockID flow.Identifier) error {
			if _, has := resultsByID[blockID]; !has {
				return fmt.Errorf("result %s not found", blockID)
			}
			return nil
		},
	)

	return results
}

// Test cases:
// * bootstrap with an empty db sets configuration correctly at beginning of spork
// * bootstrap with an empty db sets configuration correctly mid spork
// * bootstrap with a non-empty db sets configuration correctly
// * bootstrap with halted db does not start
// * catch up after finalization queue overflow
// * catch up after fetch [retry] queue overflow
// * out of order blocks are notified in order
// * blocks not in queue are refetched
// * halts are handled gracefully

func (suite *ExecutionDataRequesterSuite) TestBootstrapFromEmptyStateAtSporkStart() {

	// this is the standard case, we don't need an extra test

}

func (suite *ExecutionDataRequesterSuite) TestBootstrapFromEmptyStateMidSpork() {

	// use normal test, but the first block to finalize is midway through the list

}

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

	tests := []struct {
		name          string
		blockCount    int
		specialBlocks func(int) map[uint64]testExecutionDataCallback
	}{
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
			testData := suite.generateTestData(run.blockCount, run.specialBlocks(run.blockCount))
			edr, fd := suite.prepareRequesterTest(testData)
			suite.runRequesterTest(edr, fd, testData)
		})
	}
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

func (suite *ExecutionDataRequesterSuite) prepareRequesterTest(cfg *fetchTestRun) (ExecutionDataRequester, *pubsub.FinalizationDistributor) {
	blocks := mockBlocksStorage(cfg.blocksByID, cfg.blocksByHeight)
	results := mockResultsStorage(cfg.resultsByID)

	eds := mockExecutionDataService(cfg.executionDataEntries)

	edr, err := New(
		zerolog.New(os.Stdout).With().Timestamp().Logger(),
		metrics.NewNoopCollector(),
		suite.datastore,
		suite.blobservice,
		eds,
		uint64(0),
		DefaultMaxCachedEntries,
		DefaultMaxSearchAhead,
		blocks,
		results,
		DefaultFetchTimeout,
		false,
	)
	assert.NoError(suite.T(), err)

	finalizationDistributor := pubsub.NewFinalizationDistributor()
	finalizationDistributor.AddOnBlockFinalizedConsumer(edr.OnBlockFinalized)

	return edr, finalizationDistributor
}

func (suite *ExecutionDataRequesterSuite) runRequesterTest(edr ExecutionDataRequester, finalizationDistributor *pubsub.FinalizationDistributor, cfg *fetchTestRun) {
	// make sure test helper goroutines are cleaned up
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)
	go irrecoverableNotExpected(suite.T(), ctx, errChan)

	// setup sync to wait for all notifications
	outstandingBlocks := sync.WaitGroup{}
	outstandingBlocks.Add(cfg.sealedCount)

	fetchedExecutionData := make(map[flow.Identifier]*state_synchronization.ExecutionData, cfg.sealedCount)
	edr.AddOnExecutionDataFetchedConsumer(func(ed *state_synchronization.ExecutionData) {
		if _, has := fetchedExecutionData[ed.BlockID]; has {
			suite.T().Errorf("duplicate execution data for block %s", ed.BlockID)
			return
		}

		fetchedExecutionData[ed.BlockID] = ed
		outstandingBlocks.Done()
		suite.T().Logf("notified of execution data for block %v height %d (%d/%d)", ed.BlockID, cfg.blocksByID[ed.BlockID].Header.Height, len(fetchedExecutionData), cfg.sealedCount)
	})

	edr.Start(signalerCtx)

	<-edr.Ready()

	// Send blocks through finalizationDistributor
	var parentView uint64
	for i := cfg.startHeight; i <= cfg.endHeight; i++ {
		b := cfg.blocksByHeight[i]

		suite.T().Log(">>>> Finalizing block", b.ID(), b.Header.Height)

		block := model.BlockFromFlow(b.Header, parentView)
		finalizationDistributor.OnFinalizedBlock(block)

		parentView = b.Header.View

		// needs a slight delay otherwise it will fill the queue immediately.
		time.Sleep(5 * time.Millisecond)

		// the requester can catch up after receiving a finalization block, so on the last block,
		// put the block directly into the finalized block channel to ensure it's accepted
		if i == cfg.endHeight {
			select {
			case <-ctx.Done():
				suite.T().Error("timed out before sending last block")
			case edr.(*executionDataRequesterImpl).finalizedBlocks <- block.BlockID:
			}
		}
	}

	// Pause until we've received all of the expected notifications
	// TODO: this should honor context timeouts
	outstandingBlocks.Wait()
	suite.T().Log("All notifications received")

	verifyFetchedExecutionData(suite.T(), fetchedExecutionData, cfg)
}

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
}

func (suite *ExecutionDataRequesterSuite) generateTestData(blockCount int, missingHeightFuncs map[uint64]testExecutionDataCallback) *fetchTestRun {
	edsEntries := map[flow.Identifier]*testExecutionDataServiceEntry{}
	blocksByHeight := map[uint64]*flow.Block{}
	blocksByID := map[flow.Identifier]*flow.Block{}
	resultsByID := map[flow.Identifier]*flow.ExecutionResult{}
	executionDataByID := map[flow.Identifier]*state_synchronization.ExecutionData{}

	sealedCount := blockCount - 3
	firstSeal := blockCount - sealedCount + 2 // offset by 2 so the first block can contain multiple seals

	startHeight := uint64(0)
	endHeight := startHeight + uint64(blockCount) - 1

	var previousBlock *flow.Block
	var previousResult *flow.ExecutionResult
	for i := 0; i < blockCount; i++ {
		var seals []*flow.Header

		if i == firstSeal {
			// first block with seals contains 3
			seals = []*flow.Header{
				// intentionally out of order
				blocksByHeight[uint64(i-firstSeal+2)].Header,
				blocksByHeight[uint64(i-firstSeal)].Header,
				blocksByHeight[uint64(i-firstSeal+1)].Header,
			}
			suite.T().Logf("block %d has seals for %d, %d, %d", i, seals[1].Height, seals[2].Height, seals[0].Height)
		} else if i > firstSeal {
			seals = []*flow.Header{
				blocksByHeight[uint64(i-firstSeal+2)].Header,
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
		if i < sealedCount {
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

func verifyFetchedExecutionData(t *testing.T, actual map[flow.Identifier]*state_synchronization.ExecutionData, cfg *fetchTestRun) {
	expected := cfg.executionDataByID
	assert.Len(t, actual, len(expected))

	for i := 0; i < cfg.sealedCount; i++ {
		height := cfg.startHeight + uint64(i)
		block := cfg.blocksByHeight[height]
		blockID := block.ID()

		expectedED := expected[blockID]
		actualED, has := actual[blockID]
		if !has {
			assert.Fail(t, "missing execution data for block %v", blockID)
			continue
		}

		assert.Equal(t, expectedED, actualED, "execution data for block %v doesn't match", blockID)
	}
}
