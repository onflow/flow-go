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
	vote        *model.Vote
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
	es.vote = &model.Vote{
		BlockID:  es.votingBlock.BlockID,
		View:     es.votingBlock.View,
		SignerID: flow.ZeroID,
		SigData:  nil,
	}
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
// a qc can be built for the block, trigged view change
func (es *EventHandlerV2Suite) TestOnReceiveProposal_ForCurView_NoVote_IsNextLeader_QCBuilt_ViewChange() {
	proposal := createProposal(es.initView, es.initView-1)
	es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()
	// I'm the next leader
	es.committee.leaders[es.initView+1] = struct{}{}
	// a qc can be built for this block
	//es.voteAggregator.qcs[proposal.Block.BlockID] = createQC(proposal.Block)
	// qc triggered view change
	es.endView++
	// I'm the leader of cur view (7)
	// I'm not the leader of next view (8), trigger view change

	err := es.eventhandler.OnReceiveProposal(proposal)
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
		//es.voteAggregator.qcs[proposal.Block.BlockID] = createQC(proposal.Block)
		es.voter.votable[proposal.Block.BlockID] = struct{}{}
		// should trigger 100 view change
		es.endView++

		err := es.eventhandler.OnReceiveProposal(proposal)
		require.NoError(es.T(), err)
		lastCall := es.communicator.Calls[len(es.communicator.Calls)-1]
		require.Equal(es.T(), "BroadcastProposalWithDelay", lastCall.Method)
		header, ok := lastCall.Arguments[0].(*flow.Header)
		require.True(es.T(), ok)
		require.Equal(es.T(), proposal.Block.View+1, header.View)
	}

	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	require.Equal(es.T(), totalView, len(es.forks.blocks))
}

// a follower receives 100 blocks
func (es *EventHandlerV2Suite) TestFollowerFollows100Blocks() {
	for i := 0; i < 100; i++ {
		// create each proposal as if they are created by some leader
		proposal := createProposal(es.initView+uint64(i), es.initView+uint64(i)-1)
		// as a follower, I receive these propsals
		err := es.eventhandler.OnReceiveProposal(proposal)
		require.NoError(es.T(), err)
		es.endView++
	}
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	require.Equal(es.T(), 100, len(es.forks.blocks))
}

// a follower receives 100 forks built on top of the same block
func (es *EventHandlerV2Suite) TestFollowerReceives100Forks() {
	for i := 0; i < 100; i++ {
		// create each proposal as if they are created by some leader
		proposal := createProposal(es.initView+uint64(i)+1, es.initView-1)
		// as a follower, I receive these propsals
		err := es.eventhandler.OnReceiveProposal(proposal)
		require.NoError(es.T(), err)
	}
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	require.Equal(es.T(), 100, len(es.forks.blocks))
}
