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
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/model/flow"
)

type rapidStuff struct {
	handler   *eventhandler.EventHandlerV2
	pacemaker hotstuff.PaceMaker
	validator *BlacklistValidator
	committee *Committee

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
	voteAggregator.On("InvalidBlock", mock.Anything).Return(nil)
	voteAggregator.On("AddBlock", mock.Anything).Return(nil)
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
	r.validator = validator
	r.committee = committee
	r.expectedCurView = curView

	for i := 0; i < 300; i++ {
		if isLeaderForView(uint64(i)) {
			r.markSelfAsLeaderForView(uint64(i))
		}
	}
}

func isLeaderForView(view uint64) bool {
	return view%3 == 0
}

func (r *rapidStuff) markInvalidProposal(blockID flow.Identifier) {
	r.validator.invalidProposals[blockID] = struct{}{}
}

func (r *rapidStuff) unmarkInvalidProposal(blockID flow.Identifier) {
	delete(r.validator.invalidProposals, blockID)
}

func randomBlockAroundCurView(t *rapid.T, curView uint64) *model.Proposal {
	// randomly create a block that could either from the past or the future
	parentView := rapid.IntRange(-3, 3).Map(func(dis int) uint64 {
		return uint64(int(curView) + dis)
	}).Draw(t, "parentView").(uint64)

	view := rapid.IntRange(1, 3).Map(func(dis int) uint64 {
		return parentView + uint64(dis)
	}).Draw(t, "view").(uint64)

	return createProposal(view, parentView)
}

// randomly receive a timeout, which will increment the current view
func (r *rapidStuff) ReceiveTimeout(t *rapid.T) {
	curView := r.curView()
	r.handler.OnLocalTimeout()
	r.expectedCurView = curView + 1
}

func (r *rapidStuff) curView() uint64 {
	return r.pacemaker.CurView()
}

// receive a block for the current view
func (r *rapidStuff) ReceiveBlockForCurView(t *rapid.T) {
	curView := r.curView()
	block := createProposal(curView, curView-1)
	r.handler.OnReceiveProposal(block)
	isNextLeader := isLeaderForView(curView + 1)
	if !isNextLeader {
		r.expectedCurView++
	}
}

func (r *rapidStuff) markSelfAsLeaderForView(view uint64) {
	r.committee.leaders[view] = struct{}{}
}

// randomly receive a invalid block
func (r *rapidStuff) ReceiveInvalidBlock(t *rapid.T) {
	curView := r.curView()

	// we don't validate own proposal, so skip views where I'm the leader
	if isLeaderForView(curView) {
		return
	}

	block := randomBlockAroundCurView(t, curView)
	r.markInvalidProposal(block.Block.BlockID)

	r.handler.OnReceiveProposal(block)

	// the marker is no longer needed after processing the block,
	// remove the marker so that the same block might be used as a
	// valid block, and not to be treated as invalid block any more
	r.unmarkInvalidProposal(block.Block.BlockID)
}

// ramdonly receive a QC for a view where I'm the leader and is the current view or future,
// which should increment the current view to qcView + 1
func (r *rapidStuff) ReceiveQC(t *rapid.T) {
	curView := r.curView()

	// we only create QC for views where we are the leader
	// so skip views where I'm no the leader
	if !isLeaderForView(curView) {
		return
	}

	qcView := rapid.IntRange(0, 4).Map(func(dis int) uint64 {
		return uint64(int(curView) + dis)
	}).Draw(t, "qcView").(uint64)

	block := createProposal(qcView+1, qcView)
	r.handler.OnQCConstructed(block.Block.QC)

	// after QC has been received, we should enter next view
	r.expectedCurView = qcView + 1
}

// ramdonly receive a QC for a view where I'm the leader, if QC's block is for old view,
// then it should not increment the view
func (r *rapidStuff) ReceiveOldQC(t *rapid.T) {
	curView := r.curView()

	qcView := rapid.IntRange(-4, -1).Map(func(dis int) uint64 {
		return uint64(int(curView) + dis)
	}).Draw(t, "qcView").(uint64)

	// we only create QC for views where we are the leader
	// so skip views where I'm no the leader
	if !isLeaderForView(qcView) {
		return
	}

	block := createProposal(qcView+1, qcView)
	r.handler.OnQCConstructed(block.Block.QC)
}

func (r *rapidStuff) Check(t *rapid.T) {
	require.Equal(t, r.expectedCurView, r.pacemaker.CurView(), "the actual current view is different from expected")
}

func TestEventHandlerRapid(t *testing.T) {
	rapid.Check(t, rapid.Run(&rapidStuff{}))
}
