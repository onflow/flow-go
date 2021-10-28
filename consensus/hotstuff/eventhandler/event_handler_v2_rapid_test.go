package eventhandler_test

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/eventhandler"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
)

// func TestEventHandlerV2(t *testing.T) {
// 	suite.Run(t, new(EventHandlerV2Suite))
// }
//
// // EventHandlerV2Suite contains mocked state for testing event handler under different scenarios.
// type EventHandlerV2Suite struct {
// 	suite.Suite
//
// 	eventhandler *eventhandler.EventHandlerV2
//
// 	paceMaker      hotstuff.PaceMaker
// 	forks          *Forks
// 	persist        *mocks.Persister
// 	blockProducer  *BlockProducer
// 	communicator   *mocks.Communicator
// 	committee      *Committee
// 	voteAggregator *mocks.VoteAggregatorV2
// 	voter          *Voter
// 	validator      *BlacklistValidator
// 	notifier       hotstuff.Consumer
//
// 	initView    uint64
// 	endView     uint64
// 	votingBlock *model.Block
// 	qc          *flow.QuorumCertificate
// 	newview     *model.NewViewEvent
// }
//
// func (es *EventHandlerV2Suite) SetupTest() {
// 	finalized, curView := uint64(3), uint64(6)
//
// 	es.paceMaker = initPaceMaker(es.T(), curView)
// 	es.forks = NewForks(es.T(), finalized)
// 	es.persist = &mocks.Persister{}
// 	es.persist.On("PutStarted", mock.Anything).Return(nil)
// 	es.blockProducer = &BlockProducer{}
// 	es.communicator = &mocks.Communicator{}
// 	es.communicator.On("BroadcastProposalWithDelay", mock.Anything, mock.Anything).Return(nil)
// 	es.communicator.On("SendVote", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
// 	es.committee = NewCommittee()
// 	es.voteAggregator = &mocks.VoteAggregatorV2{}
// 	es.voter = NewVoter(es.T(), finalized)
// 	es.validator = NewBlacklistValidator(es.T())
// 	es.notifier = &notifications.NoopConsumer{}
//
// 	eventhandler, err := eventhandler.NewEventHandlerV2(
// 		zerolog.New(os.Stderr),
// 		es.paceMaker,
// 		es.blockProducer,
// 		es.forks,
// 		es.persist,
// 		es.communicator,
// 		es.committee,
// 		es.voteAggregator,
// 		es.voter,
// 		es.validator,
// 		es.notifier)
// 	require.NoError(es.T(), err)
//
// 	es.eventhandler = eventhandler
//
// 	es.initView = curView
// 	es.endView = curView
// 	// voting block is a block for the current view, which will trigger view change
// 	es.votingBlock = createBlockWithQC(es.paceMaker.CurView(), es.paceMaker.CurView()-1)
// 	es.qc = &flow.QuorumCertificate{
// 		BlockID:   es.votingBlock.BlockID,
// 		View:      es.votingBlock.View,
// 		SignerIDs: nil,
// 		SigData:   nil,
// 	}
// 	es.newview = &model.NewViewEvent{
// 		View: es.votingBlock.View + 1, // the vote for the voting blocks will trigger a view change to the next view
// 	}
// }

// func (es *EventHandlerV2Suite) becomeLeaderForView(view uint64) {
// 	es.committee.leaders[view] = struct{}{}
// }
//
// func (es *EventHandlerV2Suite) markInvalidProposal(blockID flow.Identifier) {
// 	es.validator.invalidProposals[blockID] = struct{}{}
// }

type rapidStuff struct {
	handler   *eventhandler.EventHandlerV2
	pacemaker hotstuff.PaceMaker

	curViewChanged  bool
	expectedCurView uint64
}

func (r *rapidStuff) Init(t *rapid.T) {
	finalized, curView := uint64(3), uint64(6)

	pacemaker := initPaceMaker(t, curView)
	forks := NewForks(t, finalized)
	persist := &mocks.Persister{}
	persist.On("PutStarted", mock.Anything).Return(nil)
	blockProducer := &BlockProducer{}
	communicator := &mocks.Communicator{}
	communicator.On("BroadcastProposalWithDelay", mock.Anything, mock.Anything).Return(nil)
	communicator.On("SendVote", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	committee := NewCommittee()
	voteAggregator := &mocks.VoteAggregatorV2{}
	voter := NewVoter(t, finalized)
	validator := NewBlacklistValidator(t)
	notifier := &notifications.NoopConsumer{}

	eventhandler, err := eventhandler.NewEventHandlerV2(
		zerolog.New(os.Stderr),
		pacemaker,
		blockProducer,
		forks,
		persist,
		communicator,
		committee,
		voteAggregator,
		voter,
		validator,
		notifier)
	require.NoError(t, err)

	r.handler = eventhandler
	r.pacemaker = pacemaker
}

// randomly receive a timeout, which will increment the current view
func (r *rapidStuff) OnTimeout(t *rapid.T) {
	curView := r.pacemaker.CurView()
	r.handler.OnLocalTimeout()
	r.curViewChanged = true
	r.expectedCurView = curView + 1
}

// ramdonly receive a QC for a view where I'm the leader, if QC's block is above the current view,
// then it should increment the view

// ramdonly receive a QC for a view where I'm the leader and is the current view,
// which should increment the current view

// randomly receive a block that extends from the last block

// ramdomly receive a double proposal block that extends from the parent of the last block,
// and has the same view as the last block, should not vote for the block

// randomly receive a invalid block
func (r *rapidStuff) OnBlock(t *rapid.T) {

}

func (r *rapidStuff) Check(t *rapid.T) {
	if r.curViewChanged {
		require.Equal(t, r.expectedCurView, r.pacemaker.CurView())
		r.curViewChanged = false
	}
}

func TestEventHandlerRapid(t *testing.T) {
	rapid.Check(t, rapid.Run(&rapidStuff{}))
}
