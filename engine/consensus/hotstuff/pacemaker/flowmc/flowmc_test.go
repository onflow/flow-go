package flowmc

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	mockdist "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications/mock"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/pacemaker/flowmc/timeout"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

func initPaceMaker(t *testing.T, view uint64) (hotstuff.PaceMaker, *mockdist.Distributor) {
	eventProc := &mockdist.Distributor{}
	tc, err := timeout.NewConfig(100, 100, 0.5, 1.5, 50)
	if err != nil {
		t.Fail()
	}
	pm, err := New(view, timeout.NewController(*tc), eventProc)
	if err != nil {
		t.Fail()
	}

	eventProc.On("OnStartingBlockTimeout", uint64(3)).Return().Once()
	pm.Start()
	return pm, eventProc
}

func qc(view uint64) *types.QuorumCertificate {
	return &types.QuorumCertificate{View: view}
}

func makeBlockProposal(qcView, blockView uint64) *types.BlockProposal {
	return &types.BlockProposal{
		Block: &types.Block{View: blockView, QC: qc(qcView)},
	}
}

// Test_SkipIncreaseViewThroughQC tests that PaceMaker increases View when receiving QC,
// if applicable, by skipping views
func Test_SkipIncreaseViewThroughQC(t *testing.T) {
	pm, eventProc := initPaceMaker(t,3)

	eventProc.On("OnStartingBlockTimeout", uint64(4)).Return().Once()
	pm.UpdateCurViewWithQC(qc(3))

	eventProc.On("OnSkippedAhead", uint64(13)).Return().Once()
	eventProc.On("OnStartingBlockTimeout", uint64(13)).Return().Once()
	pm.UpdateCurViewWithQC(qc(12))

	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(13), pm.CurView())
	//eventProc.AssertNumberOfCalls(t, "OnEnteringView", 1)
}

// Test_IgnoreOldBlocks tests that PaceMaker ignores old blocks
func Test_IgnoreOldQC(t *testing.T) {
	pm, eventProc := initPaceMaker(t,3)
	pm.UpdateCurViewWithQC(qc(2))
	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(3), pm.CurView()) 
}

// Test_SkipViewThroughBlock tests that PaceMaker skips View when receiving Block containing QC with larger View Number
func Test_SkipViewThroughBlock(t *testing.T) {
	pm, eventProc := initPaceMaker(t, 3)

	eventProc.On("OnSkippedAhead", uint64(6)).Return().Once()
	eventProc.On("OnStartingBlockTimeout", uint64(6)).Return().Once()
	pm.UpdateCurViewWithBlock(makeBlockProposal(5, 9), true)
	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(6), pm.CurView())

	eventProc.On("OnSkippedAhead", uint64(23)).Return().Once()
	eventProc.On("OnStartingBlockTimeout", uint64(23)).Return().Once()
	pm.UpdateCurViewWithBlock(makeBlockProposal(22, 25), false)

	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(23), pm.CurView())
}

// Test_HandlesSkipViewAttack verifies that PaceMaker skips views based on QC.view
// but NOT based on block.View to avoid vulnerability against Fast-Forward Attack
func Test_HandlesSkipViewAttack(t *testing.T) {
	pm, eventProc := initPaceMaker(t,3)

	eventProc.On("OnSkippedAhead", uint64(6)).Return().Once()
	eventProc.On("OnStartingBlockTimeout", uint64(6)).Return().Once()
	pm.UpdateCurViewWithBlock(makeBlockProposal(5, 9), true)

	eventProc.On("OnSkippedAhead", uint64(15)).Return().Once()
	eventProc.On("OnStartingBlockTimeout", uint64(15)).Return().Once()
	pm.UpdateCurViewWithBlock(makeBlockProposal(14, 23), false)

	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(15), pm.CurView()) 
}

// Test_IgnoreOldBlocks tests that PaceMaker ignores old blocks
func Test_IgnoreOldBlocks(t *testing.T) {
	pm, eventProc := initPaceMaker(t,3)
	pm.UpdateCurViewWithBlock(makeBlockProposal(1, 2), false)
	pm.UpdateCurViewWithBlock(makeBlockProposal(1, 2), true)
	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(3), pm.CurView()) 
}

// Test_ProcessBlockForCurrentView tests that PaceMaker processes the block for the current view correctly
func Test_ProcessBlockForCurrentView(t *testing.T) {
	pm, eventProc := initPaceMaker(t,3)
	eventProc.On("OnStartingVotesTimeout", uint64(3)).Return().Once()
	pm.UpdateCurViewWithBlock(makeBlockProposal(1, 3), true)
	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(3), pm.CurView())

	pm, eventProc = initPaceMaker(t,3)
	eventProc.On("OnStartingBlockTimeout", uint64(4)).Return().Once()
	pm.UpdateCurViewWithBlock(makeBlockProposal(1, 3), false)
	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(4), pm.CurView())
}

// Test_ProcessBlockForFutureView tests that PaceMaker processes the block for future views correctly:
// *
func Test_ProcessBlockForFutureView(t *testing.T) {
	// NOT Primary for the Block's view
	pm, eventProc := initPaceMaker(t,3)
	eventProc.On("OnStartingBlockTimeout", uint64(4)).Return().Once()
	eventProc.On("OnStartingBlockTimeout", uint64(5)).Return().Once()
	pm.UpdateCurViewWithBlock(makeBlockProposal(3, 4), false)
	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(5), pm.CurView())

	pm, eventProc = initPaceMaker(t,3)
	eventProc.On("OnSkippedAhead", uint64(14)).Return().Once()
	eventProc.On("OnStartingBlockTimeout", uint64(14)).Return().Once()
	eventProc.On("OnStartingBlockTimeout", uint64(15)).Return().Once()
	pm.UpdateCurViewWithBlock(makeBlockProposal(13, 14), false)
	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(15), pm.CurView())

	pm, eventProc = initPaceMaker(t,3)
	eventProc.On("OnSkippedAhead", uint64(14)).Return().Once()
	eventProc.On("OnStartingBlockTimeout", uint64(14)).Return().Once()
	pm.UpdateCurViewWithBlock(makeBlockProposal(13, 17), false)
	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(14), pm.CurView())

	// PRIMARY for the Block's view
	pm, eventProc = initPaceMaker(t,3)
	eventProc.On("OnStartingBlockTimeout", uint64(4)).Return().Once()
	eventProc.On("OnStartingVotesTimeout", uint64(4)).Return().Once()
	pm.UpdateCurViewWithBlock(makeBlockProposal(3, 4), true)
	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(4), pm.CurView())

	pm, eventProc = initPaceMaker(t,3)
	eventProc.On("OnSkippedAhead", uint64(14)).Return().Once()
	eventProc.On("OnStartingBlockTimeout", uint64(14)).Return().Once()
	eventProc.On("OnStartingVotesTimeout", uint64(14)).Return().Once()
	pm.UpdateCurViewWithBlock(makeBlockProposal(13, 14), true)
	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(14), pm.CurView())

	pm, eventProc = initPaceMaker(t,3)
	eventProc.On("OnSkippedAhead", uint64(14)).Return().Once()
	eventProc.On("OnStartingBlockTimeout", uint64(14)).Return().Once()
	pm.UpdateCurViewWithBlock(makeBlockProposal(13, 17), true)
	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(14), pm.CurView())
}

// Test_IgnoreBlockDuplicates tests that PaceMaker ignores duplicate blocks
func Test_IgnoreBlockDuplicates(t *testing.T) {
	// NOT Primary for the Block's view
	pm, eventProc := initPaceMaker(t,3)
	eventProc.On("OnStartingBlockTimeout", uint64(4)).Return().Once()
	eventProc.On("OnStartingBlockTimeout", uint64(5)).Return().Once()
	pm.UpdateCurViewWithBlock(makeBlockProposal(3, 4), false)
	pm.UpdateCurViewWithBlock(makeBlockProposal(3, 4), false)
	pm.UpdateCurViewWithBlock(makeBlockProposal(3, 4), false)
	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(5), pm.CurView())

	// PRIMARY for the Block's view
	pm, eventProc = initPaceMaker(t,3)
	eventProc.On("OnStartingBlockTimeout", uint64(4)).Return().Once()
	eventProc.On("OnStartingVotesTimeout", uint64(4)).Return().Once()
	pm.UpdateCurViewWithBlock(makeBlockProposal(3, 4), true)
	pm.UpdateCurViewWithBlock(makeBlockProposal(3, 4), true)
	pm.UpdateCurViewWithBlock(makeBlockProposal(3, 4), true)
	eventProc.AssertExpectations(t)
	assert.Equal(t, uint64(4), pm.CurView())
}
