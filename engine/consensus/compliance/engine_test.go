package compliance

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/irrecoverable"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
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
	cs.hotstuff.On("Start", mock.Anything)
	cs.hotstuff.On("Ready", mock.Anything).Return(unittest.ClosedChannel()).Maybe()
	cs.hotstuff.On("Done", mock.Anything).Return(unittest.ClosedChannel()).Maybe()

	e, err := NewEngine(unittest.Logger(), cs.net, cs.me, cs.prov, cs.core)
	require.NoError(cs.T(), err)
	e.WithConsensus(cs.hotstuff)
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

// TestSendVote tests that single vote can be sent and properly processed
func (cs *EngineSuite) TestSendVote() {
	// create parameters to send a vote
	blockID := unittest.IdentifierFixture()
	view := rand.Uint64()
	sig := unittest.SignatureFixture()
	recipientID := unittest.IdentifierFixture()
	vote := &messages.BlockVote{
		BlockID: blockID,
		View:    view,
		SigData: sig,
	}

	done := make(chan struct{})
	*cs.con = *mocknetwork.NewConduit(cs.T())
	cs.con.On("Unicast", vote, recipientID).
		Run(func(_ mock.Arguments) { close(done) }).
		Return(nil).
		Once()

	// submit the vote
	err := cs.engine.SendVote(blockID, view, sig, recipientID)
	require.NoError(cs.T(), err, "should pass send vote")

	// wait for vote to be sent
	unittest.AssertClosesBefore(cs.T(), done, time.Second)
}

// TestBroadcastProposalWithDelay tests broadcasting proposals with different inputs
func (cs *EngineSuite) TestBroadcastProposalWithDelay() {
	// add execution node to participants to make sure we exclude them from broadcast
	cs.participants = append(cs.participants, unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution)))

	// generate a parent with height and chain ID set
	parent := unittest.BlockHeaderFixture()
	parent.ChainID = "test"
	parent.Height = 10
	cs.headerDB[parent.ID()] = parent

	// create a block with the parent and store the payload with correct ID
	block := unittest.BlockWithParentFixture(parent)
	block.Header.ProposerID = cs.myID
	cs.payloadDB[block.ID()] = block.Payload

	cs.Run("should fail with wrong proposer", func() {
		header := *block.Header
		header.ProposerID = unittest.IdentifierFixture()
		err := cs.engine.BroadcastProposalWithDelay(&header, 0)
		require.Error(cs.T(), err, "should fail with wrong proposer")
		header.ProposerID = cs.myID
	})

	// should fail with changed (missing) parent
	cs.Run("should fail with changed/missing parent", func() {
		header := *block.Header
		header.ParentID[0]++
		err := cs.engine.BroadcastProposalWithDelay(&header, 0)
		require.Error(cs.T(), err, "should fail with missing parent")
		header.ParentID[0]--
	})

	// should fail with wrong block ID (payload unavailable)
	cs.Run("should fail with wrong block ID", func() {
		header := *block.Header
		header.View++
		err := cs.engine.BroadcastProposalWithDelay(&header, 0)
		require.Error(cs.T(), err, "should fail with missing payload")
		header.View--
	})

	cs.Run("should broadcast proposal and pass to HotStuff for valid proposals", func() {
		expectedBroadcastMsg := &messages.BlockProposal{
			Header:  block.Header,
			Payload: block.Payload,
		}

		submitted := make(chan struct{}) // closed when proposal is submitted to hotstuff
		cs.hotstuff.On("SubmitProposal", block.Header).
			Run(func(args mock.Arguments) { close(submitted) }).
			Once()

		broadcasted := make(chan struct{}) // closed when proposal is broadcast
		*cs.con = *mocknetwork.NewConduit(cs.T())
		cs.con.On("Publish", expectedBroadcastMsg, cs.participants[1].NodeID, cs.participants[2].NodeID).
			Run(func(_ mock.Arguments) { close(broadcasted) }).
			Return(nil).
			Once()

		// submit to broadcast proposal
		err := cs.engine.BroadcastProposalWithDelay(block.Header, 0)
		require.NoError(cs.T(), err, "header broadcast should pass")

		unittest.AssertClosesBefore(cs.T(), util.AllClosed(broadcasted, submitted), time.Second)
	})
}

// TestSubmittingMultipleVotes tests that we can send multiple votes and they
// are queued and processed in expected way
func (cs *EngineSuite) TestSubmittingMultipleEntries() {
	// create a vote
	originID := unittest.IdentifierFixture()
	voteCount := 15

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for i := 0; i < voteCount; i++ {
			vote := messages.BlockVote{
				BlockID: unittest.IdentifierFixture(),
				View:    rand.Uint64(),
				SigData: unittest.SignatureFixture(),
			}
			cs.voteAggregator.On("AddVote", &model.Vote{
				View:     vote.View,
				BlockID:  vote.BlockID,
				SignerID: originID,
				SigData:  vote.SigData,
			}).Return().Once()
			// execute the vote submission
			err := cs.engine.Process(channels.ConsensusCommittee, originID, &vote)
			cs.Assert().NoError(err)
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		// create a proposal that directly descends from the latest finalized header
		originID := cs.participants[1].NodeID
		block := unittest.BlockWithParentFixture(cs.head)
		proposal := unittest.ProposalFromBlock(block)

		// store the data for retrieval
		cs.headerDB[block.Header.ParentID] = cs.head
		cs.hotstuff.On("SubmitProposal", block.Header).Return()
		err := cs.engine.Process(channels.ConsensusCommittee, originID, proposal)
		cs.Assert().NoError(err)
		wg.Done()
	}()

	// wait for all messages to be delivered to the engine message queue
	wg.Wait()
	// wait for the votes queue to drain
	assert.Eventually(cs.T(), func() bool {
		return cs.engine.pendingVotes.(*engine.FifoMessageStore).Len() == 0
	}, time.Second, time.Millisecond*10)
}

// TestOnFinalizedBlock tests if finalized block gets processed when send through `Engine`.
// Tests the whole processing pipeline.
func (cs *EngineSuite) TestOnFinalizedBlock() {
	finalizedBlock := unittest.BlockHeaderFixture()
	cs.head = finalizedBlock

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

	cs.engine.OnFinalizedBlock(model.BlockFromFlow(finalizedBlock))
	unittest.AssertReturnsBefore(cs.T(), wg.Wait, time.Second, "an expected call to block buffer wasn't made")
}
