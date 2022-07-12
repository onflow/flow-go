package eventhandler

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/onflow/flow-go/model/flow"
)

const (
	startRepTimeout        float64 = 400.0 // Milliseconds
	minRepTimeout          float64 = 100.0 // Milliseconds
	maxRepTimeout          float64 = 600.0 // Milliseconds
	multiplicativeIncrease float64 = 1.5   // multiplicative factor
	multiplicativeDecrease float64 = 0.85  // multiplicative factor
)

// TestPaceMaker is a real pacemaker module with logging for view changes
type TestPaceMaker struct {
	hotstuff.PaceMaker
}

var _ hotstuff.PaceMaker = (*TestPaceMaker)(nil)

func NewTestPaceMaker(t require.TestingT, timeoutController *timeout.Controller,
	notifier hotstuff.Consumer,
	persist hotstuff.Persister,
) *TestPaceMaker {
	p, err := pacemaker.New(timeoutController, notifier, persist)
	if err != nil {
		panic(err)
	}
	return &TestPaceMaker{p}
}

func (p *TestPaceMaker) ProcessQC(qc *flow.QuorumCertificate) (*model.NewViewEvent, error) {
	oldView := p.CurView()
	newView, err := p.PaceMaker.ProcessQC(qc)
	log.Info().Msgf("pacemaker.ProcessQC old view: %v, new view: %v\n", oldView, p.CurView())
	return newView, err
}

func (p *TestPaceMaker) ProcessTC(tc *flow.TimeoutCertificate) (*model.NewViewEvent, error) {
	oldView := p.CurView()
	newView, err := p.PaceMaker.ProcessTC(tc)
	log.Info().Msgf("pacemaker.ProcessTC old view: %v, new view: %v\n", oldView, p.CurView())
	return newView, err
}

func (p *TestPaceMaker) NewestQC() *flow.QuorumCertificate {
	return p.PaceMaker.NewestQC()
}

func (p *TestPaceMaker) LastViewTC() *flow.TimeoutCertificate {
	return p.PaceMaker.LastViewTC()
}

// using a real pacemaker for testing event handler
func initPaceMaker(t require.TestingT, livenessData *hotstuff.LivenessData) hotstuff.PaceMaker {
	notifier := &mocks.Consumer{}
	tc, err := timeout.NewConfig(
		time.Duration(startRepTimeout*1e6),
		time.Duration(minRepTimeout*1e6),
		time.Duration(maxRepTimeout*1e6),
		multiplicativeIncrease,
		multiplicativeDecrease,
		0)
	require.NoError(t, err)
	persist := &mocks.Persister{}
	persist.On("PutLivenessData", mock.Anything).Return(nil).Maybe()
	persist.On("GetLivenessData").Return(livenessData, nil).Once()
	pm := NewTestPaceMaker(t, timeout.NewController(tc), notifier, persist)
	notifier.On("OnStartingTimeout", mock.Anything).Return()
	notifier.On("OnQcTriggeredViewChange", mock.Anything, mock.Anything).Return()
	notifier.On("OnTcTriggeredViewChange", mock.Anything, mock.Anything).Return()
	notifier.On("OnReachedTimeout", mock.Anything).Return()
	pm.Start()
	return pm
}

type Committee struct {
	mocks.DynamicCommittee
	// to mock I'm the leader of a certain view, add the view into the keys of leaders field
	leaders map[uint64]struct{}
}

func NewCommittee() *Committee {
	return &Committee{
		leaders: make(map[uint64]struct{}),
	}
}

func (c *Committee) LeaderForView(view uint64) (flow.Identifier, error) {
	_, isLeader := c.leaders[view]
	if isLeader {
		return flow.Identifier{0x01}, nil
	}
	return flow.Identifier{0x00}, nil
}

func (c *Committee) Self() flow.Identifier {
	return flow.Identifier{0x01}
}

// The SafetyRules mock will not vote for any block unless the block's ID exists in votable field's key
type SafetyRules struct {
	*mocks.SafetyRules
	votable map[flow.Identifier]struct{}
}

func NewSafetyRules(t *testing.T) *SafetyRules {
	safetyRules := &SafetyRules{
		SafetyRules: mocks.NewSafetyRules(t),
		votable:     make(map[flow.Identifier]struct{}),
	}

	// SafetyRules will not vote for any block, unless the blockID exists in votable map
	safetyRules.On("ProduceVote", mock.Anything, mock.Anything).Return(
		func(block *model.Proposal, _ uint64) *model.Vote {
			_, ok := safetyRules.votable[block.Block.BlockID]
			if !ok {
				return nil
			}
			return createVote(block.Block)
		},
		func(block *model.Proposal, _ uint64) error {
			_, ok := safetyRules.votable[block.Block.BlockID]
			if !ok {
				return model.NewNoVoteErrorf("block not found")
			}
			return nil
		}).Maybe()

	safetyRules.On("ProduceTimeout", mock.Anything, mock.Anything, mock.Anything).Return(
		func(curView uint64, newestQC *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) *model.TimeoutObject {
			return helper.TimeoutObjectFixture(func(timeout *model.TimeoutObject) {
				timeout.View = curView
				timeout.NewestQC = newestQC
				timeout.LastViewTC = lastViewTC
			})
		},
		func(uint64, *flow.QuorumCertificate, *flow.TimeoutCertificate) error { return nil }).Maybe()

	return safetyRules
}

// Forks mock allows to customize the AddBlock function by specifying the addProposal callbacks
type Forks struct {
	*mocks.Forks
	// proposals stores all the proposals that have been added to the forks
	proposals map[flow.Identifier]*model.Proposal
	finalized uint64
	t         require.TestingT
	// addProposal is to customize the logic to change finalized view
	addProposal func(block *model.Proposal) error
}

func NewForks(t *testing.T, finalized uint64) *Forks {
	f := &Forks{
		Forks:     mocks.NewForks(t),
		proposals: make(map[flow.Identifier]*model.Proposal),
		finalized: finalized,
	}

	f.On("AddProposal", mock.Anything).Return(func(proposal *model.Proposal) error {
		log.Info().Msgf("forks.AddProposal received Proposal for view: %v, qc: %v\n", proposal.Block.View, proposal.Block.QC.View)
		return f.addProposal(proposal)
	}).Maybe()

	f.On("FinalizedView").Return(func() uint64 {
		return f.finalized
	}).Maybe()

	f.On("GetProposal", mock.Anything).Return(func(blockID flow.Identifier) *model.Proposal {
		b := f.proposals[blockID]
		return b
	}, func(blockID flow.Identifier) bool {
		b, ok := f.proposals[blockID]
		var view uint64
		if ok {
			view = b.Block.View
		}
		log.Info().Msgf("forks.GetProposal found %v: view: %v\n", ok, view)
		return ok
	}).Maybe()

	f.On("GetProposalsForView", mock.Anything).Return(func(view uint64) []*model.Proposal {
		proposals := make([]*model.Proposal, 0)
		for _, b := range f.proposals {
			if b.Block.View == view {
				proposals = append(proposals, b)
			}
		}
		log.Info().Msgf("forks.GetProposalsForView found %v block(s) for view %v\n", len(proposals), view)
		return proposals
	}).Maybe()

	f.addProposal = func(proposal *model.Proposal) error {
		block := proposal.Block
		f.proposals[block.BlockID] = proposal
		if block.QC == nil {
			panic(fmt.Sprintf("block has no QC: %v", block.View))
		}
		return nil
	}

	return f
}

// BlockProducer mock will always make a valid block
type BlockProducer struct{}

func (b *BlockProducer) MakeBlockProposal(qc *flow.QuorumCertificate, view uint64, lastViewTC *flow.TimeoutCertificate) (*model.Proposal, error) {
	return &model.Proposal{
		Block:      helper.MakeBlock(helper.WithBlockView(view), helper.WithBlockQC(qc)),
		LastViewTC: lastViewTC,
	}, nil
}

func TestEventHandler(t *testing.T) {
	suite.Run(t, new(EventHandlerSuite))
}

// EventHandlerSuite contains mocked state for testing event handler under different scenarios.
type EventHandlerSuite struct {
	suite.Suite

	eventhandler *EventHandler

	paceMaker         hotstuff.PaceMaker
	forks             *Forks
	persist           *mocks.Persister
	blockProducer     *BlockProducer
	communicator      *mocks.Communicator
	committee         *Committee
	voteAggregator    *mocks.VoteAggregator
	timeoutAggregator *mocks.TimeoutAggregator
	safetyRules       *SafetyRules
	notifier          hotstuff.Consumer

	initView       uint64
	endView        uint64
	parentProposal *model.Proposal
	votingProposal *model.Proposal
	qc             *flow.QuorumCertificate
	tc             *flow.TimeoutCertificate
	newview        *model.NewViewEvent
}

func (es *EventHandlerSuite) SetupTest() {
	finalized := uint64(3)

	es.parentProposal = createProposal(4, 3)
	newestQC := createQC(es.parentProposal.Block)

	livenessData := &hotstuff.LivenessData{
		CurrentView: newestQC.View + 1,
		NewestQC:    newestQC,
	}

	es.paceMaker = initPaceMaker(es.T(), livenessData)
	es.forks = NewForks(es.T(), finalized)
	es.persist = mocks.NewPersister(es.T())
	es.persist.On("PutStarted", mock.Anything).Return(nil).Maybe()
	es.blockProducer = &BlockProducer{}
	es.communicator = mocks.NewCommunicator(es.T())
	es.communicator.On("BroadcastProposalWithDelay", mock.Anything, mock.Anything).Return(nil).Maybe()
	es.communicator.On("SendVote", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	es.communicator.On("BroadcastTimeout", mock.Anything).Return(nil).Maybe()
	es.committee = NewCommittee()
	es.voteAggregator = mocks.NewVoteAggregator(es.T())
	es.timeoutAggregator = mocks.NewTimeoutAggregator(es.T())
	es.safetyRules = NewSafetyRules(es.T())
	es.notifier = &notifications.NoopConsumer{}

	eventhandler, err := NewEventHandler(
		zerolog.New(os.Stderr),
		es.paceMaker,
		es.blockProducer,
		es.forks,
		es.persist,
		es.communicator,
		es.committee,
		es.voteAggregator,
		es.timeoutAggregator,
		es.safetyRules,
		es.notifier)
	require.NoError(es.T(), err)

	es.eventhandler = eventhandler

	es.initView = livenessData.CurrentView
	es.endView = livenessData.CurrentView
	// voting block is a block for the current view, which will trigger view change
	es.votingProposal = helper.MakeProposal(
		helper.WithBlock(
			helper.MakeBlock(
				helper.WithBlockView(es.paceMaker.CurView()),
				helper.WithParentBlock(es.parentProposal.Block))))
	es.qc = helper.MakeQC(helper.WithQCBlock(es.votingProposal.Block))

	// create a TC that will trigger view change for current view, based on newest QC
	es.tc = helper.MakeTC(helper.WithTCView(es.paceMaker.CurView()),
		helper.WithTCNewestQC(es.votingProposal.Block.QC))
	es.newview = &model.NewViewEvent{
		View: es.votingProposal.Block.View + 1, // the vote for the voting proposals will trigger a view change to the next view
	}

	// add es.parentProposal into forks, otherwise we won't vote or propose based on it's QC sicne the parent is unknown
	es.forks.proposals[es.parentProposal.Block.BlockID] = es.parentProposal
}

// TestStartNewView_ParentProposalNotFound tests next scenario: constructed TC, it contains NewestQC that references block that we
// don't know about, proposal can't be generated because we can't be sure that resulting block payload is valid.
func (es *EventHandlerSuite) TestStartNewView_ParentProposalNotFound() {
	newestQC := helper.MakeQC(helper.WithQCView(es.initView + 10))
	tc := helper.MakeTC(helper.WithTCView(newestQC.View+1),
		helper.WithTCNewestQC(newestQC))

	es.endView = tc.View + 1

	// I'm leader for next block
	es.committee.leaders[es.endView] = struct{}{}

	err := es.eventhandler.OnTCConstructed(tc)
	require.NoError(es.T(), err)

	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.forks.AssertCalled(es.T(), "GetProposal", newestQC.BlockID)
	es.communicator.AssertNotCalled(es.T(), "BroadcastProposalWithDelay")
}

// TestOnReceiveProposal_StaleProposal test that proposals lower than finalized view are not processed at all
// we are not interested in this data because we already performed finalization of that height.
func (es *EventHandlerSuite) TestOnReceiveProposal_StaleProposal() {
	proposal := createProposal(es.forks.FinalizedView(), es.forks.FinalizedView()-1)
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	es.voteAggregator.AssertNotCalled(es.T(), "AddBlock", proposal)
}

// TestOnReceiveProposal_QCOlderThanCurView tests scenario: received a valid proposal with QC that has older view,
// the proposal's QC shouldn't trigger view change.
func (es *EventHandlerSuite) TestOnReceiveProposal_QCOlderThanCurView() {
	proposal := createProposal(es.initView-1, es.initView-2)
	es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()

	// should not trigger view change
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.voteAggregator.AssertCalled(es.T(), "AddBlock", proposal)
}

// TestOnReceiveProposal_TCOlderThanCurView tests scenario: received a valid proposal with QC and TC that has older view,
// the proposal's QC shouldn't trigger view change.
func (es *EventHandlerSuite) TestOnReceiveProposal_TCOlderThanCurView() {
	proposal := createProposal(es.initView-1, es.initView-3)
	proposal.LastViewTC = helper.MakeTC(helper.WithTCView(proposal.Block.View-1), helper.WithTCNewestQC(proposal.Block.QC))
	es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()

	// should not trigger view change
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.voteAggregator.AssertCalled(es.T(), "AddBlock", proposal)
}

// TestOnReceiveProposal_NoVote tests scenario: received a valid proposal for cur view, but not a safe node to vote, and I'm the next leader
// should not vote.
func (es *EventHandlerSuite) TestOnReceiveProposal_NoVote() {
	proposal := createProposal(es.initView, es.initView-1)
	es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()

	// I'm the next leader
	es.committee.leaders[es.initView+1] = struct{}{}
	// no vote for this proposal
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.voteAggregator.AssertCalled(es.T(), "AddBlock", proposal)
}

// TestOnReceiveProposal_NoVote_ParentProposalNotFound tests scenario: received a valid proposal for cur view, no parent for this proposal found
// should not vote.
func (es *EventHandlerSuite) TestOnReceiveProposal_NoVote_ParentProposalNotFound() {
	proposal := createProposal(es.initView, es.initView-1)
	es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()

	// remove parent from known proposals
	delete(es.forks.proposals, proposal.Block.QC.BlockID)

	// no vote for this proposal, no parent found
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.voteAggregator.AssertCalled(es.T(), "AddBlock", proposal)
}

// TestOnReceiveProposal_Vote_NextLeader tests scenario: received a valid proposal for cur view, safe to vote, I'm the next leader
// should vote and add vote to VoteAggregator.
func (es *EventHandlerSuite) TestOnReceiveProposal_Vote_NextLeader() {
	proposal := createProposal(es.initView, es.initView-1)
	es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()
	es.voteAggregator.On("AddVote", mock.Anything).Return().Once()

	// I'm the next leader
	es.committee.leaders[es.initView+1] = struct{}{}

	// proposal is safe to vote
	es.safetyRules.votable[proposal.Block.BlockID] = struct{}{}

	// vote should be created for this proposal
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
}

// TestOnReceiveProposal_Vote_NextLeader tests scenario: received a valid proposal for cur view, safe to vote, I'm not the next leader
// should vote and send vote to next leader.
func (es *EventHandlerSuite) TestOnReceiveProposal_Vote_NotNextLeader() {
	proposal := createProposal(es.initView, es.initView-1)
	es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()

	// proposal is safe to vote
	es.safetyRules.votable[proposal.Block.BlockID] = struct{}{}

	// vote should be created for this proposal
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")

	lastCall := es.communicator.Calls[len(es.communicator.Calls)-1]
	// the last call is SendVote
	require.Equal(es.T(), "SendVote", lastCall.Method)
	blockID, ok := lastCall.Arguments[0].(flow.Identifier)
	require.True(es.T(), ok)
	require.Equal(es.T(), proposal.Block.BlockID, blockID)
}

// TestOnQCConstructed_HappyPath tests that building a QC for current view triggers view change. We are not leader for next
// round, so no proposal is expected.
func (es *EventHandlerSuite) TestOnQCConstructed_HappyPath() {
	// voting block exists
	es.forks.proposals[es.votingProposal.Block.BlockID] = es.votingProposal

	// a qc is built
	qc := createQC(es.votingProposal.Block)

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
	es.communicator.AssertNotCalled(es.T(), "BroadcastProposalWithDelay")
}

// TestOnQCConstructed_FutureView tests that building a QC for future view triggers view change
func (es *EventHandlerSuite) TestOnQCConstructed_FutureView() {
	// voting block exists
	curView := es.paceMaker.CurView()

	// b1 is for current view
	// b2 and b3 is for future view, but branched out from the same parent as b1
	b1 := createProposal(curView, curView-1)
	b2 := createProposal(curView+1, curView-1)
	b3 := createProposal(curView+2, curView-1)

	// a qc is built
	// qc3 is for future view
	// qc2 is an older than qc3
	// since vote aggregator can concurrently process votes and build qcs,
	// we prepare qcs at different view to be processed, and verify the view change.
	qc1 := createQC(b1.Block)
	qc2 := createQC(b2.Block)
	qc3 := createQC(b3.Block)

	// all three proposals are known
	es.forks.proposals[b1.Block.BlockID] = b1
	es.forks.proposals[b2.Block.BlockID] = b2
	es.forks.proposals[b3.Block.BlockID] = b3

	// test that qc for future view should trigger view change
	err := es.eventhandler.OnQCConstructed(qc3)
	endView := b3.Block.View + 1 // next view
	require.NoError(es.T(), err, "if a vote can trigger a QC to be built,"+
		"and the QC triggered a view change, then start new view")
	require.Equal(es.T(), endView, es.paceMaker.CurView(), "incorrect view change")

	// the same qc would not trigger view change
	err = es.eventhandler.OnQCConstructed(qc3)
	endView = b3.Block.View + 1 // next view
	require.NoError(es.T(), err, "same qc should not trigger view change")
	require.Equal(es.T(), endView, es.paceMaker.CurView(), "incorrect view change")

	// old QCs won't trigger view change
	err = es.eventhandler.OnQCConstructed(qc2)
	require.NoError(es.T(), err)
	require.Equal(es.T(), endView, es.paceMaker.CurView(), "incorrect view change")

	err = es.eventhandler.OnQCConstructed(qc1)
	require.NoError(es.T(), err)
	require.Equal(es.T(), endView, es.paceMaker.CurView(), "incorrect view change")
}

// TestOnQCConstructed_NextLeaderProposes tests that after receiving a valid proposal for cur view, and I'm the next leader,
// a QC can be built for the block, triggered view change, and I will propose
func (es *EventHandlerSuite) TestOnQCConstructed_NextLeaderProposes() {
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

// TestOnTCConstructed_HappyPath tests that building a TC for current view triggers view change
func (es *EventHandlerSuite) TestOnTCConstructed_HappyPath() {
	// voting block exists
	es.forks.proposals[es.votingProposal.Block.BlockID] = es.votingProposal

	// a qc is built
	tc := helper.MakeTC(helper.WithTCView(es.initView), helper.WithTCNewestQC(es.qc))

	// expect a view change
	es.endView++

	err := es.eventhandler.OnTCConstructed(tc)
	require.NoError(es.T(), err, "TC should trigger a view change and start of new view")
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
}

// TestOnTCConstructed_NextLeaderProposes tests that after receiving TC and advancing view we as next leader create a proposal
// and broadcast it
func (es *EventHandlerSuite) TestOnTCConstructed_NextLeaderProposes() {
	es.committee.leaders[es.tc.View+1] = struct{}{}
	es.endView++
	err := es.eventhandler.OnTCConstructed(es.tc)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "TC didn't trigger view change")

	lastCall := es.communicator.Calls[len(es.communicator.Calls)-1]
	// the last call is BroadcastProposal
	require.Equal(es.T(), "BroadcastProposalWithDelay", lastCall.Method)
	header, ok := lastCall.Arguments[0].(*flow.Header)
	require.True(es.T(), ok)
	// it should broadcast a header as the same as endView
	require.Equal(es.T(), es.endView, header.View)

	// proposed block should contain valid newest QC and lastViewTC
	expectedNewestQC := es.paceMaker.NewestQC()
	proposal := model.ProposalFromFlow(header, expectedNewestQC.View)
	require.Equal(es.T(), expectedNewestQC, proposal.Block.QC)
	require.Equal(es.T(), es.paceMaker.LastViewTC(), proposal.LastViewTC)
}

// TestOnTimeout tests that event handler produces TimeoutObject and broadcasts it to other members of consensus
// committee. Additionally, It has to contribute TimeoutObject to timeout aggregation process by sending it to TimeoutAggregator.
func (es *EventHandlerSuite) TestOnTimeout() {

	es.timeoutAggregator.On("AddTimeout", mock.Anything).Return().Once()

	err := es.eventhandler.OnLocalTimeout()
	require.NoError(es.T(), err)

	// TimeoutObject shouldn't trigger view change
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")

	lastCall := es.communicator.Calls[len(es.communicator.Calls)-1]
	// the last call is BroadcastProposal
	require.Equal(es.T(), "BroadcastTimeout", lastCall.Method)
	timeoutObject, ok := lastCall.Arguments[0].(*model.TimeoutObject)
	require.True(es.T(), ok)
	// it should broadcast a TO with same view as endView
	require.Equal(es.T(), es.endView, timeoutObject.View)
}

// Test100Timeout tests that receiving 100 TCs for increasing views advances rounds
func (es *EventHandlerSuite) Test100Timeout() {
	for i := 0; i < 100; i++ {
		tc := helper.MakeTC(helper.WithTCView(es.initView + uint64(i)))
		err := es.eventhandler.OnTCConstructed(tc)
		es.endView++
		require.NoError(es.T(), err)
	}
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
}

// TestLeaderBuild100Blocks tests scenario where leader builds 100 proposals one after another
func (es *EventHandlerSuite) TestLeaderBuild100Blocks() {
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

		// for first proposal we need to store the parent otherwise it won't be voted for
		if i == 0 {
			parentBlock := helper.MakeProposal(
				helper.WithBlock(
					helper.MakeBlock(func(block *model.Block) {
						block.BlockID = proposal.Block.QC.BlockID
						block.View = proposal.Block.QC.View
					})))
			es.forks.proposals[parentBlock.Block.BlockID] = parentBlock
		}

		es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()
		es.voteAggregator.On("AddVote", proposal.ProposerVote()).Return(nil).Once()

		es.safetyRules.votable[proposal.Block.BlockID] = struct{}{}
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
	require.Equal(es.T(), totalView, len(es.forks.proposals)-1)
	es.voteAggregator.AssertExpectations(es.T())
}

// TestFollowerFollows100Blocks tests scenario where follower receives 100 proposals one after another
func (es *EventHandlerSuite) TestFollowerFollows100Blocks() {
	for i := 0; i < 100; i++ {
		// create each proposal as if they are created by some leader
		proposal := createProposal(es.initView+uint64(i)+1, es.initView+uint64(i))
		es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()
		// as a follower, I receive these proposals
		err := es.eventhandler.OnReceiveProposal(proposal)
		require.NoError(es.T(), err)
		es.endView++
	}
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	require.Equal(es.T(), 100, len(es.forks.proposals)-1)
}

// TestFollowerReceives100Forks tests scenario where follower receives 100 forks built on top of the same block
func (es *EventHandlerSuite) TestFollowerReceives100Forks() {
	for i := 0; i < 100; i++ {
		// create each proposal as if they are created by some leader
		proposal := createProposal(es.initView+uint64(i)+1, es.initView-1)
		proposal.LastViewTC = helper.MakeTC(helper.WithTCView(es.initView+uint64(i)),
			helper.WithTCNewestQC(proposal.Block.QC))
		// expect a view change since fork can be made only if last view has ended with TC.
		es.endView++
		es.voteAggregator.On("AddBlock", proposal).Return(nil).Once()
		// as a follower, I receive these proposals
		err := es.eventhandler.OnReceiveProposal(proposal)
		require.NoError(es.T(), err)
	}
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	require.Equal(es.T(), 100, len(es.forks.proposals)-1)
}

// TestStart_PendingBlocksRecovery tests a scenario where node has unprocessed pending proposals that were not processed
// by event handler yet. After startup, we need to process all pending proposals.
func (es *EventHandlerSuite) TestStart_PendingBlocksRecovery() {

	var pendingProposals []*model.Proposal
	proposal := createProposal(es.initView+1, es.initView)
	pendingProposals = append(pendingProposals, proposal)
	proposalWithTC := helper.MakeProposal(helper.WithBlock(
		helper.MakeBlock(
			helper.WithBlockView(es.initView+10),
			helper.WithBlockQC(proposal.Block.QC))),
		func(proposal *model.Proposal) {
			proposal.LastViewTC = helper.MakeTC(
				helper.WithTCView(proposal.Block.View-1),
				helper.WithTCNewestQC(proposal.Block.QC))
		},
	)
	pendingProposals = append(pendingProposals, proposalWithTC)
	proposal = createProposal(proposalWithTC.Block.View+1, proposalWithTC.Block.View)
	pendingProposals = append(pendingProposals, proposal)

	for _, proposal := range pendingProposals {
		es.forks.proposals[proposal.Block.BlockID] = proposal
	}

	lastProposal := pendingProposals[len(pendingProposals)-1]
	es.endView = lastProposal.Block.View

	es.forks.On("NewestView").Return(es.endView).Once()

	err := es.eventhandler.Start()
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
}

// TestCreateProposal_SanityChecks tests that proposing logic performs sanity checks when creating new block proposal.
// Specifically it tests a case where TC contains QC which: TC.View == TC.NewestQC.View
func (es *EventHandlerSuite) TestCreateProposal_SanityChecks() {
	// round ended with TC where TC.View == TC.NewestQC.View
	tc := helper.MakeTC(helper.WithTCView(es.initView),
		helper.WithTCNewestQC(helper.MakeQC(helper.WithQCBlock(es.votingProposal.Block))))

	es.forks.proposals[es.votingProposal.Block.BlockID] = es.votingProposal

	// I'm the next leader
	es.committee.leaders[tc.View+1] = struct{}{}

	err := es.eventhandler.OnTCConstructed(tc)
	require.NoError(es.T(), err)

	lastCall := es.communicator.Calls[len(es.communicator.Calls)-1]
	// the last call is BroadcastProposal
	require.Equal(es.T(), "BroadcastProposalWithDelay", lastCall.Method)
	header, ok := lastCall.Arguments[0].(*flow.Header)
	require.True(es.T(), ok)
	// we need to make sure that produced proposal contains only QC even if there is TC for previous view as well
	require.Nil(es.T(), header.LastViewTC)

	require.Equal(es.T(), tc.NewestQC, es.paceMaker.NewestQC())
	require.Equal(es.T(), tc, es.paceMaker.LastViewTC())
	require.Equal(es.T(), tc.View+1, es.paceMaker.CurView(), "incorrect view change")
}

func createBlock(view uint64) *model.Block {
	blockID := flow.MakeID(struct {
		BlockID uint64
	}{
		BlockID: view,
	})
	return &model.Block{
		BlockID: blockID,
		View:    view,
	}
}

func createBlockWithQC(view uint64, qcview uint64) *model.Block {
	block := createBlock(view)
	parent := createBlock(qcview)
	block.QC = createQC(parent)
	return block
}

func createQC(parent *model.Block) *flow.QuorumCertificate {
	qc := &flow.QuorumCertificate{
		BlockID:       parent.BlockID,
		View:          parent.View,
		SignerIndices: nil,
		SigData:       nil,
	}
	return qc
}

func createVote(block *model.Block) *model.Vote {
	return &model.Vote{
		View:     block.View,
		BlockID:  block.BlockID,
		SignerID: flow.ZeroID,
		SigData:  nil,
	}
}

func createProposal(view uint64, qcview uint64) *model.Proposal {
	block := createBlockWithQC(view, qcview)
	return &model.Proposal{
		Block:   block,
		SigData: nil,
	}
}
