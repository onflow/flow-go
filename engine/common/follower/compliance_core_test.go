package follower

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/common/follower/cache"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestFollowerCore(t *testing.T) {
	suite.Run(t, new(CoreSuite))
}

// CoreSuite maintains minimal state for testing ComplianceCore.
// Performs startup & shutdown using `module.Startable` and `module.ReadyDoneAware` interfaces.
type CoreSuite struct {
	suite.Suite

	originID         flow.Identifier
	finalizedBlock   *flow.Header
	state            *protocol.FollowerState
	follower         *module.HotStuffFollower
	sync             *module.BlockRequester
	validator        *hotstuff.Validator
	followerConsumer *hotstuff.FollowerConsumer

	ctx    irrecoverable.SignalerContext
	cancel context.CancelFunc
	errs   <-chan error
	core   *ComplianceCore
}

func (s *CoreSuite) SetupTest() {
	s.state = protocol.NewFollowerState(s.T())
	s.follower = module.NewHotStuffFollower(s.T())
	s.validator = hotstuff.NewValidator(s.T())
	s.sync = module.NewBlockRequester(s.T())
	s.followerConsumer = hotstuff.NewFollowerConsumer(s.T())

	s.originID = unittest.IdentifierFixture()
	s.finalizedBlock = unittest.BlockHeaderFixture()
	finalSnapshot := protocol.NewSnapshot(s.T())
	finalSnapshot.On("Head").Return(func() *flow.Header { return s.finalizedBlock }, nil).Once()
	s.state.On("Final").Return(finalSnapshot).Once()

	metrics := metrics.NewNoopCollector()
	var err error
	s.core, err = NewComplianceCore(
		unittest.Logger(),
		metrics,
		metrics,
		s.followerConsumer,
		s.state,
		s.follower,
		s.validator,
		s.sync,
		trace.NewNoopTracer(),
	)
	require.NoError(s.T(), err)

	s.ctx, s.cancel, s.errs = irrecoverable.WithSignallerAndCancel(context.Background())
	s.core.Start(s.ctx)
	unittest.RequireCloseBefore(s.T(), s.core.Ready(), time.Second, "core failed to start")
}

// TearDownTest stops the engine and checks there are no errors thrown to the SignallerContext.
func (s *CoreSuite) TearDownTest() {
	s.cancel()
	unittest.RequireCloseBefore(s.T(), s.core.Done(), time.Second, "core failed to stop")
	select {
	case err := <-s.errs:
		assert.NoError(s.T(), err)
	default:
	}
}

// TestProcessingSingleBlock tests processing a range with length 1, it must result in block being validated and added to cache.
// If block is already in cache it should be no-op.
func (s *CoreSuite) TestProcessingSingleBlock() {
	block := unittest.BlockWithParentFixture(s.finalizedBlock)

	// incoming block has to be validated
	s.validator.On("ValidateProposal", model.ProposalFromFlow(block.Header)).Return(nil).Once()

	err := s.core.OnBlockRange(s.originID, []*flow.Block{block})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), s.core.pendingCache.Peek(block.ID()))

	err = s.core.OnBlockRange(s.originID, []*flow.Block{block})
	require.NoError(s.T(), err)
}

// TestAddFinalizedBlock tests that adding block below finalized height results in processing it, but since cache was pruned
// to finalized view, it must be rejected by it.
func (s *CoreSuite) TestAddFinalizedBlock() {
	block := unittest.BlockFixture()
	block.Header.View = s.finalizedBlock.View - 1 // block is below finalized view

	// incoming block has to be validated
	s.validator.On("ValidateProposal", model.ProposalFromFlow(block.Header)).Return(nil).Once()

	err := s.core.OnBlockRange(s.originID, []*flow.Block{&block})
	require.NoError(s.T(), err)
	require.Nil(s.T(), s.core.pendingCache.Peek(block.ID()))
}

// TestProcessingRangeHappyPath tests processing range of blocks with length > 1, which should result
// in a chain of certified blocks that have been
//  1. validated
//  2. added to the pending cache
//  3. added to the pending tree
//  4. added to the protocol state
//
// Finally, the certified blocks should be forwarded to the HotStuff follower.
func (s *CoreSuite) TestProcessingRangeHappyPath() {
	blocks := unittest.ChainFixtureFrom(10, s.finalizedBlock)

	var wg sync.WaitGroup
	wg.Add(len(blocks) - 1)
	for i := 1; i < len(blocks); i++ {
		s.state.On("ExtendCertified", mock.Anything, blocks[i-1], blocks[i].Header.QuorumCertificate()).Return(nil).Once()
		s.follower.On("AddCertifiedBlock", blockWithID(blocks[i-1].ID())).Run(func(args mock.Arguments) {
			wg.Done()
		}).Return().Once()
	}
	s.validator.On("ValidateProposal", model.ProposalFromFlow(blocks[len(blocks)-1].Header)).Return(nil).Once()

	err := s.core.OnBlockRange(s.originID, blocks)
	require.NoError(s.T(), err)

	unittest.RequireReturnsBefore(s.T(), wg.Wait, 500*time.Millisecond, "expect all blocks to be processed before timeout")
}

// TestProcessingNotOrderedBatch tests that submitting a batch which is not properly ordered(meaning the batch is not connected)
// has to result in error.
func (s *CoreSuite) TestProcessingNotOrderedBatch() {
	blocks := unittest.ChainFixtureFrom(10, s.finalizedBlock)
	blocks[2], blocks[3] = blocks[3], blocks[2]

	s.validator.On("ValidateProposal", model.ProposalFromFlow(blocks[len(blocks)-1].Header)).Return(nil).Once()

	err := s.core.OnBlockRange(s.originID, blocks)
	require.ErrorIs(s.T(), err, cache.ErrDisconnectedBatch)
}

// TestProcessingInvalidBlock tests that processing a batch which ends with invalid block discards the whole batch
func (s *CoreSuite) TestProcessingInvalidBlock() {
	blocks := unittest.ChainFixtureFrom(10, s.finalizedBlock)

	invalidProposal := model.ProposalFromFlow(blocks[len(blocks)-1].Header)
	sentinelError := model.NewInvalidProposalErrorf(invalidProposal, "")
	s.validator.On("ValidateProposal", invalidProposal).Return(sentinelError).Once()
	s.followerConsumer.On("OnInvalidBlockDetected", flow.Slashable[model.InvalidProposalError]{
		OriginID: s.originID,
		Message:  sentinelError.(model.InvalidProposalError),
	}).Return().Once()
	err := s.core.OnBlockRange(s.originID, blocks)
	require.NoError(s.T(), err, "sentinel error has to be handled internally")

	exception := errors.New("validate-proposal-exception")
	s.validator.On("ValidateProposal", invalidProposal).Return(exception).Once()
	err = s.core.OnBlockRange(s.originID, blocks)
	require.ErrorIs(s.T(), err, exception, "exception has to be propagated")
}

// TestProcessingBlocksAfterShutdown tests that submitting blocks after shutdown doesn't block producers.
func (s *CoreSuite) TestProcessingBlocksAfterShutdown() {
	s.cancel()
	unittest.RequireCloseBefore(s.T(), s.core.Done(), time.Second, "core failed to stop")

	// at this point workers are stopped and processing valid range of connected blocks won't be delivered
	// to the protocol state

	blocks := unittest.ChainFixtureFrom(10, s.finalizedBlock)
	s.validator.On("ValidateProposal", model.ProposalFromFlow(blocks[len(blocks)-1].Header)).Return(nil).Once()

	err := s.core.OnBlockRange(s.originID, blocks)
	require.NoError(s.T(), err)
}

// TestProcessingConnectedRangesOutOfOrder tests that processing range of connected blocks [B1 <- ... <- BN+1] our of order
// results in extending [B1 <- ... <- BN] in correct order.
func (s *CoreSuite) TestProcessingConnectedRangesOutOfOrder() {
	blocks := unittest.ChainFixtureFrom(10, s.finalizedBlock)
	midpoint := len(blocks) / 2
	firstHalf, secondHalf := blocks[:midpoint], blocks[midpoint:]

	s.validator.On("ValidateProposal", mock.Anything).Return(nil).Once()
	err := s.core.OnBlockRange(s.originID, secondHalf)
	require.NoError(s.T(), err)

	var wg sync.WaitGroup
	wg.Add(len(blocks) - 1)
	for _, block := range blocks[:len(blocks)-1] {
		s.follower.On("AddCertifiedBlock", blockWithID(block.ID())).Return().Run(func(args mock.Arguments) {
			wg.Done()
		}).Once()
	}

	lastSubmittedBlockID := flow.ZeroID
	s.state.On("ExtendCertified", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		block := args.Get(1).(*flow.Block)
		if lastSubmittedBlockID != flow.ZeroID {
			if block.Header.ParentID != lastSubmittedBlockID {
				s.Failf("blocks not sequential",
					"blocks submitted to protocol state are not sequential at height %d", block.Header.Height)
			}
		}
		lastSubmittedBlockID = block.ID()
	}).Return(nil).Times(len(blocks) - 1)

	s.validator.On("ValidateProposal", mock.Anything).Return(nil).Once()
	err = s.core.OnBlockRange(s.originID, firstHalf)
	require.NoError(s.T(), err)
	unittest.RequireReturnsBefore(s.T(), wg.Wait, time.Millisecond*500, "expect to process all blocks before timeout")
}

// TestDetectingProposalEquivocation tests that block equivocation is properly detected and reported to specific consumer.
func (s *CoreSuite) TestDetectingProposalEquivocation() {
	block := unittest.BlockWithParentFixture(s.finalizedBlock)
	otherBlock := unittest.BlockWithParentFixture(s.finalizedBlock)
	otherBlock.Header.View = block.Header.View

	s.validator.On("ValidateProposal", mock.Anything).Return(nil).Times(2)
	s.followerConsumer.On("OnDoubleProposeDetected", mock.Anything, mock.Anything).Return().Once()

	err := s.core.OnBlockRange(s.originID, []*flow.Block{block})
	require.NoError(s.T(), err)

	err = s.core.OnBlockRange(s.originID, []*flow.Block{otherBlock})
	require.NoError(s.T(), err)
}

// TestConcurrentAdd simulates multiple workers adding batches of connected blocks out of order.
// We use the following setup:
// Number of workers - workers
//   - Number of workers - workers
//   - Number of batches submitted by worker - batchesPerWorker
//   - Number of blocks in each batch submitted by worker - blocksPerBatch
//   - Each worker submits batchesPerWorker*blocksPerBatch blocks
//
// In total we will submit workers*batchesPerWorker*blocksPerBatch
// After submitting all blocks we expect that chain of blocks except last one will be added to the protocol state and
// submitted for further processing to Hotstuff layer.
func (s *CoreSuite) TestConcurrentAdd() {
	workers := 5
	batchesPerWorker := 10
	blocksPerBatch := 10
	blocksPerWorker := blocksPerBatch * batchesPerWorker
	blocks := unittest.ChainFixtureFrom(workers*blocksPerWorker, s.finalizedBlock)
	targetSubmittedBlockID := blocks[len(blocks)-2].ID()
	require.Lessf(s.T(), len(blocks), defaultPendingBlocksCacheCapacity, "this test works under assumption that we operate under cache upper limit")

	s.validator.On("ValidateProposal", mock.Anything).Return(nil) // any proposal is valid
	done := make(chan struct{})

	s.follower.On("AddCertifiedBlock", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		// ensure that proposals are submitted in-order
		block := args.Get(0).(*model.CertifiedBlock)
		if block.ID() == targetSubmittedBlockID {
			close(done)
		}
	}).Return().Times(len(blocks) - 1) // all proposals have to be submitted
	lastSubmittedBlockID := flow.ZeroID
	s.state.On("ExtendCertified", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		block := args.Get(1).(*flow.Block)
		if lastSubmittedBlockID != flow.ZeroID {
			if block.Header.ParentID != lastSubmittedBlockID {
				s.Failf("blocks not sequential",
					"blocks submitted to protocol state are not sequential at height %d", block.Header.Height)
			}
		}
		lastSubmittedBlockID = block.ID()
	}).Return(nil).Times(len(blocks) - 1)

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(blocks []*flow.Block) {
			defer wg.Done()
			for batch := 0; batch < batchesPerWorker; batch++ {
				err := s.core.OnBlockRange(s.originID, blocks[batch*blocksPerBatch:(batch+1)*blocksPerBatch])
				require.NoError(s.T(), err)
			}
		}(blocks[i*blocksPerWorker : (i+1)*blocksPerWorker])
	}

	unittest.RequireReturnsBefore(s.T(), wg.Wait, time.Millisecond*500, "should submit blocks before timeout")
	unittest.AssertClosesBefore(s.T(), done, time.Millisecond*500, "should process all blocks before timeout")
}

// blockWithID returns a testify `argumentMatcher` that only accepts blocks with the given ID
func blockWithID(expectedBlockID flow.Identifier) interface{} {
	return mock.MatchedBy(func(block *model.CertifiedBlock) bool { return expectedBlockID == block.ID() })
}
