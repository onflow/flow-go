package eventhandler

import (
	"context"
	"errors"
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
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	minRepTimeout             float64 = 100.0 // Milliseconds
	maxRepTimeout             float64 = 600.0 // Milliseconds
	multiplicativeIncrease    float64 = 1.5   // multiplicative factor
	happyPathMaxRoundFailures uint64  = 6     // number of failed rounds before first timeout increase
)

// TestPaceMaker is a real pacemaker module with logging for view changes
type TestPaceMaker struct {
	hotstuff.PaceMaker
}

var _ hotstuff.PaceMaker = (*TestPaceMaker)(nil)

func NewTestPaceMaker(
	timeoutController *timeout.Controller,
	proposalDelayProvider hotstuff.ProposalDurationProvider,
	notifier hotstuff.Consumer,
	persist hotstuff.Persister,
) *TestPaceMaker {
	p, err := pacemaker.New(timeoutController, proposalDelayProvider, notifier, persist)
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
func initPaceMaker(t require.TestingT, ctx context.Context, livenessData *hotstuff.LivenessData) hotstuff.PaceMaker {
	notifier := &mocks.Consumer{}
	tc, err := timeout.NewConfig(time.Duration(minRepTimeout*1e6), time.Duration(maxRepTimeout*1e6), multiplicativeIncrease, happyPathMaxRoundFailures, time.Duration(maxRepTimeout*1e6))
	require.NoError(t, err)
	persist := &mocks.Persister{}
	persist.On("PutLivenessData", mock.Anything).Return(nil).Maybe()
	persist.On("GetLivenessData").Return(livenessData, nil).Once()
	pm := NewTestPaceMaker(timeout.NewController(tc), pacemaker.NoProposalDelay(), notifier, persist)
	notifier.On("OnStartingTimeout", mock.Anything).Return()
	notifier.On("OnQcTriggeredViewChange", mock.Anything, mock.Anything, mock.Anything).Return()
	notifier.On("OnTcTriggeredViewChange", mock.Anything, mock.Anything, mock.Anything).Return()
	notifier.On("OnViewChange", mock.Anything, mock.Anything).Maybe()
	pm.Start(ctx)
	return pm
}

// Committee mocks hotstuff.DynamicCommittee and allows to easily control leader for some view.
type Committee struct {
	*mocks.Replicas
	// to mock I'm the leader of a certain view, add the view into the keys of leaders field
	leaders map[uint64]struct{}
}

func NewCommittee(t *testing.T) *Committee {
	committee := &Committee{
		Replicas: mocks.NewReplicas(t),
		leaders:  make(map[uint64]struct{}),
	}
	self := unittest.IdentityFixture(unittest.WithNodeID(flow.Identifier{0x01}))
	committee.On("LeaderForView", mock.Anything).Return(func(view uint64) flow.Identifier {
		_, isLeader := committee.leaders[view]
		if isLeader {
			return self.NodeID
		}
		return flow.Identifier{0x00}
	}, func(view uint64) error {
		return nil
	}).Maybe()

	committee.On("Self").Return(self.NodeID).Maybe()

	return committee
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
	proposals map[flow.Identifier]*model.Block
	finalized uint64
	t         require.TestingT
	// addProposal is to customize the logic to change finalized view
	addProposal func(block *model.Block) error
}

func NewForks(t *testing.T, finalized uint64) *Forks {
	f := &Forks{
		Forks:     mocks.NewForks(t),
		proposals: make(map[flow.Identifier]*model.Block),
		finalized: finalized,
	}

	f.On("AddValidatedBlock", mock.Anything).Return(func(proposal *model.Block) error {
		log.Info().Msgf("forks.AddValidatedBlock received Proposal for view: %v, QC: %v\n", proposal.View, proposal.QC.View)
		return f.addProposal(proposal)
	}).Maybe()

	f.On("FinalizedView").Return(func() uint64 {
		return f.finalized
	}).Maybe()

	f.On("GetBlock", mock.Anything).Return(func(blockID flow.Identifier) *model.Block {
		b := f.proposals[blockID]
		return b
	}, func(blockID flow.Identifier) bool {
		b, ok := f.proposals[blockID]
		var view uint64
		if ok {
			view = b.View
		}
		log.Info().Msgf("forks.GetBlock found %v: view: %v\n", ok, view)
		return ok
	}).Maybe()

	f.On("GetBlocksForView", mock.Anything).Return(func(view uint64) []*model.Block {
		proposals := make([]*model.Block, 0)
		for _, b := range f.proposals {
			if b.View == view {
				proposals = append(proposals, b)
			}
		}
		log.Info().Msgf("forks.GetBlocksForView found %v block(s) for view %v\n", len(proposals), view)
		return proposals
	}).Maybe()

	f.addProposal = func(block *model.Block) error {
		f.proposals[block.BlockID] = block
		if block.QC == nil {
			panic(fmt.Sprintf("block has no QC: %v", block.View))
		}
		return nil
	}

	return f
}

// BlockProducer mock will always make a valid block
type BlockProducer struct {
	proposerID flow.Identifier
}

func (b *BlockProducer) MakeBlockProposal(view uint64, qc *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) (*flow.Header, error) {
	return model.ProposalToFlow(&model.Proposal{
		Block: helper.MakeBlock(
			helper.WithBlockView(view),
			helper.WithBlockQC(qc),
			helper.WithBlockProposer(b.proposerID),
		),
		LastViewTC: lastViewTC,
	}), nil
}

func TestEventHandler(t *testing.T) {
	suite.Run(t, new(EventHandlerSuite))
}

// EventHandlerSuite contains mocked state for testing event handler under different scenarios.
type EventHandlerSuite struct {
	suite.Suite

	eventhandler *EventHandler

	paceMaker     hotstuff.PaceMaker
	forks         *Forks
	persist       *mocks.Persister
	blockProducer *BlockProducer
	committee     *Committee
	notifier      *mocks.Consumer
	safetyRules   *SafetyRules

	initView       uint64 // the current view at the beginning of the test case
	endView        uint64 // the expected current view at the end of the test case
	parentProposal *model.Proposal
	votingProposal *model.Proposal
	qc             *flow.QuorumCertificate
	tc             *flow.TimeoutCertificate
	newview        *model.NewViewEvent
	ctx            context.Context
	stop           context.CancelFunc
}

func (es *EventHandlerSuite) SetupTest() {
	finalized := uint64(3)

	es.parentProposal = createProposal(4, 3)
	newestQC := createQC(es.parentProposal.Block)

	livenessData := &hotstuff.LivenessData{
		CurrentView: newestQC.View + 1,
		NewestQC:    newestQC,
	}

	es.ctx, es.stop = context.WithCancel(context.Background())

	es.committee = NewCommittee(es.T())
	es.paceMaker = initPaceMaker(es.T(), es.ctx, livenessData)
	es.forks = NewForks(es.T(), finalized)
	es.persist = mocks.NewPersister(es.T())
	es.persist.On("PutStarted", mock.Anything).Return(nil).Maybe()
	es.blockProducer = &BlockProducer{proposerID: es.committee.Self()}
	es.safetyRules = NewSafetyRules(es.T())
	es.notifier = mocks.NewConsumer(es.T())
	es.notifier.On("OnEventProcessed").Maybe()
	es.notifier.On("OnEnteringView", mock.Anything, mock.Anything).Maybe()
	es.notifier.On("OnStart", mock.Anything).Maybe()
	es.notifier.On("OnReceiveProposal", mock.Anything, mock.Anything).Maybe()
	es.notifier.On("OnReceiveQc", mock.Anything, mock.Anything).Maybe()
	es.notifier.On("OnReceiveTc", mock.Anything, mock.Anything).Maybe()
	es.notifier.On("OnPartialTc", mock.Anything, mock.Anything).Maybe()
	es.notifier.On("OnLocalTimeout", mock.Anything).Maybe()
	es.notifier.On("OnCurrentViewDetails", mock.Anything, mock.Anything, mock.Anything).Maybe()

	eventhandler, err := NewEventHandler(
		zerolog.New(os.Stderr),
		es.paceMaker,
		es.blockProducer,
		es.forks,
		es.persist,
		es.committee,
		es.safetyRules,
		es.notifier)
	require.NoError(es.T(), err)

	es.eventhandler = eventhandler

	es.initView = livenessData.CurrentView
	es.endView = livenessData.CurrentView
	// voting block is a block for the current view, which will trigger view change
	es.votingProposal = createProposal(es.paceMaker.CurView(), es.parentProposal.Block.View)
	es.qc = helper.MakeQC(helper.WithQCBlock(es.votingProposal.Block))

	// create a TC that will trigger view change for current view, based on newest QC
	es.tc = helper.MakeTC(helper.WithTCView(es.paceMaker.CurView()),
		helper.WithTCNewestQC(es.votingProposal.Block.QC))
	es.newview = &model.NewViewEvent{
		View: es.votingProposal.Block.View + 1, // the vote for the voting proposals will trigger a view change to the next view
	}

	// add es.parentProposal into forks, otherwise we won't vote or propose based on it's QC sicne the parent is unknown
	es.forks.proposals[es.parentProposal.Block.BlockID] = es.parentProposal.Block
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

	err := es.eventhandler.OnReceiveTc(tc)
	require.NoError(es.T(), err)

	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.forks.AssertCalled(es.T(), "GetBlock", newestQC.BlockID)
	es.notifier.AssertNotCalled(es.T(), "OnOwnProposal", mock.Anything, mock.Anything)
}

// TestOnReceiveProposal_StaleProposal test that proposals lower than finalized view are not processed at all
// we are not interested in this data because we already performed finalization of that height.
func (es *EventHandlerSuite) TestOnReceiveProposal_StaleProposal() {
	proposal := createProposal(es.forks.FinalizedView()-1, es.forks.FinalizedView()-2)
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	es.forks.AssertNotCalled(es.T(), "AddBlock", proposal)
}

// TestOnReceiveProposal_QCOlderThanCurView tests scenario: received a valid proposal with QC that has older view,
// the proposal's QC shouldn't trigger view change.
func (es *EventHandlerSuite) TestOnReceiveProposal_QCOlderThanCurView() {
	proposal := createProposal(es.initView-1, es.initView-2)

	// should not trigger view change
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.forks.AssertCalled(es.T(), "AddValidatedBlock", proposal.Block)
}

// TestOnReceiveProposal_TCOlderThanCurView tests scenario: received a valid proposal with QC and TC that has older view,
// the proposal's QC shouldn't trigger view change.
func (es *EventHandlerSuite) TestOnReceiveProposal_TCOlderThanCurView() {
	proposal := createProposal(es.initView-1, es.initView-3)
	proposal.LastViewTC = helper.MakeTC(helper.WithTCView(proposal.Block.View-1), helper.WithTCNewestQC(proposal.Block.QC))

	// should not trigger view change
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.forks.AssertCalled(es.T(), "AddValidatedBlock", proposal.Block)
}

// TestOnReceiveProposal_NoVote tests scenario: received a valid proposal for cur view, but not a safe node to vote, and I'm the next leader
// should not vote.
func (es *EventHandlerSuite) TestOnReceiveProposal_NoVote() {
	proposal := createProposal(es.initView, es.initView-1)

	// I'm the next leader
	es.committee.leaders[es.initView+1] = struct{}{}
	// no vote for this proposal
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.forks.AssertCalled(es.T(), "AddValidatedBlock", proposal.Block)
}

// TestOnReceiveProposal_NoVote_ParentProposalNotFound tests scenario: received a valid proposal for cur view, no parent for this proposal found
// should not vote.
func (es *EventHandlerSuite) TestOnReceiveProposal_NoVote_ParentProposalNotFound() {
	proposal := createProposal(es.initView, es.initView-1)

	// remove parent from known proposals
	delete(es.forks.proposals, proposal.Block.QC.BlockID)

	// no vote for this proposal, no parent found
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.Error(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.forks.AssertCalled(es.T(), "AddValidatedBlock", proposal.Block)
}

// TestOnReceiveProposal_Vote_NextLeader tests scenario: received a valid proposal for cur view, safe to vote, I'm the next leader
// should vote and add vote to VoteAggregator.
func (es *EventHandlerSuite) TestOnReceiveProposal_Vote_NextLeader() {
	proposal := createProposal(es.initView, es.initView-1)

	// I'm the next leader
	es.committee.leaders[es.initView+1] = struct{}{}

	// proposal is safe to vote
	es.safetyRules.votable[proposal.Block.BlockID] = struct{}{}

	es.notifier.On("OnOwnVote", proposal.Block.BlockID, proposal.Block.View, mock.Anything, mock.Anything).Once()

	// vote should be created for this proposal
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
}

// TestOnReceiveProposal_Vote_NotNextLeader tests scenario: received a valid proposal for cur view, safe to vote, I'm not the next leader
// should vote and send vote to next leader.
func (es *EventHandlerSuite) TestOnReceiveProposal_Vote_NotNextLeader() {
	proposal := createProposal(es.initView, es.initView-1)

	// proposal is safe to vote
	es.safetyRules.votable[proposal.Block.BlockID] = struct{}{}

	es.notifier.On("OnOwnVote", proposal.Block.BlockID, mock.Anything, mock.Anything, mock.Anything).Once()

	// vote should be created for this proposal
	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
}

// TestOnReceiveProposal_ProposeAfterReceivingTC tests a scenario where we have received TC which advances to view where we are
// leader but no proposal can be created because we don't have parent proposal. After receiving missing parent proposal we have
// all available data to construct a valid proposal. We need to ensure this.
func (es *EventHandlerSuite) TestOnReceiveProposal_ProposeAfterReceivingQC() {

	qc := es.qc

	// first process QC this should advance view
	err := es.eventhandler.OnReceiveQc(qc)
	require.NoError(es.T(), err)
	require.Equal(es.T(), qc.View+1, es.paceMaker.CurView(), "expect a view change")
	es.notifier.AssertNotCalled(es.T(), "OnOwnProposal", mock.Anything, mock.Anything)

	// we are leader for current view
	es.committee.leaders[es.paceMaker.CurView()] = struct{}{}

	es.notifier.On("OnOwnProposal", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		header, ok := args[0].(*flow.Header)
		require.True(es.T(), ok)
		// it should broadcast a header as the same as current view
		require.Equal(es.T(), es.paceMaker.CurView(), header.View)
	}).Once()

	// processing this proposal shouldn't trigger view change since we have already seen QC.
	// we have used QC to advance rounds, but no proposal was made because we were missing parent block
	// when we have received parent block we can try proposing again.
	err = es.eventhandler.OnReceiveProposal(es.votingProposal)
	require.NoError(es.T(), err)

	require.Equal(es.T(), qc.View+1, es.paceMaker.CurView(), "expect a view change")
}

// TestOnReceiveProposal_ProposeAfterReceivingTC tests a scenario where we have received TC which advances to view where we are
// leader but no proposal can be created because we don't have parent proposal. After receiving missing parent proposal we have
// all available data to construct a valid proposal. We need to ensure this.
func (es *EventHandlerSuite) TestOnReceiveProposal_ProposeAfterReceivingTC() {

	// TC contains a QC.BlockID == es.votingProposal
	tc := helper.MakeTC(helper.WithTCView(es.votingProposal.Block.View+1),
		helper.WithTCNewestQC(es.qc))

	// first process TC this should advance view
	err := es.eventhandler.OnReceiveTc(tc)
	require.NoError(es.T(), err)
	require.Equal(es.T(), tc.View+1, es.paceMaker.CurView(), "expect a view change")
	es.notifier.AssertNotCalled(es.T(), "OnOwnProposal", mock.Anything, mock.Anything)

	// we are leader for current view
	es.committee.leaders[es.paceMaker.CurView()] = struct{}{}

	es.notifier.On("OnOwnProposal", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		header, ok := args[0].(*flow.Header)
		require.True(es.T(), ok)
		// it should broadcast a header as the same as current view
		require.Equal(es.T(), es.paceMaker.CurView(), header.View)
	}).Once()

	// processing this proposal shouldn't trigger view change, since we have already seen QC.
	// we have used QC to advance rounds, but no proposal was made because we were missing parent block
	// when we have received parent block we can try proposing again.
	err = es.eventhandler.OnReceiveProposal(es.votingProposal)
	require.NoError(es.T(), err)

	require.Equal(es.T(), tc.View+1, es.paceMaker.CurView(), "expect a view change")
}

// TestOnReceiveQc_HappyPath tests that building a QC for current view triggers view change. We are not leader for next
// round, so no proposal is expected.
func (es *EventHandlerSuite) TestOnReceiveQc_HappyPath() {
	// voting block exists
	es.forks.proposals[es.votingProposal.Block.BlockID] = es.votingProposal.Block

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

	err := es.eventhandler.OnReceiveQc(qc)
	require.NoError(es.T(), err, "if a vote can trigger a QC to be built,"+
		"and the QC triggered a view change, then start new view")
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.notifier.AssertNotCalled(es.T(), "OnOwnProposal", mock.Anything, mock.Anything)
}

// TestOnReceiveQc_FutureView tests that building a QC for future view triggers view change
func (es *EventHandlerSuite) TestOnReceiveQc_FutureView() {
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
	es.forks.proposals[b1.Block.BlockID] = b1.Block
	es.forks.proposals[b2.Block.BlockID] = b2.Block
	es.forks.proposals[b3.Block.BlockID] = b3.Block

	// test that qc for future view should trigger view change
	err := es.eventhandler.OnReceiveQc(qc3)
	endView := b3.Block.View + 1 // next view
	require.NoError(es.T(), err, "if a vote can trigger a QC to be built,"+
		"and the QC triggered a view change, then start new view")
	require.Equal(es.T(), endView, es.paceMaker.CurView(), "incorrect view change")

	// the same qc would not trigger view change
	err = es.eventhandler.OnReceiveQc(qc3)
	endView = b3.Block.View + 1 // next view
	require.NoError(es.T(), err, "same qc should not trigger view change")
	require.Equal(es.T(), endView, es.paceMaker.CurView(), "incorrect view change")

	// old QCs won't trigger view change
	err = es.eventhandler.OnReceiveQc(qc2)
	require.NoError(es.T(), err)
	require.Equal(es.T(), endView, es.paceMaker.CurView(), "incorrect view change")

	err = es.eventhandler.OnReceiveQc(qc1)
	require.NoError(es.T(), err)
	require.Equal(es.T(), endView, es.paceMaker.CurView(), "incorrect view change")
}

// TestOnReceiveQc_NextLeaderProposes tests that after receiving a valid proposal for cur view, and I'm the next leader,
// a QC can be built for the block, triggered view change, and I will propose
func (es *EventHandlerSuite) TestOnReceiveQc_NextLeaderProposes() {
	proposal := createProposal(es.initView, es.initView-1)
	qc := createQC(proposal.Block)
	// I'm the next leader
	es.committee.leaders[es.initView+1] = struct{}{}
	// qc triggered view change
	es.endView++
	// I'm the leader of cur view (7)
	// I'm not the leader of next view (8), trigger view change

	err := es.eventhandler.OnReceiveProposal(proposal)
	require.NoError(es.T(), err)

	es.notifier.On("OnOwnProposal", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		header, ok := args[0].(*flow.Header)
		require.True(es.T(), ok)
		// it should broadcast a header as the same as endView
		require.Equal(es.T(), es.endView, header.View)
	}).Once()

	// after receiving proposal build QC and deliver it to event handler
	err = es.eventhandler.OnReceiveQc(qc)
	require.NoError(es.T(), err)

	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.forks.AssertCalled(es.T(), "AddValidatedBlock", proposal.Block)
}

// TestOnReceiveQc_ProposeOnce tests that after constructing proposal we don't attempt to create another
// proposal for same view.
func (es *EventHandlerSuite) TestOnReceiveQc_ProposeOnce() {
	// I'm the next leader
	es.committee.leaders[es.initView+1] = struct{}{}

	es.endView++

	es.notifier.On("OnOwnProposal", mock.Anything, mock.Anything).Once()

	err := es.eventhandler.OnReceiveProposal(es.votingProposal)
	require.NoError(es.T(), err)

	// constructing QC triggers making block proposal
	err = es.eventhandler.OnReceiveQc(es.qc)
	require.NoError(es.T(), err)

	// receiving same proposal again triggers proposing logic
	err = es.eventhandler.OnReceiveProposal(es.votingProposal)
	require.NoError(es.T(), err)

	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	es.notifier.AssertNumberOfCalls(es.T(), "OnOwnProposal", 1)
}

// TestOnTCConstructed_HappyPath tests that building a TC for current view triggers view change
func (es *EventHandlerSuite) TestOnReceiveTc_HappyPath() {
	// voting block exists
	es.forks.proposals[es.votingProposal.Block.BlockID] = es.votingProposal.Block

	// a tc is built
	tc := helper.MakeTC(helper.WithTCView(es.initView), helper.WithTCNewestQC(es.votingProposal.Block.QC))

	// expect a view change
	es.endView++

	err := es.eventhandler.OnReceiveTc(tc)
	require.NoError(es.T(), err, "TC should trigger a view change and start of new view")
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
}

// TestOnTCConstructed_NextLeaderProposes tests that after receiving TC and advancing view we as next leader create a proposal
// and broadcast it
func (es *EventHandlerSuite) TestOnReceiveTc_NextLeaderProposes() {
	es.committee.leaders[es.tc.View+1] = struct{}{}
	es.endView++

	es.notifier.On("OnOwnProposal", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		header, ok := args[0].(*flow.Header)
		require.True(es.T(), ok)
		// it should broadcast a header as the same as endView
		require.Equal(es.T(), es.endView, header.View)

		// proposed block should contain valid newest QC and lastViewTC
		expectedNewestQC := es.paceMaker.NewestQC()
		proposal := model.ProposalFromFlow(header)
		require.Equal(es.T(), expectedNewestQC, proposal.Block.QC)
		require.Equal(es.T(), es.paceMaker.LastViewTC(), proposal.LastViewTC)
	}).Once()

	err := es.eventhandler.OnReceiveTc(es.tc)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "TC didn't trigger view change")
}

// TestOnTimeout tests that event handler produces TimeoutObject and broadcasts it to other members of consensus
// committee. Additionally, It has to contribute TimeoutObject to timeout aggregation process by sending it to TimeoutAggregator.
func (es *EventHandlerSuite) TestOnTimeout() {
	es.notifier.On("OnOwnTimeout", mock.Anything).Run(func(args mock.Arguments) {
		timeoutObject, ok := args[0].(*model.TimeoutObject)
		require.True(es.T(), ok)
		// it should broadcast a TO with same view as endView
		require.Equal(es.T(), es.endView, timeoutObject.View)
	}).Once()

	err := es.eventhandler.OnLocalTimeout()
	require.NoError(es.T(), err)

	// TimeoutObject shouldn't trigger view change
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
}

// TestOnTimeout_SanityChecks tests a specific scenario where pacemaker have seen both QC and TC for previous view
// and EventHandler tries to produce a timeout object, such timeout object is invalid if both QC and TC is present, we
// need to make sure that EventHandler filters out TC for last view if we know about QC for same view.
func (es *EventHandlerSuite) TestOnTimeout_SanityChecks() {
	// voting block exists
	es.forks.proposals[es.votingProposal.Block.BlockID] = es.votingProposal.Block

	// a tc is built
	tc := helper.MakeTC(helper.WithTCView(es.initView), helper.WithTCNewestQC(es.votingProposal.Block.QC))

	// expect a view change
	es.endView++

	err := es.eventhandler.OnReceiveTc(tc)
	require.NoError(es.T(), err, "TC should trigger a view change and start of new view")
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")

	// receive a QC for the same view as the TC
	qc := helper.MakeQC(helper.WithQCView(tc.View))
	err = es.eventhandler.OnReceiveQc(qc)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "QC shouldn't trigger view change")
	require.Equal(es.T(), tc, es.paceMaker.LastViewTC(), "invalid last view TC")
	require.Equal(es.T(), qc, es.paceMaker.NewestQC(), "invalid newest QC")

	es.notifier.On("OnOwnTimeout", mock.Anything).Run(func(args mock.Arguments) {
		timeoutObject, ok := args[0].(*model.TimeoutObject)
		require.True(es.T(), ok)
		require.Equal(es.T(), es.endView, timeoutObject.View)
		require.Equal(es.T(), qc, timeoutObject.NewestQC)
		require.Nil(es.T(), timeoutObject.LastViewTC)
	}).Once()

	err = es.eventhandler.OnLocalTimeout()
	require.NoError(es.T(), err)
}

// TestOnTimeout_ReplicaEjected tests that EventHandler correctly handles possible errors from SafetyRules and doesn't broadcast
// timeout objects when replica is ejected.
func (es *EventHandlerSuite) TestOnTimeout_ReplicaEjected() {
	es.Run("no-timeout", func() {
		*es.safetyRules.SafetyRules = *mocks.NewSafetyRules(es.T())
		es.safetyRules.On("ProduceTimeout", mock.Anything, mock.Anything, mock.Anything).Return(nil, model.NewNoTimeoutErrorf(""))
		err := es.eventhandler.OnLocalTimeout()
		require.NoError(es.T(), err, "should be handled as sentinel error")
	})
	es.Run("create-timeout-exception", func() {
		*es.safetyRules.SafetyRules = *mocks.NewSafetyRules(es.T())
		exception := errors.New("produce-timeout-exception")
		es.safetyRules.On("ProduceTimeout", mock.Anything, mock.Anything, mock.Anything).Return(nil, exception)
		err := es.eventhandler.OnLocalTimeout()
		require.ErrorIs(es.T(), err, exception, "expect a wrapped exception")
	})
	es.notifier.AssertNotCalled(es.T(), "OnOwnTimeout", mock.Anything)
}

// Test100Timeout tests that receiving 100 TCs for increasing views advances rounds
func (es *EventHandlerSuite) Test100Timeout() {
	for i := 0; i < 100; i++ {
		tc := helper.MakeTC(helper.WithTCView(es.initView + uint64(i)))
		err := es.eventhandler.OnReceiveTc(tc)
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
			parentBlock := helper.MakeBlock(func(block *model.Block) {
				block.BlockID = proposal.Block.QC.BlockID
				block.View = proposal.Block.QC.View
			})
			es.forks.proposals[parentBlock.BlockID] = parentBlock
		}

		es.safetyRules.votable[proposal.Block.BlockID] = struct{}{}
		// should trigger 100 view change
		es.endView++

		es.notifier.On("OnOwnProposal", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			header, ok := args[0].(*flow.Header)
			require.True(es.T(), ok)
			require.Equal(es.T(), proposal.Block.View+1, header.View)
		}).Once()
		es.notifier.On("OnOwnVote", proposal.Block.BlockID, proposal.Block.View, mock.Anything, mock.Anything).Once()

		err := es.eventhandler.OnReceiveProposal(proposal)
		require.NoError(es.T(), err)
		err = es.eventhandler.OnReceiveQc(qc)
		require.NoError(es.T(), err)
	}

	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	require.Equal(es.T(), totalView, (len(es.forks.proposals)-1)/2)
}

// TestFollowerFollows100Blocks tests scenario where follower receives 100 proposals one after another
func (es *EventHandlerSuite) TestFollowerFollows100Blocks() {
	// add parent proposal otherwise we can't propose
	parentProposal := createProposal(es.initView, es.initView-1)
	es.forks.proposals[parentProposal.Block.BlockID] = parentProposal.Block
	for i := 0; i < 100; i++ {
		// create each proposal as if they are created by some leader
		proposal := createProposal(es.initView+uint64(i)+1, es.initView+uint64(i))
		// as a follower, I receive these proposals
		err := es.eventhandler.OnReceiveProposal(proposal)
		require.NoError(es.T(), err)
		es.endView++
	}
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	require.Equal(es.T(), 100, len(es.forks.proposals)-2)
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
		// as a follower, I receive these proposals
		err := es.eventhandler.OnReceiveProposal(proposal)
		require.NoError(es.T(), err)
	}
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	require.Equal(es.T(), 100, len(es.forks.proposals)-1)
}

// TestStart_ProposeOnce tests that after starting event handler we don't create proposal in case we have already proposed
// for this view.
func (es *EventHandlerSuite) TestStart_ProposeOnce() {
	// I'm the next leader
	es.committee.leaders[es.initView+1] = struct{}{}
	es.endView++

	// STEP 1: simulating events _before_ a crash: EventHandler receives proposal and then a QC for the proposal (from VoteAggregator)
	es.notifier.On("OnOwnProposal", mock.Anything, mock.Anything).Once()
	err := es.eventhandler.OnReceiveProposal(es.votingProposal)
	require.NoError(es.T(), err)

	// constructing QC triggers making block proposal
	err = es.eventhandler.OnReceiveQc(es.qc)
	require.NoError(es.T(), err)
	es.notifier.AssertNumberOfCalls(es.T(), "OnOwnProposal", 1)

	// Here, a hypothetical crash would happen.
	// During crash recovery, Forks and PaceMaker are recovered to have exactly the same in-memory state as before
	// Start triggers proposing logic. But as our own proposal for the view is already in Forks, we should not propose again.
	err = es.eventhandler.Start(es.ctx)
	require.NoError(es.T(), err)
	require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")

	// assert that broadcast wasn't trigger again, i.e. there should have been only one event `OnOwnProposal` in total
	es.notifier.AssertNumberOfCalls(es.T(), "OnOwnProposal", 1)
}

// TestCreateProposal_SanityChecks tests that proposing logic performs sanity checks when creating new block proposal.
// Specifically it tests a case where TC contains QC which: TC.View == TC.NewestQC.View
func (es *EventHandlerSuite) TestCreateProposal_SanityChecks() {
	// round ended with TC where TC.View == TC.NewestQC.View
	tc := helper.MakeTC(helper.WithTCView(es.initView),
		helper.WithTCNewestQC(helper.MakeQC(helper.WithQCBlock(es.votingProposal.Block))))

	es.forks.proposals[es.votingProposal.Block.BlockID] = es.votingProposal.Block

	// I'm the next leader
	es.committee.leaders[tc.View+1] = struct{}{}

	es.notifier.On("OnOwnProposal", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		header, ok := args[0].(*flow.Header)
		require.True(es.T(), ok)
		// we need to make sure that produced proposal contains only QC even if there is TC for previous view as well
		require.Nil(es.T(), header.LastViewTC)
	}).Once()

	err := es.eventhandler.OnReceiveTc(tc)
	require.NoError(es.T(), err)

	require.Equal(es.T(), tc.NewestQC, es.paceMaker.NewestQC())
	require.Equal(es.T(), tc, es.paceMaker.LastViewTC())
	require.Equal(es.T(), tc.View+1, es.paceMaker.CurView(), "incorrect view change")
}

// TestOnReceiveProposal_ProposalForActiveView tests that when receiving proposal for active we don't attempt to create a proposal
// Receiving proposal can trigger proposing logic only in case we have received missing block for past views.
func (es *EventHandlerSuite) TestOnReceiveProposal_ProposalForActiveView() {
	// receive proposal where we are leader, meaning that we have produced this proposal
	es.committee.leaders[es.votingProposal.Block.View] = struct{}{}

	err := es.eventhandler.OnReceiveProposal(es.votingProposal)
	require.NoError(es.T(), err)

	es.notifier.AssertNotCalled(es.T(), "OnOwnProposal", mock.Anything, mock.Anything)
}

// TestOnPartialTcCreated_ProducedTimeout tests that when receiving partial TC for active view we will create a timeout object
// immediately.
func (es *EventHandlerSuite) TestOnPartialTcCreated_ProducedTimeout() {
	partialTc := &hotstuff.PartialTcCreated{
		View:       es.initView,
		NewestQC:   es.parentProposal.Block.QC,
		LastViewTC: nil,
	}

	es.notifier.On("OnOwnTimeout", mock.Anything).Run(func(args mock.Arguments) {
		timeoutObject, ok := args[0].(*model.TimeoutObject)
		require.True(es.T(), ok)
		// it should broadcast a TO with same view as partialTc.View
		require.Equal(es.T(), partialTc.View, timeoutObject.View)
	}).Once()

	err := es.eventhandler.OnPartialTcCreated(partialTc)
	require.NoError(es.T(), err)

	// partial TC shouldn't trigger view change
	require.Equal(es.T(), partialTc.View, es.paceMaker.CurView(), "incorrect view change")
}

// TestOnPartialTcCreated_NotActiveView tests that we don't create timeout object if partial TC was delivered for a past, non-current view.
// NOTE: it is not possible to receive a partial timeout for a FUTURE view, unless the partial timeout contains
// either a QC/TC allowing us to enter that view, therefore that case is not covered here.
// See TestOnPartialTcCreated_QcAndTcProcessing instead.
func (es *EventHandlerSuite) TestOnPartialTcCreated_NotActiveView() {
	partialTc := &hotstuff.PartialTcCreated{
		View:     es.initView - 1,
		NewestQC: es.parentProposal.Block.QC,
	}

	err := es.eventhandler.OnPartialTcCreated(partialTc)
	require.NoError(es.T(), err)

	// partial TC shouldn't trigger view change
	require.Equal(es.T(), es.initView, es.paceMaker.CurView(), "incorrect view change")
	// we don't want to create timeout if partial TC was delivered for view different than active one.
	es.notifier.AssertNotCalled(es.T(), "OnOwnTimeout", mock.Anything)
}

// TestOnPartialTcCreated_QcAndTcProcessing tests that EventHandler processes QC and TC included in hotstuff.PartialTcCreated
// data structure. This tests cases like the following example:
// * the pacemaker is in view 10
// * we observe a partial timeout for view 11 with a QC for view 10
// * we should change to view 11 using the QC, then broadcast a timeout for view 11
func (es *EventHandlerSuite) TestOnPartialTcCreated_QcAndTcProcessing() {

	testOnPartialTcCreated := func(partialTc *hotstuff.PartialTcCreated) {
		es.endView++

		es.notifier.On("OnOwnTimeout", mock.Anything).Run(func(args mock.Arguments) {
			timeoutObject, ok := args[0].(*model.TimeoutObject)
			require.True(es.T(), ok)
			// it should broadcast a TO with same view as partialTc.View
			require.Equal(es.T(), partialTc.View, timeoutObject.View)
		}).Once()

		err := es.eventhandler.OnPartialTcCreated(partialTc)
		require.NoError(es.T(), err)

		require.Equal(es.T(), es.endView, es.paceMaker.CurView(), "incorrect view change")
	}

	es.Run("qc-triggered-view-change", func() {
		partialTc := &hotstuff.PartialTcCreated{
			View:     es.qc.View + 1,
			NewestQC: es.qc,
		}
		testOnPartialTcCreated(partialTc)
	})
	es.Run("tc-triggered-view-change", func() {
		tc := helper.MakeTC(helper.WithTCView(es.endView), helper.WithTCNewestQC(es.qc))
		partialTc := &hotstuff.PartialTcCreated{
			View:       tc.View + 1,
			NewestQC:   tc.NewestQC,
			LastViewTC: tc,
		}
		testOnPartialTcCreated(partialTc)
	})
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
