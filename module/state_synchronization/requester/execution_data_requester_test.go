package requester

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	"github.com/onflow/flow-go/network/compressor"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type ExecutionDataRequesterSuite struct {
	suite.Suite

	state   *protocolmock.State
	blocks  *storagemock.Blocks
	results *storagemock.ExecutionResults

	allBlocks        map[flow.Identifier]*flow.Block
	allResults       map[flow.Identifier]*flow.ExecutionResult
	allExecutionData map[flow.Identifier]*state_synchronization.ExecutionData

	allBlobs map[cid.Cid]bool

	sealedBlockCount int

	ctx         context.Context
	eds         state_synchronization.ExecutionDataService
	logger      zerolog.Logger
	blobservice *mocknetwork.BlobService
	datastore   datastore.Batching

	cleanup func()
}

func TestExecutionDataRequesterSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ExecutionDataRequesterSuite))
}

// func (suite *ExecutionDataRequesterSuite) SetupTest() {
// 	var cancel context.CancelFunc
// 	suite.ctx, cancel = context.WithCancel(context.Background())

// 	suite.datastore = dssync.MutexWrap(datastore.NewMapDatastore())
// 	suite.blobservice = mockBlobService(blockstore.NewBlockstore(suite.datastore))

// 	suite.cleanup = func() {
// 		cancel()
// 	}

// 	suite.logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

// 	suite.eds = state_synchronization.NewExecutionDataService(&cborcodec.Codec{}, compressor.NewLz4Compressor(), suite.blobservice, metrics.NewNoopCollector(), suite.logger)

// 	suite.state = new(protocolmock.State)
// 	suite.blocks = new(storagemock.Blocks)
// 	suite.results = new(storagemock.ExecutionResults)
// 	suite.allBlocks = make(map[flow.Identifier]*flow.Block)
// 	suite.allResults = make(map[flow.Identifier]*flow.ExecutionResult)
// 	suite.allBlobs = make(map[cid.Cid]bool)
// 	suite.allExecutionData = make(map[flow.Identifier]*state_synchronization.ExecutionData)

// 	var blocks []*flow.Block
// 	var results []*flow.ExecutionResult

// 	var previousBlock *flow.Block
// 	var previousResult *flow.ExecutionResult

// 	// These blocks will be sealed
// 	blockCount := 5000
// 	sealedCount := 4997
// 	firstSeal := blockCount - sealedCount + 2 // offset by 2 so the first block can contain multiple seals
// 	for i := 0; i < blockCount; i++ {
// 		var block *flow.Block
// 		var result *flow.ExecutionResult

// 		if i == 0 {
// 			block = unittest.GenesisFixture()
// 		} else if i < firstSeal {
// 			block = unittest.BlockWithParentFixture(previousBlock.Header)
// 		} else if i == firstSeal {
// 			// first block with seals contains 3
// 			block = unittest.BlockWithParentAndSeals(previousBlock.Header, []*flow.Header{
// 				// intentionally out of order
// 				suite.allBlocks[i-firstSeal+2].Header,
// 				suite.allBlocks[i-firstSeal].Header,
// 				suite.allBlocks[i-firstSeal+1].Header,
// 			})
// 		} else {
// 			block = unittest.BlockWithParentAndSeals(previousBlock.Header, []*flow.Header{
// 				suite.allBlocks[i-firstSeal+2].Header,
// 			})
// 		}

// 		ed := unittest.ExecutionDataFixture(block.ID())
// 		cid, _, err := suite.eds.Add(suite.ctx, ed)
// 		require.NoError(suite.T(), err)

// 		opts := []func(result *flow.ExecutionResult){
// 			unittest.WithBlock(block),
// 			unittest.WithExecutionDataID(cid),
// 		}

// 		if i > 0 {
// 			opts = append(opts, unittest.WithPreviousResult(*previousResult))
// 		}

// 		result = unittest.ExecutionResultFixture(opts...)

// 		suite.allBlocks = append(blocks, block)
// 		results = append(results, result)

// 		suite.allExecutionData[block.ID()] = unittest.ExecutionDataFixture(block.ID())
// 		suite.allBlobs[flow.FlowIDToCid(cid)] = true

// 		previousBlock = block
// 		previousResult = result
// 	}

// 	suite.allBlocks = blocks
// 	suite.allResults = results
// 	suite.sealedBlockCount = sealedCount

// 	suite.blocks.On("ByID", mock.AnythingOfType("flow.Identifier")).Return(
// 		func(blockID flow.Identifier) *flow.Block {
// 			for _, block := range blocks {
// 				if block.ID() == blockID {
// 					return block
// 				}
// 			}
// 			return nil
// 		},
// 		func(blockID flow.Identifier) error {
// 			for _, block := range blocks {
// 				if block.ID() == blockID {
// 					return nil
// 				}
// 			}
// 			return fmt.Errorf("block %#v not found", blockID)
// 		},
// 	)

// 	suite.blocks.On("ByHeight", mock.AnythingOfType("uint64")).Return(
// 		func(height uint64) *flow.Block {
// 			for _, block := range blocks {
// 				if block.Header.Height == height {
// 					return block
// 				}
// 			}
// 			return nil
// 		},
// 		func(height uint64) error {
// 			for _, block := range blocks {
// 				if block.Header.Height == height {
// 					return nil
// 				}
// 			}
// 			return fmt.Errorf("height %d not found", height)
// 		},
// 	)

// 	suite.results.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(
// 		func(blockID flow.Identifier) *flow.ExecutionResult {
// 			for _, result := range results {
// 				if result.BlockID == blockID {
// 					return result
// 				}
// 			}
// 			return nil
// 		},
// 		func(blockID flow.Identifier) error {
// 			for _, result := range results {
// 				if result.BlockID == blockID {
// 					return nil
// 				}
// 			}
// 			return fmt.Errorf("result for block %v not found", blockID)
// 		},
// 	)
// }

func (suite *ExecutionDataRequesterSuite) TearDownTest() {
	if suite.cleanup != nil {
		suite.cleanup()
	}
}

// func (suite *ExecutionDataRequesterSuite) TestExecutionDataRequester() {
// 	ctx, cancel := context.WithTimeout(suite.ctx, 120*time.Second)
// 	defer cancel()

// 	signalerCtx, _ := irrecoverable.WithSignaler(ctx)

// 	finalizationDistributor := pubsub.NewFinalizationDistributor()

// 	edr, err := NewExecutionDataRequester(
// 		suite.logger,
// 		metrics.NewNoopCollector(),
// 		finalizationDistributor,
// 		suite.datastore,
// 		suite.blobservice,
// 		suite.eds,
// 		suite.allBlocks[0],
// 		suite.blocks,
// 		suite.results,
// 	)
// 	assert.NoError(suite.T(), err)

// 	outstandingBlocks := sync.WaitGroup{}
// 	outstandingBlocks.Add(suite.sealedBlockCount)

// 	blockCount := len(suite.allBlocks)

// 	sendLast := make(chan struct{})
// 	fetchedExecutionData := make(map[flow.Identifier]*state_synchronization.ExecutionData, suite.sealedBlockCount)
// 	edr.AddOnExecutionDataFetchedConsumer(func(ed *state_synchronization.ExecutionData) {
// 		fetchedExecutionData[ed.BlockID] = ed
// 		outstandingBlocks.Done()

// 		if blockCount-len(fetchedExecutionData) < 400 {
// 			select {
// 			case sendLast <- struct{}{}:
// 			default:
// 			}
// 		}
// 	})

// 	edr.Start(signalerCtx)

// 	<-edr.Ready()

// 	// Send blocks through finalizationDistributor
// 	var parentView uint64
// 	for i, b := range suite.allBlocks {
// 		suite.T().Log(">>>> Finalizing block", b.ID(), b.Header.Height)

// 		block := model.BlockFromFlow(b.Header, parentView)
// 		finalizationDistributor.OnFinalizedBlock(block)

// 		parentView = b.Header.View

// 		// needs a slight delay otherwise it will fill the queue immediately.
// 		time.Sleep(1 * time.Millisecond)

// 		// the requester can catch up after receiving a finalization block, so on the last
// 		// block, pause until the queue is no longer full and add it
// 		if i == len(suite.allBlocks)-2 {
// 			<-sendLast
// 		}
// 	}

// 	outstandingBlocks.Wait()

// 	assert.Len(suite.T(), fetchedExecutionData, suite.sealedBlockCount)
// 	for blockID, ed := range suite.allExecutionData {
// 		assert.Equal(suite.T(), ed, fetchedExecutionData[blockID], "execution data for block %v is missing", ed.BlockID)
// 	}
// }

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
	bex.On("HasBlob", mock.Anything, mock.AnythingOfType("cid.Cid")).Return(
		func(ctx context.Context, c cid.Cid) bool {
			has, _ := bs.Has(ctx, c)
			return has
		},
		func(ctx context.Context, c cid.Cid) error {
			_, err := bs.Has(ctx, c)
			return err
		},
	)

	noop := module.NoopReadyDoneAware{}
	bex.On("Ready").Return(func() <-chan struct{} { return noop.Ready() })

	return bex
}

type testExecutionDataServiceEntry struct {
	ExecutionData *state_synchronization.ExecutionData
	Err           error
	fn            func() (*state_synchronization.ExecutionData, error)
}

func mockExecutionDataService(edStore map[flow.Identifier]*testExecutionDataServiceEntry) *syncmock.ExecutionDataService {
	eds := new(syncmock.ExecutionDataService)

	get := func(ctx context.Context, id flow.Identifier) (*state_synchronization.ExecutionData, error) {
		ed, has := edStore[id]

		// return not found
		if !has {
			return nil, &state_synchronization.BlobNotFoundError{}
		}

		// return the specific execution data
		if ed.ExecutionData != nil {
			return ed.ExecutionData, nil
		}

		// return a custom error
		if ed.Err != nil {
			return nil, ed.Err
		}

		// use a callback. this is useful for injecting a pause or custom error behavior
		return ed.fn()
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

	eds.On("Has", mock.Anything, mock.AnythingOfType("flow.Identifier")).Return(
		func(ctx context.Context, id flow.Identifier) bool {
			if ed, _ := get(ctx, id); ed != nil {
				return true
			}
			return false
		},
		func(ctx context.Context, id flow.Identifier) error {
			_, err := get(ctx, id)
			return err
		},
	)

	noop := module.NoopReadyDoneAware{}
	eds.On("Ready").Return(func() <-chan struct{} { return noop.Ready() })

	return eds
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
// * happy case (many blocks in order)
// * many blocks, with some missed on first try
// * many blocks, with some missed after many tries (eventually succeed)
// * catch up after finalization queue overflow
// * catch up after fetch [retry] queue overflow
// * out of order blocks are notified in order
// * blocks not in queue are refetched
// * halts are handled gracefully

func (suite *ExecutionDataRequesterSuite) TestBootstrapFromEmptyStateAtSporkStart() {

	// start with empty DB
	// genesis block is > 0
	// latest height == genesis block height
	// check that notification state is correct

}

func (suite *ExecutionDataRequesterSuite) TestBootstrapFromEmptyStateMidSpork() {

	// start with empty DB
	// genesis block is > 0
	// latest height > genesis block height
	// check that it enqueues and downloads the correct blocks
	// check that notification state is correct afterwards

}

func (suite *ExecutionDataRequesterSuite) TestBootstrapFromExistingStateMidSpork() {

	// start with db that contains existing state
	// genesis block is > 0
	// latest height > genesis block height
	// check that it enqueues and downloads the correct blocks
	// check that notification state is correct afterwards

}

// Tests that blocks that are missed are properly retried and backfilled
func TestRequestBlocksWithSomeMissed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	testData := map[flow.Identifier]*testExecutionDataServiceEntry{}

	testBlocksByHeight := map[uint64]*flow.Block{}
	testBlocksByID := map[flow.Identifier]*flow.Block{}
	testResultsByID := map[flow.Identifier]*flow.ExecutionResult{}
	testExecutionDataByID := map[flow.Identifier]*state_synchronization.ExecutionData{}

	blocks := mockBlocksStorage(testBlocksByID, testBlocksByHeight)
	results := mockResultsStorage(testResultsByID)

	eds := mockExecutionDataService(testData)

	blockCount := 100
	sealedCount := blockCount - 3
	firstSeal := blockCount - sealedCount - 1

	startHeight := uint64(0)
	endHeight := startHeight + uint64(blockCount) - 1

	missingHeight := 50

	var previousBlock *flow.Block
	var previousResult *flow.ExecutionResult
	for i := 0; i < blockCount; i++ {
		var seals []*flow.Header

		if i >= firstSeal {
			seals = []*flow.Header{
				testBlocksByHeight[uint64(i-firstSeal)].Header,
			}
		}

		block := buildBlock(uint64(i), previousBlock, seals)

		ed := unittest.ExecutionDataFixture(block.ID())

		cid := unittest.IdentifierFixture()

		result := buildResult(block, cid, previousResult)

		testBlocksByHeight[block.Header.Height] = block
		testBlocksByID[block.ID()] = block
		testResultsByID[block.ID()] = result

		// ignore all the data we don't need to verify the test
		if i < sealedCount {
			testExecutionDataByID[block.ID()] = ed

			if i == missingHeight {
				attempts := 0
				testData[cid] = &testExecutionDataServiceEntry{
					fn: func() (*state_synchronization.ExecutionData, error) {
						if attempts < 10 { // run twice for every attempt
							attempts++
							// This should fail the first 4 fetch attempts
							return nil, errors.New("simulating fetch error")
						}
						return ed, nil
					},
				}
			} else {
				testData[cid] = &testExecutionDataServiceEntry{ExecutionData: ed}
			}
		}

		previousBlock = block
		previousResult = result
	}

	finalizationDistributor := pubsub.NewFinalizationDistributor()

	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	bs := mockBlobService(blockstore.NewBlockstore(ds))

	edr, err := NewExecutionDataRequester(
		logger,
		metrics.NewNoopCollector(),
		finalizationDistributor,
		ds,
		bs,
		eds,
		testBlocksByHeight[uint64(0)],
		blocks,
		results,
	)
	assert.NoError(t, err)

	runFetch(t, ctx, edr, finalizationDistributor, fetchTestRun{
		sealedCount:       sealedCount,
		startHeight:       startHeight,
		endHeight:         endHeight,
		blocksByHeight:    testBlocksByHeight,
		blocksByID:        testBlocksByID,
		resultsByID:       testResultsByID,
		executionDataByID: testExecutionDataByID,
	})
}

func TestHappyCase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Second)
	defer cancel()

	//
	// Setup test data
	//

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	bs := mockBlobService(blockstore.NewBlockstore(ds))

	eds := state_synchronization.NewExecutionDataService(&cbor.Codec{}, compressor.NewLz4Compressor(), bs, metrics.NewNoopCollector(), logger)

	testBlocksByHeight := map[uint64]*flow.Block{}
	testBlocksByID := map[flow.Identifier]*flow.Block{}
	testResultsByID := map[flow.Identifier]*flow.ExecutionResult{}
	testExecutionDataByID := map[flow.Identifier]*state_synchronization.ExecutionData{}

	blocks := mockBlocksStorage(testBlocksByID, testBlocksByHeight)
	results := mockResultsStorage(testResultsByID)

	blockCount := 1000
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
				testBlocksByHeight[uint64(i-firstSeal+2)].Header,
				testBlocksByHeight[uint64(i-firstSeal)].Header,
				testBlocksByHeight[uint64(i-firstSeal+1)].Header,
			}
		} else if i > firstSeal {
			seals = []*flow.Header{
				testBlocksByHeight[uint64(i-firstSeal+2)].Header,
			}
		}

		block := buildBlock(uint64(i), previousBlock, seals)

		ed := unittest.ExecutionDataFixture(block.ID())
		cid, _, err := eds.Add(ctx, ed)
		require.NoError(t, err)

		result := buildResult(block, cid, previousResult)

		testBlocksByHeight[block.Header.Height] = block
		testBlocksByID[block.ID()] = block
		testResultsByID[block.ID()] = result

		// ignore all the data we don't need to verify the test
		if i < sealedCount {
			testExecutionDataByID[block.ID()] = ed
		}

		previousBlock = block
		previousResult = result
	}

	//
	// Run the test
	//

	finalizationDistributor := pubsub.NewFinalizationDistributor()

	edr, err := NewExecutionDataRequester(
		logger,
		metrics.NewNoopCollector(),
		finalizationDistributor,
		ds,
		bs,
		eds,
		testBlocksByHeight[uint64(0)],
		blocks,
		results,
	)
	assert.NoError(t, err)

	runFetch(t, ctx, edr, finalizationDistributor, fetchTestRun{
		sealedCount:       sealedCount,
		startHeight:       startHeight,
		endHeight:         endHeight,
		blocksByHeight:    testBlocksByHeight,
		blocksByID:        testBlocksByID,
		resultsByID:       testResultsByID,
		executionDataByID: testExecutionDataByID,
	})
}

type fetchTestRun struct {
	sealedCount           int
	startHeight           uint64
	endHeight             uint64
	blocksByID            map[flow.Identifier]*flow.Block
	blocksByHeight        map[uint64]*flow.Block
	resultsByID           map[flow.Identifier]*flow.ExecutionResult
	executionDataByID     map[flow.Identifier]*state_synchronization.ExecutionData
	expectedIrrecoverable error
}

func runFetch(t *testing.T, ctx context.Context, edr ExecutionDataRequester, finalizationDistributor *pubsub.FinalizationDistributor, cfg fetchTestRun) {
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)
	go irrecoverableNotExpected(t, ctx, errChan)

	// setup sync to wait for all notifications
	outstandingBlocks := sync.WaitGroup{}
	outstandingBlocks.Add(cfg.sealedCount)

	sendLast := make(chan struct{}, 1)
	fetchedExecutionData := make(map[flow.Identifier]*state_synchronization.ExecutionData, cfg.sealedCount)
	edr.AddOnExecutionDataFetchedConsumer(func(ed *state_synchronization.ExecutionData) {
		t.Logf("fetched execution data: %#v", ed)
		fetchedExecutionData[ed.BlockID] = ed
		outstandingBlocks.Done()

		t.Logf("notified of execution data for block %v height %d (%d/%d)", ed.BlockID, cfg.blocksByID[ed.BlockID].Header.Height, len(fetchedExecutionData), cfg.sealedCount)

		// signal that we're near the end of the expected notifications. this is used to ensure
		// that the last finalized block is successfully enqueued, and the finalization processor
		// can backfill any blocks missed due to queue overflow.
		if cfg.sealedCount-len(fetchedExecutionData) < finalizationQueueLength {
			select {
			case sendLast <- struct{}{}:
			default:
			}
		}
	})

	edr.Start(signalerCtx)

	<-edr.Ready()

	// Send blocks through finalizationDistributor
	var parentView uint64
	for i := cfg.startHeight; i <= cfg.endHeight; i++ {
		b := cfg.blocksByHeight[i]

		t.Log(">>>> Finalizing block", b.ID(), b.Header.Height)

		block := model.BlockFromFlow(b.Header, parentView)
		finalizationDistributor.OnFinalizedBlock(block)

		parentView = b.Header.View

		// needs a slight delay otherwise it will fill the queue immediately.
		time.Sleep(500 * time.Microsecond)

		// the requester can catch up after receiving a finalization block, so on the last
		// block, pause until the queue is no longer full and add it
		if i == cfg.endHeight {
			<-sendLast
		}
	}

	// Pause until we've received all of the expected notifications
	outstandingBlocks.Wait()
	t.Log("All notifications received")

	verifyFetchedExecutionData(t, fetchedExecutionData, cfg.executionDataByID)
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
		t.Errorf("unexpected irrecoverable error: %v", err)
	}
}

func verifyFetchedExecutionData(t *testing.T, actual map[flow.Identifier]*state_synchronization.ExecutionData, expected map[flow.Identifier]*state_synchronization.ExecutionData) {
	assert.Len(t, actual, len(expected))

	for blockID, expectedED := range expected {
		actualED, has := actual[blockID]
		if !has {
			assert.Fail(t, "missing execution data for block %v", blockID)
			continue
		}

		assert.Equal(t, expectedED, actualED, "execution data for block %v doesn't match", blockID)
	}
}
