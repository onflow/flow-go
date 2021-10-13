package eventhandler_test

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/eventhandler"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/model/flow"
)

func TestEventHandlerV2(t *testing.T) {
	suite.Run(t, new(EventHandlerV2Suite))
}

// EventHandlerV2Suite contains mocked state for testing event handler under different scenarios.
type EventHandlerV2Suite struct {
	suite.Suite

	eventhandler *eventhandler.EventHandlerV2

	paceMaker      hotstuff.PaceMaker
	forks          *Forks
	persist        *mocks.Persister
	blockProducer  *BlockProducer
	communicator   *mocks.Communicator
	committee      *Committee
	voteAggregator *mocks.VoteAggregatorV2
	voter          *Voter
	validator      *BlacklistValidator
	notifier       hotstuff.Consumer

	initView    uint64
	endView     uint64
	votingBlock *model.Block
	qc          *flow.QuorumCertificate
	newview     *model.NewViewEvent
}

func (es *EventHandlerV2Suite) SetupTest() {
	finalized, curView := uint64(3), uint64(6)

	es.paceMaker = initPaceMaker(es.T(), curView)
	es.forks = NewForks(es.T(), finalized)
	es.persist = &mocks.Persister{}
	es.persist.On("PutStarted", mock.Anything).Return(nil)
	es.blockProducer = &BlockProducer{}
	es.communicator = &mocks.Communicator{}
	es.communicator.On("BroadcastProposalWithDelay", mock.Anything, mock.Anything).Return(nil)
	es.communicator.On("SendVote", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	es.committee = NewCommittee()
	es.voteAggregator = &mocks.VoteAggregatorV2{}
	es.voter = NewVoter(es.T(), finalized)
	es.validator = NewBlacklistValidator(es.T())
	es.notifier = &notifications.NoopConsumer{}

	eventhandler, err := eventhandler.NewEventHandlerV2(
		zerolog.New(os.Stderr),
		es.paceMaker,
		es.blockProducer,
		es.forks,
		es.persist,
		es.communicator,
		es.committee,
		es.voteAggregator,
		es.voter,
		es.validator,
		es.notifier)
	require.NoError(es.T(), err)

	es.eventhandler = eventhandler

	es.initView = curView
	es.endView = curView
	// voting block is a block for the current view, which will trigger view change
	es.votingBlock = createBlockWithQC(es.paceMaker.CurView(), es.paceMaker.CurView()-1)
	es.qc = &flow.QuorumCertificate{
		BlockID:   es.votingBlock.BlockID,
		View:      es.votingBlock.View,
		SignerIDs: nil,
		SigData:   nil,
	}
	es.newview = &model.NewViewEvent{
		View: es.votingBlock.View + 1, // the vote for the voting blocks will trigger a view change to the next view
	}
}

func (es *EventHandlerV2Suite) TestQCBuiltViewChanged() {
	// voting block exists
	es.forks.blocks[es.votingBlock.BlockID] = es.votingBlock

	// a qc is built
	qc := createQC(es.votingBlock)

	// new qc is added to forks
	// view changed
	// I'm not the next leader
	// haven't received block for next view
	// goes to the new view
	es.endView++
	// not the leader of the newview
	// don't have block for the newview
	// over

	err := es.eventhandler.OnQCConstructed(qc)
	require.NoError(es.T(), err, "if a vote can trigger a QC to be built,"+
		"and the QC triggered a view change, then start new view")
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
}

// in the newview, I'm not the leader, and I have the cur block,
// and the block is not a safe node, and I'm the next leader, and no qc built for this block.
func (es *EventHandlerV2Suite) TestInNewView_NotLeader_HasBlock_NoVote_IsNextLeader_NoQC() {
	// voting block exists
	es.forks.blocks[es.votingBlock.BlockID] = es.votingBlock
	// a qc is built
	qc := createQC(es.votingBlock)
	// viewchanged
	es.endView++
	// not leader for newview

	// has block for newview
	newviewblock := createBlockWithQC(es.newview.View, es.newview.View-1)
	es.forks.blocks[newviewblock.BlockID] = newviewblock

	// not to vote for the new view block

	// I'm the next leader
	es.committee.leaders[es.newview.View+1] = struct{}{}

	// no QC for the new view
	err := es.eventhandler.OnQCConstructed(qc)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
}

// TestInNewView_NotLeader_HasBlock_NoVote_IsNextLeader_QCBuilt_NoViewChange doesn't exist
// in the newview, I'm not the leader, and I have the cur block,
// and the block is not a safe node, and I'm the next leader, and a qc is built for this block,
// and the qc triggered view change.
func (es *EventHandlerV2Suite) TestInNewView_NotLeader_HasBlock_NoVote_IsNextLeader_QCBuilt_ViewChanged() {
	// voting block exists
	es.forks.blocks[es.votingBlock.BlockID] = es.votingBlock
	// a qc is built
	qc := createQC(es.votingBlock)
	// viewchanged
	es.endView++
	// not leader for newview

	// has block for newview
	newviewblock := createBlockWithQC(es.newview.View, es.newview.View-1)
	es.forks.blocks[newviewblock.BlockID] = newviewblock

	// not to vote for the new view block

	// I'm the next leader
	es.committee.leaders[es.newview.View+1] = struct{}{}

	// qc built for the new view block
	nextQC := createQC(newviewblock)
	// view change by this qc
	es.endView++

	err := es.eventhandler.OnQCConstructed(qc)
	require.NoError(es.T(), err)

	// no broadcast shouldn't be made with first qc because we are not a leader
	es.communicator.AssertNotCalled(es.T(), "BroadcastProposalWithDelay")

	err = es.eventhandler.OnQCConstructed(nextQC)
	require.NoError(es.T(), err)

	lastCall := es.communicator.Calls[len(es.communicator.Calls)-1]
	// the last call is BroadcastProposal
	require.Equal(es.T(), "BroadcastProposalWithDelay", lastCall.Method)
	header, ok := lastCall.Arguments[0].(*flow.Header)
	require.True(es.T(), ok)
	// it should broadcast a header as the same as endView
	require.Equal(es.T(), es.endView, header.View)
}

// in the newview, I'm not the leader, and I have the cur block,
// and the block is a safe node to vote, and I'm the next leader, and no qc is built for this block.
func (es *EventHandlerV2Suite) TestInNewView_NotLeader_HasBlock_NotSafeNode_IsNextLeader_Voted_NoQC() {
	// voting block exists
	es.forks.blocks[es.votingBlock.BlockID] = es.votingBlock
	// a qc is built
	qc := createQC(es.votingBlock)
	// viewchanged by new qc
	es.endView++
	// not leader for newview

	// has block for newview
	newviewblock := createBlockWithQC(es.newview.View, es.newview.View-1)
	es.forks.blocks[newviewblock.BlockID] = newviewblock

	// not to vote for the new view block

	// I'm the next leader
	es.committee.leaders[es.newview.View+1] = struct{}{}

	// no qc for the newview block

	// should not trigger view change
	err := es.eventhandler.OnQCConstructed(qc)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
}

// in the newview, I'm not the leader, and I have the cur block,
// and the block is not a safe node to vote, and I'm not the next leader
func (es *EventHandlerV2Suite) TestInNewView_NotLeader_HasBlock_NotSafeNode_NotNextLeader() {
	// voting block exists
	es.forks.blocks[es.votingBlock.BlockID] = es.votingBlock
	// a qc is built
	qc := createQC(es.votingBlock)
	// viewchanged by new qc
	es.endView++

	// view changed to newview
	// I'm not the leader for newview

	// have received block for cur view
	newviewblock := createBlockWithQC(es.newview.View, es.newview.View-1)
	es.forks.blocks[newviewblock.BlockID] = newviewblock

	// I'm not the next leader
	// no vote for this block
	// goes to the next view
	es.endView++
	// not leader for next view

	err := es.eventhandler.OnQCConstructed(qc)
	require.NoError(es.T(), err, "if a vote can trigger a QC to be built,"+
		"and the QC triggered a view change, then start new view")
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
}

// receiving an invalid proposal should not trigger view change
func (es *EventHandlerV2Suite) TestOnReceiveProposal_InvalidProposal_NoViewChange() {
	proposal := createProposal(es.initView, es.initView-1)
	// invalid proposal
	es.validator.invalidProposals[proposal.Block.BlockID] = struct{}{}

	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
}

// received a valid proposal that has older view, and cannot build qc from votes for this block,
// the proposal's QC didn't trigger view change
func (es *EventHandlerV2Suite) TestOnReceiveProposal_OlderThanCurView_CannotBuildQCFromVotes_NoViewChange() {
	proposal := createProposal(es.initView-1, es.initView-2)
	es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()

	// can not build qc from votes for block
	// should not trigger view change
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.voteAggregator.AssertCalled(es.T(), "AddBlock", proposal)
}

// received a valid proposal that has older view, and can built a qc from votes for this block,
// the proposal's QC didn't trigger view change
func (es *EventHandlerV2Suite) TestOnReceiveProposal_OlderThanCurView_CanBuildQCFromVotes_NoViewChange() {
	proposal := createProposal(es.initView-1, es.initView-2)
	es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()

	//should not trigger view change
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.voteAggregator.AssertCalled(es.T(), "AddBlock", proposal)
}

// received a valid proposal that has newer view, and cannot build qc from votes for this block,
// the proposal's QC triggered view change
func (es *EventHandlerV2Suite) TestOnReceiveProposal_NewerThanCurView_CannotBuildQCFromVotes_ViewChange() {
	proposal := createProposal(es.initView+1, es.initView)
	es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()

	// can not build qc from votes for block
	// block 7 triggered view change
	es.endView++

	// not leader of view 7, go to view 8
	es.endView++
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.voteAggregator.AssertCalled(es.T(), "AddBlock", proposal)
}

// received a valid proposal that has newer view, and can build qc from votes for this block,
// the proposal's QC triggered view change
func (es *EventHandlerV2Suite) TestOnReceiveProposal_NewerThanCurView_CanBuildQCFromVotes_ViewChange() {
	proposal := createProposal(es.initView+1, es.initView)
	es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()

	es.forks.blocks[proposal.Block.BlockID] = proposal.Block

	// trigged view change
	es.endView++
	// the proposal is for next view, has block for next view, no vote, trigger view change
	es.endView++

	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.voteAggregator.AssertCalled(es.T(), "AddBlock", proposal)
}

// received a valid proposal whose QC that has newer view, and cannot build qc from votes for this block,
// the proposal's QC triggered view change
func (es *EventHandlerV2Suite) TestOnReceiveProposal_QCNewerThanCurView_CannotBuildQCFromVotes_ViewChanged() {
	proposal := createProposal(es.initView+2, es.initView+1)
	es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()

	// can not build qc from votes for block
	// block 8 triggered view change
	es.endView = es.endView + 2

	// not leader of view 8, go to view 9
	es.endView++
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	require.Contains(es.T(), es.forks.blocks, proposal.Block.BlockID, "proposal block should be stored")
	es.voteAggregator.AssertCalled(es.T(), "AddBlock", proposal)
}

// received a valid proposal for cur view, but not a safe node to vote, and I'm the next leader,
// no qc for the block
func (es *EventHandlerV2Suite) TestOnReceiveProposal_ForCurView_NoVote_IsNextLeader_NoQC() {
	proposal := createProposal(es.initView, es.initView-1)
	es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()

	// I'm the next leader
	es.committee.leaders[es.initView+1] = struct{}{}
	// no qc can be built for this block
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.voteAggregator.AssertCalled(es.T(), "AddBlock", proposal)
}

// received a valid proposal for cur view, but not a safe node to vote, and I'm the next leader,
// a qc can be built for the block, triggered view change
func (es *EventHandlerV2Suite) TestOnReceiveProposal_ForCurView_NoVote_IsNextLeader_QCBuilt_ViewChange() {
	proposal := createProposal(es.initView, es.initView-1)
	qc := createQC(proposal.Block)
	es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()
	// I'm the next leader
	es.committee.leaders[es.initView+1] = struct{}{}
	// qc triggered view change
	es.endView++
	// I'm the leader of cur view (7)
	// I'm not the leader of next view (8), trigger view change

	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)

	// after receiving proposal build QC and deliver it to event handler
	err = es.eventhandler.OnQCConstructed(qc)
	require.NoError(es.T(), err)

	lastCall := es.communicator.Calls[len(es.communicator.Calls)-1]
	// the last call is BroadcastProposal
	require.Equal(es.T(), "BroadcastProposalWithDelay", lastCall.Method)
	header, ok := lastCall.Arguments[0].(*flow.Header)
	require.True(es.T(), ok)
	// it should broadcast a header as the same as endView
	require.Equal(es.T(), es.endView, header.View)

	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.voteAggregator.AssertCalled(es.T(), "AddBlock", proposal)
}

// received a unverifiable proposal for future view, no view change
func (es *EventHandlerV2Suite) TestOnReceiveProposal_Unverifiable() {
	// qc.View is below the finalized view
	proposal := createProposal(es.forks.finalized+2, es.forks.finalized-1)

	es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()

	// proposal is unverifiable
	es.validator.unverifiable[proposal.Block.BlockID] = struct{}{}

	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.voteAggregator.AssertCalled(es.T(), "AddBlock", proposal)
}

func (es *EventHandlerV2Suite) TestOnTimeout() {
	err := es.eventhandler.OnLocalTimeout()
	// timeout will trigger viewchange
	es.endView++
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
}

func (es *EventHandlerV2Suite) Test100Timeout() {
	for i := 0; i < 100; i++ {
		err := es.eventhandler.OnLocalTimeout()
		es.endView++
		require.NoError(es.T(), err)
	}
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
}

// a leader builds 100 blocks one after another
func (es *EventHandlerV2Suite) TestLeaderBuild100Blocks() {
	// I'm the leader for the first view
	es.committee.leaders[es.initView] = struct{}{}

	totalView := 100
	for i := 0; i < totalView; i++ {
		// I'm the leader for 100 views
		// I'm the next leader
		es.committee.leaders[es.initView+uint64(i+1)] = struct{}{}
		// I can build qc for all 100 views
		proposal := createProposal(es.initView+uint64(i), es.initView+uint64(i)-1)
		qc := createQC(proposal.Block)

		es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()
		es.voteAggregator.On("AddVote", proposal.ProposerVote()).Return(nil).Once()

		es.voter.votable[proposal.Block.BlockID] = struct{}{}
		// should trigger 100 view change
		es.endView++

		err := es.eventhandler.OnReceiveProposal(proposal)
		require.NoError(es.T(), err)
		err = es.eventhandler.OnQCConstructed(qc)
		require.NoError(es.T(), err)

		lastCall := es.communicator.Calls[len(es.communicator.Calls)-1]
		require.Equal(es.T(), "BroadcastProposalWithDelay", lastCall.Method)
		header, ok := lastCall.Arguments[0].(*flow.Header)
		require.True(es.T(), ok)
		require.Equal(es.T(), proposal.Block.View+1, header.View)
	}

	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	require.Equal(es.T(), totalView, len(es.forks.blocks))
	es.voteAggregator.AssertExpectations(es.T())
}

// a follower receives 100 blocks
func (es *EventHandlerV2Suite) TestFollowerFollows100Blocks() {
	for i := 0; i < 100; i++ {
		// create each proposal as if they are created by some leader
		proposal := createProposal(es.initView+uint64(i), es.initView+uint64(i)-1)
		es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()
		// as a follower, I receive these propsals
		err := es.eventhandler.OnReceiveProposal(proposal)
		require.NoError(es.T(), err)
		es.endView++
	}
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	require.Equal(es.T(), 100, len(es.forks.blocks))
	es.voteAggregator.AssertExpectations(es.T())
}

// a follower receives 100 forks built on top of the same block
func (es *EventHandlerV2Suite) TestFollowerReceives100Forks() {
	for i := 0; i < 100; i++ {
		// create each proposal as if they are created by some leader
		proposal := createProposal(es.initView+uint64(i)+1, es.initView-1)
		es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()
		// as a follower, I receive these propsals
		err := es.eventhandler.OnReceiveProposal(proposal)
		require.NoError(es.T(), err)
	}
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	require.Equal(es.T(), 100, len(es.forks.blocks))
	es.voteAggregator.AssertExpectations(es.T())
}
