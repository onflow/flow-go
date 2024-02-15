package compliance

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/irrecoverable"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestComplianceEngine(t *testing.T) {
	suite.Run(t, new(EngineSuite))
}

// EngineSuite tests the compliance engine.
type EngineSuite struct {
	CommonSuite

	ctx    irrecoverable.SignalerContext
	cancel context.CancelFunc
	errs   <-chan error
	engine *Engine
}

func (cs *EngineSuite) SetupTest() {
	cs.CommonSuite.SetupTest()

	e, err := NewEngine(unittest.Logger(), cs.me, cs.core)
	require.NoError(cs.T(), err)
	cs.engine = e

	cs.ctx, cs.cancel, cs.errs = irrecoverable.WithSignallerAndCancel(context.Background())
	cs.engine.Start(cs.ctx)
	go unittest.FailOnIrrecoverableError(cs.T(), cs.ctx.Done(), cs.errs)

	unittest.AssertClosesBefore(cs.T(), cs.engine.Ready(), time.Second)
}

// TearDownTest stops the engine and checks there are no errors thrown to the SignallerContext.
func (cs *EngineSuite) TearDownTest() {
	cs.cancel()
	unittest.RequireCloseBefore(cs.T(), cs.engine.Done(), time.Second, "engine failed to stop")
	select {
	case err := <-cs.errs:
		assert.NoError(cs.T(), err)
	default:
	}
}

// TestSubmittingMultipleVotes tests that we can send multiple blocks, and they
// are queued and processed in expected way
func (cs *EngineSuite) TestSubmittingMultipleEntries() {
	// create a vote
	blockCount := 15

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for i := 0; i < blockCount; i++ {
			block := unittest.BlockWithParentFixture(cs.head)
			proposal := messages.NewBlockProposal(block)
			hotstuffProposal := model.ProposalFromFlow(block.Header)
			cs.hotstuff.On("SubmitProposal", hotstuffProposal).Return().Once()
			cs.voteAggregator.On("AddBlock", hotstuffProposal).Once()
			cs.validator.On("ValidateProposal", hotstuffProposal).Return(nil).Once()
			// execute the block submission
			cs.engine.OnBlockProposal(flow.Slashable[*messages.BlockProposal]{
				OriginID: unittest.IdentifierFixture(),
				Message:  proposal,
			})
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		// create a proposal that directly descends from the latest finalized header
		block := unittest.BlockWithParentFixture(cs.head)
		proposal := unittest.ProposalFromBlock(block)

		hotstuffProposal := model.ProposalFromFlow(block.Header)
		cs.hotstuff.On("SubmitProposal", hotstuffProposal).Return().Once()
		cs.voteAggregator.On("AddBlock", hotstuffProposal).Once()
		cs.validator.On("ValidateProposal", hotstuffProposal).Return(nil).Once()
		cs.engine.OnBlockProposal(flow.Slashable[*messages.BlockProposal]{
			OriginID: unittest.IdentifierFixture(),
			Message:  proposal,
		})
		wg.Done()
	}()

	// wait for all messages to be delivered to the engine message queue
	wg.Wait()
	// wait for the votes queue to drain
	assert.Eventually(cs.T(), func() bool {
		return cs.engine.pendingBlocks.Len() == 0
	}, time.Second, time.Millisecond*10)
}

// TestOnFinalizedBlock tests if finalized block gets processed when send through `Engine`.
// Tests the whole processing pipeline.
func (cs *EngineSuite) TestOnFinalizedBlock() {
	finalizedBlock := unittest.BlockHeaderFixture()
	cs.head = finalizedBlock
	cs.headerDB[finalizedBlock.ID()] = finalizedBlock

	*cs.pending = *modulemock.NewPendingBlockBuffer(cs.T())
	// wait for both expected calls before ending the test
	wg := new(sync.WaitGroup)
	wg.Add(2)
	cs.pending.On("PruneByView", finalizedBlock.View).
		Run(func(_ mock.Arguments) { wg.Done() }).
		Return(nil).Once()
	cs.pending.On("Size").
		Run(func(_ mock.Arguments) { wg.Done() }).
		Return(uint(0)).Once()

	err := cs.engine.processOnFinalizedBlock(model.BlockFromFlow(finalizedBlock))
	require.NoError(cs.T(), err)
	unittest.AssertReturnsBefore(cs.T(), wg.Wait, time.Second, "an expected call to block buffer wasn't made")
}
