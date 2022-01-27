package state_synchronization_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2/options"
	"github.com/ipfs/go-datastore"
	badgerds "github.com/ipfs/go-ds-badger2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/ledger"
	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/compressor"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type ExecutionDataTestSuite struct {
	suite.Suite

	state   *protocolmock.State
	blocks  *storagemock.Blocks
	results *storagemock.ExecutionResults

	allBlocks        []*flow.Block
	allResults       []*flow.ExecutionResult
	allExecutionData []*state_synchronization.ExecutionData

	sealedBlockCount int

	ctx         context.Context
	eds         state_synchronization.ExecutionDataService
	logger      zerolog.Logger
	blobservice network.BlobService
	datastore   *badgerds.Datastore

	cleanup func()
}

func TestExecutionDataRequesterSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ExecutionDataTestSuite))
}

func (suite *ExecutionDataTestSuite) SetupTest() {

	var cancel context.CancelFunc
	suite.ctx, cancel = context.WithCancel(context.Background())
	signalerCtx, _ := irrecoverable.WithSignaler(suite.ctx)

	var cleanup func()
	suite.blobservice, suite.datastore, cleanup = makeBlobService(signalerCtx, suite.T(), "execution-data-requester-test")

	suite.cleanup = func() {
		cancel()
		cleanup()
	}

	suite.logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	suite.eds = state_synchronization.NewExecutionDataService(&cborcodec.Codec{}, compressor.NewLz4Compressor(), suite.blobservice, metrics.NewNoopCollector(), suite.logger)

	suite.state = new(protocolmock.State)
	suite.blocks = new(storagemock.Blocks)
	suite.results = new(storagemock.ExecutionResults)

	var blocks []*flow.Block
	var results []*flow.ExecutionResult
	var executionData []*state_synchronization.ExecutionData

	var previousBlock *flow.Block
	var previousResult *flow.ExecutionResult

	// These blocks will be sealed
	blockCount := 20
	sealedCount := 17
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

		ed := executionDataFixture(block)
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
		executionData = append(executionData, executionDataFixture(block))

		previousBlock = block
		previousResult = result
	}

	suite.allBlocks = blocks
	suite.allResults = results
	suite.allExecutionData = executionData
	suite.sealedBlockCount = sealedCount

	suite.blocks.On("ByID", mock.Anything).Return(
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

	suite.blocks.On("ByHeight", mock.Anything).Return(
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

	suite.results.On("ByBlockID", mock.Anything).Return(
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
			return fmt.Errorf("result for block %#v not found", blockID)
		},
	)
}

func (suite *ExecutionDataTestSuite) TearDownTest() {
	suite.cleanup()
}

func (suite *ExecutionDataTestSuite) TestExecutionDataRequester() {

	signalerCtx, _ := irrecoverable.WithSignaler(suite.ctx)

	finalizationDistributor := pubsub.NewFinalizationDistributor()

	edr, err := state_synchronization.NewExecutionDataRequester(
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

	fetchedExecutionData := make([]*state_synchronization.ExecutionData, 0)
	edr.AddOnExecutionDataFetchedConsumer(func(ed *state_synchronization.ExecutionData) {
		fetchedExecutionData = append(fetchedExecutionData, ed)
	})

	edr.Start(signalerCtx)

	<-edr.Ready()

	// Send blocks through finalizationDistributor
	var parentView uint64
	for _, b := range suite.allBlocks {
		suite.T().Log("")
		suite.T().Log(">>>> Finalizing block", b.ID())

		block := model.BlockFromFlow(b.Header, parentView)
		finalizationDistributor.OnFinalizedBlock(block)

		parentView = b.Header.View
		time.Sleep(10 * time.Millisecond)
	}

	assert.Len(suite.T(), fetchedExecutionData, suite.sealedBlockCount)
	for _, ed := range fetchedExecutionData {
		assert.Contains(suite.T(), suite.allExecutionData, ed, "execution data for block %v is missing", ed.BlockID)
	}
}

func makeBlobService(ctx irrecoverable.SignalerContext, t *testing.T, name string) (network.BlobService, *badgerds.Datastore, func()) {
	dsDir := filepath.Join(os.TempDir(), name)
	require.NoError(t, os.RemoveAll(dsDir))

	err := os.Mkdir(dsDir, 0755)
	require.NoError(t, err)

	opts := badgerds.DefaultOptions.WithTTL(5 * time.Minute)
	opts.Options = opts.WithCompression(options.None)

	ds, err := badgerds.NewDatastore(dsDir, &opts)
	require.NoError(t, err)

	bs, _ := createBlobService(ctx, t, ds, name)
	return bs, ds, func() {
		require.NoError(t, os.RemoveAll(dsDir))
	}
}

func createBlobService(ctx irrecoverable.SignalerContext, t *testing.T, ds datastore.Batching, name string, dhtOpts ...dht.Option) (network.BlobService, host.Host) {
	h, err := libp2p.New()
	require.NoError(t, err)

	cr, err := dht.New(ctx, h, dhtOpts...)
	require.NoError(t, err)

	service := network.NewBlobService(h, cr, name, ds)

	service.Start(ctx)

	<-service.Ready()

	return service, h
}

func executionDataFixture(block *flow.Block) *state_synchronization.ExecutionData {
	return &state_synchronization.ExecutionData{
		BlockID:            block.ID(),
		Collections:        []*flow.Collection{},
		Events:             []flow.EventsList{},
		TrieUpdates:        []*ledger.TrieUpdate{},
		TransactionResults: []flow.TransactionResult{},
	}
}
