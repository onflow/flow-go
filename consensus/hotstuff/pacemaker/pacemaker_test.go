package pacemaker

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/pacemaker/timeout"
)

const (
	startRepTimeout        float64 = 400.0 // Milliseconds
	minRepTimeout          float64 = 100.0 // Milliseconds
	voteTimeoutFraction    float64 = 0.5   // multiplicative factor
	multiplicativeIncrease float64 = 1.5   // multiplicative factor
	multiplicativeDecrease float64 = 0.85  // multiplicative factor
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

func initPaceMaker(t *testing.T, view uint64) (hotstuff.PaceMaker, *mocks.Consumer) {
	notifier := &mocks.Consumer{}
	tc, err := timeout.NewConfig(
		time.Duration(startRepTimeout*1e6),
		time.Duration(minRepTimeout*1e6),
		voteTimeoutFraction,
		multiplicativeIncrease,
		multiplicativeDecrease,
		0)
	if err != nil {
		t.Fail()
	}
	pm, err := New(view, timeout.NewController(tc), notifier)
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
	case <-time.After(time.Duration(2) * time.Duration(startRepTimeout) * time.Millisecond):
		t.Fail() // to prevent test from hanging
	}
	actualTimeout := float64(time.Since(start).Milliseconds()) // in millisecond
	fmt.Println(actualTimeout)
	assert.GreaterOrEqual(t, actualTimeout, startRepTimeout, "the actual timeout should be greater or equal to the init timeout")
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

func Test_ReplicaTimeoutAgain(t *testing.T) {
	start := time.Now()
	pm, notifier := initPaceMaker(t, 3) // initPaceMaker also calls Start() on PaceMaker
	notifier.On("OnReachedTimeout", mock.Anything)
	notifier.On("OnStartingTimeout", mock.Anything)

	// wait until the timeout is hit for the first time
	select {
	case <-pm.TimeoutChannel():
	case <-time.After(time.Duration(2*startRepTimeout) * time.Millisecond):
	}

	// calculate the actual timeout duration that has been waited.
	actualTimeout := float64(time.Since(start).Milliseconds()) // in millisecond
	assert.GreaterOrEqual(t, actualTimeout, startRepTimeout, "the actual timeout should be greater or equal to the init timeout")
	assert.Less(t, actualTimeout, 2*startRepTimeout, "the actual timeout was too long")

	// reset timer
	start = time.Now()

	// trigger view change and restart the pacemaker timer. The next timeout should take 1.5 longer
	_ = pm.OnTimeout()

	// wait until the timeout is hit again
	select {
	case <-pm.TimeoutChannel():
	case <-time.After(time.Duration(3) * time.Duration(startRepTimeout) * time.Millisecond):
	}

	nv := pm.OnTimeout()

	// calculate the actual timeout duration that has been waited again
	actualTimeout = float64(time.Since(start).Milliseconds()) // in millisecond
	assert.GreaterOrEqual(t, actualTimeout, 1.5*startRepTimeout, "the actual timeout should be greater or equal to the timeout")
	assert.Less(t, actualTimeout, 2*startRepTimeout, "the actual timeout was too long")

	var changed bool
	// timeout reduce linearly
	select {
	case <-pm.TimeoutChannel():
		break
	case <-time.After(time.Duration(0.1 * startRepTimeout)):
		nv, changed = pm.UpdateCurViewWithBlock(makeBlock(nv.View-1, nv.View), false)
		break
	}

	require.True(t, changed)

	// timeout reduce linearly
	select {
	case <-pm.TimeoutChannel():
		break
	case <-time.After(time.Duration(0.1 * startRepTimeout)):
		// start the timer before the UpdateCurViewWithBlock is called, which starts the internal timer.
		// This is required to make the tests more accurate, because when it's running on CI,
		// there might be some delay for time.Now() gets called, which impact the accuracy of the actual timeout
		// measurement

		start = time.Now()
		nv, changed = pm.UpdateCurViewWithBlock(makeBlock(nv.View-1, nv.View), false)
		break
	}

	require.True(t, changed)

	// wait until the timeout is hit again
	select {
	case <-pm.TimeoutChannel():
	case <-time.After(time.Duration(3) * time.Duration(startRepTimeout) * time.Millisecond):
	}

	actualTimeout = float64(time.Since(start).Microseconds()) * 0.001 // in millisecond
	// the actual timeout should be startRepTimeout * 1.5 *1.5- 2*additiveDecrease
	// because it hits timeout twice and then received blocks for current view twice
	assert.GreaterOrEqual(t, actualTimeout, 1.5*1.5*startRepTimeout*multiplicativeDecrease*multiplicativeDecrease, "the actual timeout should be greater or equal to the timeout")
	assert.Less(t, actualTimeout, 1.5*1.5*startRepTimeout*multiplicativeDecrease, "the actual timeout is too long")
}

// Test_VoteTimeout tests that vote timeout fires as expected
func Test_VoteTimeout(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3) // initPaceMaker also calls Start() on PaceMaker
	start := time.Now()

	notifier.On("OnStartingTimeout", expectedTimerInfo(3, model.VoteCollectionTimeout)).Return().Once()
	pm.UpdateCurViewWithBlock(makeBlock(2, 3), true)
	notifier.AssertExpectations(t)

	// the previous pacemaker update decreased the vote timeout by (additiveDecrease * voteTimeoutFraction)
	expectedTimeout := (startRepTimeout * multiplicativeDecrease) * voteTimeoutFraction
	select {
	case <-pm.TimeoutChannel():
		break // testing path: corresponds to EventLoop picking up timeout from channel
	case <-time.After(time.Duration(2) * time.Duration(expectedTimeout) * time.Millisecond):
		t.Fail() // to prevent test from hanging
	}
	duration := float64(time.Since(start).Milliseconds()) // in millisecond
	fmt.Println(duration)
	fmt.Println(expectedTimeout)
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
