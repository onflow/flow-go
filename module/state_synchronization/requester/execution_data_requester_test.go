package requester

import (
	"context"
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
	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization"
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

	allBlocks        []*flow.Block
	allResults       []*flow.ExecutionResult
	allExecutionData []*state_synchronization.ExecutionData

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

func (suite *ExecutionDataRequesterSuite) SetupTest() {
	var cancel context.CancelFunc
	suite.ctx, cancel = context.WithCancel(context.Background())

	suite.datastore = dssync.MutexWrap(datastore.NewMapDatastore())
	suite.blobservice = mockBlobService(blockstore.NewBlockstore(suite.datastore))

	suite.cleanup = func() {
		cancel()
	}

	suite.logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	suite.eds = state_synchronization.NewExecutionDataService(&cborcodec.Codec{}, compressor.NewLz4Compressor(), suite.blobservice, metrics.NewNoopCollector(), suite.logger)

	suite.state = new(protocolmock.State)
	suite.blocks = new(storagemock.Blocks)
	suite.results = new(storagemock.ExecutionResults)
	suite.allBlobs = make(map[cid.Cid]bool)

	var blocks []*flow.Block
	var results []*flow.ExecutionResult
	var executionData []*state_synchronization.ExecutionData

	var previousBlock *flow.Block
	var previousResult *flow.ExecutionResult

	// These blocks will be sealed
	blockCount := 5000
	sealedCount := 4997
	firstSeal := blockCount - sealedCount + 2 // offset by 2 so the first block can contain multiple seals
	for i := 0; i < blockCount; i++ {
		var block *flow.Block
		var result *flow.ExecutionResult

		if i == 0 {
			block = unittest.GenesisFixture()
		} else if i < firstSeal {
			block = unittest.BlockWithParentFixture(previousBlock.Header)
		} else if i == firstSeal {
			// first block with seals contains 3
			block = unittest.BlockWithParentAndSeals(previousBlock.Header, []*flow.Header{
				// intentionally out of order
				blocks[i-firstSeal+2].Header,
				blocks[i-firstSeal].Header,
				blocks[i-firstSeal+1].Header,
			})
		} else {
			block = unittest.BlockWithParentAndSeals(previousBlock.Header, []*flow.Header{
				blocks[i-firstSeal+2].Header,
			})
		}

		ed := unittest.ExecutionDataFixture(block.ID())
		cid, _, err := suite.eds.Add(suite.ctx, ed)
		require.NoError(suite.T(), err)

		opts := []func(result *flow.ExecutionResult){
			unittest.WithBlock(block),
			unittest.WithExecutionDataID(cid),
		}

		if i > 0 {
			opts = append(opts, unittest.WithPreviousResult(*previousResult))
		}

		result = unittest.ExecutionResultFixture(opts...)

		blocks = append(blocks, block)
		results = append(results, result)
		executionData = append(executionData, unittest.ExecutionDataFixture(block.ID()))

		suite.allBlobs[flow.FlowIDToCid(cid)] = true

		previousBlock = block
		previousResult = result
	}

	suite.allBlocks = blocks
	suite.allResults = results
	suite.allExecutionData = executionData
	suite.sealedBlockCount = sealedCount

	suite.blocks.On("ByID", mock.AnythingOfType("flow.Identifier")).Return(
		func(blockID flow.Identifier) *flow.Block {
			for _, block := range blocks {
				if block.ID() == blockID {
					return block
				}
			}
			return nil
		},
		func(blockID flow.Identifier) error {
			for _, block := range blocks {
				if block.ID() == blockID {
					return nil
				}
			}
			return fmt.Errorf("block %#v not found", blockID)
		},
	)

	suite.blocks.On("ByHeight", mock.AnythingOfType("uint64")).Return(
		func(height uint64) *flow.Block {
			for _, block := range blocks {
				if block.Header.Height == height {
					return block
				}
			}
			return nil
		},
		func(height uint64) error {
			for _, block := range blocks {
				if block.Header.Height == height {
					return nil
				}
			}
			return fmt.Errorf("height %d not found", height)
		},
	)

	suite.results.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(
		func(blockID flow.Identifier) *flow.ExecutionResult {
			for _, result := range results {
				if result.BlockID == blockID {
					return result
				}
			}
			return nil
		},
		func(blockID flow.Identifier) error {
			for _, result := range results {
				if result.BlockID == blockID {
					return nil
				}
			}
			return fmt.Errorf("result for block %v not found", blockID)
		},
	)

	suite.blobservice.On("HasBlob", mock.Anything, mock.AnythingOfType("cid.Cid")).Return(
		func(_ context.Context, cid cid.Cid) bool {
			return suite.allBlobs[cid]
		},
		func(_ context.Context, cid cid.Cid) error {
			if suite.allBlobs[cid] {
				return nil
			}
			return fmt.Errorf("execution data %v not found", cid)
		},
	)
}

func (suite *ExecutionDataRequesterSuite) TearDownTest() {
	if suite.cleanup != nil {
		suite.cleanup()
	}
}

func (suite *ExecutionDataRequesterSuite) TestExecutionDataRequester() {
	ctx, cancel := context.WithTimeout(suite.ctx, 120*time.Second)
	defer cancel()

	signalerCtx, _ := irrecoverable.WithSignaler(ctx)

	finalizationDistributor := pubsub.NewFinalizationDistributor()

	edr, err := NewExecutionDataRequester(
		suite.logger,
		metrics.NewNoopCollector(),
		finalizationDistributor,
		suite.datastore,
		suite.blobservice,
		suite.eds,
		suite.allBlocks[0],
		suite.blocks,
		suite.results,
	)
	assert.NoError(suite.T(), err)

	outstandingBlocks := sync.WaitGroup{}
	outstandingBlocks.Add(suite.sealedBlockCount)

	blockCount := len(suite.allBlocks)

	sendLast := make(chan struct{})
	fetchedExecutionData := make([]*state_synchronization.ExecutionData, 0, suite.sealedBlockCount)
	edr.AddOnExecutionDataFetchedConsumer(func(ed *state_synchronization.ExecutionData) {
		fetchedExecutionData = append(fetchedExecutionData, ed)
		outstandingBlocks.Done()

		if blockCount-len(fetchedExecutionData) < 400 {
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
	for i, b := range suite.allBlocks {
		suite.T().Log(">>>> Finalizing block", b.ID(), b.Header.Height)

		block := model.BlockFromFlow(b.Header, parentView)
		finalizationDistributor.OnFinalizedBlock(block)

		parentView = b.Header.View

		// needs a slight delay otherwise it will fill the queue immediately.
		time.Sleep(1 * time.Millisecond)

		// the requester can catch up after receiving a finalization block, so on the last
		// block, pause until the queue is no longer full and add it
		if i == len(suite.allBlocks)-2 {
			<-sendLast
		}
	}

	outstandingBlocks.Wait()

	assert.Len(suite.T(), fetchedExecutionData, suite.sealedBlockCount)
	for _, ed := range fetchedExecutionData {
		assert.Contains(suite.T(), suite.allExecutionData, ed, "execution data for block %v is missing", ed.BlockID)
	}
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

	ready := make(chan struct{})
	close(ready)
	bex.On("AddBlobs", mock.Anything, mock.AnythingOfType("[]blocks.Block")).Return(bs.PutMany)
	bex.On("Ready").Return(func() <-chan struct{} { return ready })

	return bex
}

// Test cases:
// * bootstrap with an empty db sets configuration correctly
// * bootstrap with a non-empty db sets configuration correctly
// * bootstrap with halted db does not start
// * happy case (many blocks in order)
// * many blocks, with some missed on first try
// * many blocks, with some missed after many tries (eventually succeed)
// * catch up after finization queue overflow
// * catch up after fetch [retry] queue overflow
// * out of order blocks are notified in order
// * blocks not in queue are refetched
// * halts are handled gracefully
