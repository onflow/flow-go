package pacemaker

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	mockdist "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications/mock"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/pacemaker/timeout"
	model "github.com/dapperlabs/flow-go/model/hotstuff"
)

const (
	startRepTimeout        float64 = 400.0 // Milliseconds
	minRepTimeout          float64 = 100.0 // Milliseconds
	voteTimeoutFraction    float64 = 0.5   // multiplicative factor
	multiplicativeIncrease float64 = 1.5   // multiplicative factor
	additiveDecrease       float64 = 50    // Milliseconds
)

func expectedTimerInfo(view uint64, mode model.TimeoutMode) interface{} {
	return mock.MatchedBy(
		func(timerInfo *model.TimerInfo) bool {
			return timerInfo.View == view && timerInfo.Mode == mode
		})
}

func expectedTimeoutInfo(view uint64, mode model.TimeoutMode) interface{} {
	return mock.MatchedBy(
		func(timerInfo *model.TimerInfo) bool {
			return timerInfo.View == view && timerInfo.Mode == mode
		})
}

func initPaceMaker(t *testing.T, view uint64) (hotstuff.PaceMaker, *mockdist.Consumer) {
	notifier := &mockdist.Consumer{}
	tc, err := timeout.NewConfig(
		time.Duration(startRepTimeout*1e6),
		time.Duration(minRepTimeout*1e6),
		voteTimeoutFraction,
		multiplicativeIncrease,
		time.Duration(additiveDecrease*1e6))
	if err != nil {
		t.Fail()
	}
	pm, err := NewFlowPaceMaker(view, timeout.NewController(*tc), notifier)
	if err != nil {
		t.Fail()
	}
	notifier.On("OnStartingTimeout", expectedTimerInfo(view, model.ReplicaTimeout)).Return().Once()
	pm.Start()
	return pm, notifier
}

func qc(view uint64) *model.QuorumCertificate {
	return &model.QuorumCertificate{View: view}
}

func makeBlock(qcView, blockView uint64) *model.Block {
	return &model.Block{View: blockView, QC: qc(qcView)}
}

// Test_SkipIncreaseViewThroughQC tests that PaceMaker increases View when receiving QC,
// if applicable, by skipping views
func Test_SkipIncreaseViewThroughQC(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3)

	notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
	nve, nveOccurred := pm.UpdateCurViewWithQC(qc(3))
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(4), pm.CurView())
	assert.True(t, nveOccurred && nve.View == 4)

	notifier.On("OnSkippedAhead", uint64(13)).Return().Once()
	notifier.On("OnStartingTimeout", expectedTimerInfo(13, model.ReplicaTimeout)).Return().Once()
	nve, nveOccurred = pm.UpdateCurViewWithQC(qc(12))
	assert.True(t, nveOccurred && nve.View == 13)

	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(13), pm.CurView())
}

// Test_IgnoreOldBlocks tests that PaceMaker ignores old blocks
func Test_IgnoreOldQC(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3)
	nve, nveOccurred := pm.UpdateCurViewWithQC(qc(2))
	assert.True(t, !nveOccurred && nve == nil)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(3), pm.CurView())
}

// Test_SkipViewThroughBlock tests that PaceMaker skips View when receiving Block containing QC with larger View Number
func Test_SkipViewThroughBlock(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3)

	notifier.On("OnSkippedAhead", uint64(6)).Return().Once()
	notifier.On("OnStartingTimeout", expectedTimerInfo(6, model.ReplicaTimeout)).Return().Once()
	nve, nveOccurred := pm.UpdateCurViewWithBlock(makeBlock(5, 9), true)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(6), pm.CurView())
	assert.True(t, nveOccurred && nve.View == 6)

	notifier.On("OnSkippedAhead", uint64(23)).Return().Once()
	notifier.On("OnStartingTimeout", expectedTimerInfo(23, model.ReplicaTimeout)).Return().Once()
	nve, nveOccurred = pm.UpdateCurViewWithBlock(makeBlock(22, 25), false)
	assert.True(t, nveOccurred && nve.View == 23)

	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(23), pm.CurView())
}

// Test_HandlesSkipViewAttack verifies that PaceMaker skips views based on QC.view
// but NOT based on block.View to avoid vulnerability against Fast-Forward Attack
func Test_HandlesSkipViewAttack(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3)

	notifier.On("OnSkippedAhead", uint64(6)).Return().Once()
	notifier.On("OnStartingTimeout", expectedTimerInfo(6, model.ReplicaTimeout)).Return().Once()
	nve, nveOccurred := pm.UpdateCurViewWithBlock(makeBlock(5, 9), true)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(6), pm.CurView())
	assert.True(t, nveOccurred && nve.View == 6)

	notifier.On("OnSkippedAhead", uint64(15)).Return().Once()
	notifier.On("OnStartingTimeout", expectedTimerInfo(15, model.ReplicaTimeout)).Return().Once()
	nve, nveOccurred = pm.UpdateCurViewWithBlock(makeBlock(14, 23), false)
	assert.True(t, nveOccurred && nve.View == 15)

	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(15), pm.CurView())
}

// Test_IgnoreOldBlocks tests that PaceMaker ignores old blocks
func Test_IgnoreOldBlocks(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3)
	pm.UpdateCurViewWithBlock(makeBlock(1, 2), false)
	pm.UpdateCurViewWithBlock(makeBlock(1, 2), true)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(3), pm.CurView())
}

// Test_ProcessBlockForCurrentView tests that PaceMaker processes the block for the current view correctly
func Test_ProcessBlockForCurrentView(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3)
	notifier.On("OnStartingTimeout", expectedTimerInfo(3, model.VoteCollectionTimeout)).Return().Once()
	nve, nveOccurred := pm.UpdateCurViewWithBlock(makeBlock(1, 3), true)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(3), pm.CurView())
	assert.True(t, !nveOccurred && nve == nil)

	pm, notifier = initPaceMaker(t, 3)
	notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
	nve, nveOccurred = pm.UpdateCurViewWithBlock(makeBlock(1, 3), false)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(4), pm.CurView())
	assert.True(t, nveOccurred && nve.View == 4)
}

// Test_FutureBlockWithQcForCurrentView tests that PaceMaker processes the block with
//    block.qc.view = curView
//    block.view = curView +1
// correctly. Specifically, it should induce a view change to the block.view, which
// enables processing the block right away, i.e. switch to block.view + 1
func Test_FutureBlockWithQcForCurrentView(t *testing.T) {
	// NOT Primary for the Block's view
	pm, notifier := initPaceMaker(t, 3)
	notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
	notifier.On("OnStartingTimeout", expectedTimerInfo(5, model.ReplicaTimeout)).Return().Once()
	nve, nveOccurred := pm.UpdateCurViewWithBlock(makeBlock(3, 4), false)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(5), pm.CurView())
	assert.True(t, nveOccurred && nve.View == 5)

	// PRIMARY for the Block's view
	pm, notifier = initPaceMaker(t, 3)
	notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
	notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.VoteCollectionTimeout)).Return().Once()
	nve, nveOccurred = pm.UpdateCurViewWithBlock(makeBlock(3, 4), true)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(4), pm.CurView())
	assert.True(t, nveOccurred && nve.View == 4)
}

// Test_FutureBlockWithQcForCurrentView tests that PaceMaker processes the block with
//    block.qc.view > curView
//    block.view = block.qc.view +1
// correctly. Specifically, it should induce a view change to block.view, which
// enables processing the block right away, i.e. switch to block.view + 1
func Test_FutureBlockWithQcForFutureView(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3)
	notifier.On("OnSkippedAhead", uint64(14)).Return().Once()
	notifier.On("OnStartingTimeout", expectedTimerInfo(14, model.ReplicaTimeout)).Return().Once()
	notifier.On("OnStartingTimeout", expectedTimerInfo(15, model.ReplicaTimeout)).Return().Once()
	nve, nveOccurred := pm.UpdateCurViewWithBlock(makeBlock(13, 14), false)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(15), pm.CurView())
	assert.True(t, nveOccurred && nve.View == 15)

	pm, notifier = initPaceMaker(t, 3)
	notifier.On("OnSkippedAhead", uint64(14)).Return().Once()
	notifier.On("OnStartingTimeout", expectedTimerInfo(14, model.ReplicaTimeout)).Return().Once()
	notifier.On("OnStartingTimeout", expectedTimerInfo(14, model.VoteCollectionTimeout)).Return().Once()
	nve, nveOccurred = pm.UpdateCurViewWithBlock(makeBlock(13, 14), true)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(14), pm.CurView())
	assert.True(t, nveOccurred && nve.View == 14)
}

// Test_FutureBlockWithQcForCurrentView tests that PaceMaker processes the block with
//    block.qc.view > curView
//    block.view > block.qc.view +1
// correctly. Specifically, it should induce a view change to the block.qc.view +1,
// which is not sufficient to process the block. I.e. we expect view change to block.qc.view +1
// enables processing the block right away.
func Test_FutureBlockWithQcForFutureFutureView(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3)
	notifier.On("OnSkippedAhead", uint64(14)).Return().Once()
	notifier.On("OnStartingTimeout", expectedTimerInfo(14, model.ReplicaTimeout)).Return().Once()
	nve, nveOccurred := pm.UpdateCurViewWithBlock(makeBlock(13, 17), false)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(14), pm.CurView())
	assert.True(t, nveOccurred && nve.View == 14)

	pm, notifier = initPaceMaker(t, 3)
	notifier.On("OnSkippedAhead", uint64(14)).Return().Once()
	notifier.On("OnStartingTimeout", expectedTimerInfo(14, model.ReplicaTimeout)).Return().Once()
	nve, nveOccurred = pm.UpdateCurViewWithBlock(makeBlock(13, 17), true)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(14), pm.CurView())
	assert.True(t, nveOccurred && nve.View == 14)
}

// Test_IgnoreBlockDuplicates tests that PaceMaker ignores duplicate blocks
func Test_IgnoreBlockDuplicates(t *testing.T) {
	// NOT Primary for the Block's view
	pm, notifier := initPaceMaker(t, 3)
	notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
	notifier.On("OnStartingTimeout", expectedTimerInfo(5, model.ReplicaTimeout)).Return().Once()
	nve, nveOccurred := pm.UpdateCurViewWithBlock(makeBlock(3, 4), false)
	assert.True(t, nveOccurred && nve.View == 5)
	nve, nveOccurred = pm.UpdateCurViewWithBlock(makeBlock(3, 4), false)
	assert.True(t, !nveOccurred && nve == nil)
	nve, nveOccurred = pm.UpdateCurViewWithBlock(makeBlock(3, 4), false)
	assert.True(t, !nveOccurred && nve == nil)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(5), pm.CurView())

	// PRIMARY for the Block's view
	pm, notifier = initPaceMaker(t, 3)
	notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
	notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.VoteCollectionTimeout)).Return().Once()
	nve, nveOccurred = pm.UpdateCurViewWithBlock(makeBlock(3, 4), true)
	assert.True(t, nveOccurred && nve.View == 4)
	nve, nveOccurred = pm.UpdateCurViewWithBlock(makeBlock(3, 4), true)
	assert.True(t, !nveOccurred && nve == nil)
	nve, nveOccurred = pm.UpdateCurViewWithBlock(makeBlock(3, 4), true)
	assert.True(t, !nveOccurred && nve == nil)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(4), pm.CurView())
}

// Test_ReplicaTimeout tests that replica timeout fires as expected
func Test_ReplicaTimeout(t *testing.T) {
	start := time.Now()
	pm, notifier := initPaceMaker(t, 3) // initPaceMaker also calls Start() on PaceMaker

	select {
	case <-pm.TimeoutChannel():
		break // testing path: corresponds to EventLoop picking up timeout from channel
	case <-time.NewTimer(time.Duration(2) * time.Duration(startRepTimeout) * time.Millisecond).C:
		t.Fail() // to prevent test from hanging
	}
	duration := float64(time.Since(start).Milliseconds()) // in millisecond
	fmt.Println(duration)
	assert.True(t, math.Abs(duration-startRepTimeout) < 0.1*startRepTimeout)
	// While the timeout event has been put in the channel,
	// PaceMaker should NOT react on it without the timeout event being processed by the EventHandler
	assert.Equal(t, uint64(3), pm.CurView())

	// here the, the Event loop would now call EventHandler.OnTimeout() -> PaceMaker.OnTimeout()
	notifier.On("OnReachedTimeout", expectedTimeoutInfo(3, model.ReplicaTimeout)).Return().Once()
	notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
	viewChange := pm.OnTimeout()
	if viewChange == nil {
		assert.Fail(t, "Expecting ViewChange event as result of timeout")
	}

	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(4), pm.CurView())
}

// Test_VoteTimeout tests that vote timeout fires as expected
func Test_VoteTimeout(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3) // initPaceMaker also calls Start() on PaceMaker
	start := time.Now()

	notifier.On("OnStartingTimeout", expectedTimerInfo(3, model.VoteCollectionTimeout)).Return().Once()
	pm.UpdateCurViewWithBlock(makeBlock(2, 3), true)
	notifier.AssertExpectations(t)

	expectedTimeout := startRepTimeout * voteTimeoutFraction
	select {
	case <-pm.TimeoutChannel():
		break // testing path: corresponds to EventLoop picking up timeout from channel
	case <-time.NewTimer(time.Duration(2) * time.Duration(expectedTimeout) * time.Millisecond).C:
		t.Fail() // to prevent test from hanging
	}
	duration := float64(time.Since(start).Milliseconds()) // in millisecond
	fmt.Println(duration)
	assert.True(t, math.Abs(duration-expectedTimeout) < 0.1*expectedTimeout)
	// While the timeout event has been put in the channel,
	// PaceMaker should NOT react on it without the timeout event being processed by the EventHandler
	assert.Equal(t, uint64(3), pm.CurView())

	// here the, the Event loop would now call EventHandler.OnTimeout() -> PaceMaker.OnTimeout()
	notifier.On("OnReachedTimeout", expectedTimeoutInfo(3, model.VoteCollectionTimeout)).Return().Once()
	notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
	viewChange := pm.OnTimeout()
	if viewChange == nil {
		assert.Fail(t, "Expecting ViewChange event as result of timeout")
	}
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(4), pm.CurView())
}
