package cruisectl

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	mockmodule "github.com/onflow/flow-go/module/mock"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// BlockRateControllerSuite encapsulates tests for the BlockRateController.
type BlockRateControllerSuite struct {
	suite.Suite

	initialView            uint64
	epochCounter           uint64
	curEpochFirstView      uint64
	curEpochFinalView      uint64
	epochFallbackTriggered bool

	metrics  *mockmodule.CruiseCtlMetrics
	state    *mockprotocol.State
	params   *mockprotocol.Params
	snapshot *mockprotocol.Snapshot
	epochs   *mocks.EpochQuery
	curEpoch *mockprotocol.Epoch
	config   *Config
	ctx      irrecoverable.SignalerContext
	cancel   context.CancelFunc

	ctl *BlockRateController
}

func TestBlockRateController(t *testing.T) {
	suite.Run(t, new(BlockRateControllerSuite))
}

// SetupTest initializes mocks and default values.
func (bs *BlockRateControllerSuite) SetupTest() {
	bs.config = DefaultConfig()
	bs.initialView = 0
	bs.epochCounter = uint64(0)
	bs.curEpochFirstView = uint64(0)
	bs.curEpochFinalView = uint64(604_800) // 1 view/sec
	bs.epochFallbackTriggered = false

	bs.metrics = mockmodule.NewCruiseCtlMetrics(bs.T())
	bs.metrics.On("PIDError", mock.Anything, mock.Anything, mock.Anything).Maybe()
	bs.metrics.On("TargetProposalDuration", mock.Anything).Maybe()
	bs.metrics.On("ControllerOutput", mock.Anything).Maybe()

	bs.state = mockprotocol.NewState(bs.T())
	bs.params = mockprotocol.NewParams(bs.T())
	bs.snapshot = mockprotocol.NewSnapshot(bs.T())
	bs.epochs = mocks.NewEpochQuery(bs.T(), bs.epochCounter)
	bs.curEpoch = mockprotocol.NewEpoch(bs.T())

	bs.state.On("Final").Return(bs.snapshot)
	bs.state.On("AtHeight", mock.Anything).Return(bs.snapshot).Maybe()
	bs.state.On("Params").Return(bs.params)
	bs.params.On("EpochFallbackTriggered").Return(
		func() bool { return bs.epochFallbackTriggered },
		func() error { return nil })
	bs.snapshot.On("Phase").Return(
		func() flow.EpochPhase { return bs.epochs.Phase() },
		func() error { return nil })
	bs.snapshot.On("Epochs").Return(bs.epochs)
	bs.curEpoch.On("Counter").Return(bs.epochCounter, nil)
	bs.curEpoch.On("FirstView").Return(bs.curEpochFirstView, nil)
	bs.curEpoch.On("FinalView").Return(bs.curEpochFinalView, nil)
	bs.epochs.Add(bs.curEpoch)

	bs.ctx, bs.cancel = irrecoverable.NewMockSignalerContextWithCancel(bs.T(), context.Background())
}

// CreateAndStartController creates and starts the BlockRateController.
// Should be called only once per test case.
func (bs *BlockRateControllerSuite) CreateAndStartController() {
	ctl, err := NewBlockRateController(unittest.Logger(), bs.metrics, bs.config, bs.state, bs.initialView)
	require.NoError(bs.T(), err)
	bs.ctl = ctl
	bs.ctl.Start(bs.ctx)
	unittest.RequireCloseBefore(bs.T(), bs.ctl.Ready(), time.Second, "component did not start")
}

// StopController stops the BlockRateController.
func (bs *BlockRateControllerSuite) StopController() {
	bs.cancel()
	unittest.RequireCloseBefore(bs.T(), bs.ctl.Done(), time.Second, "component did not stop")
}

// AssertCorrectInitialization checks that the controller is configured as expected after construction.
func (bs *BlockRateControllerSuite) AssertCorrectInitialization() {
	// proposal delay should be initialized to default value
	assert.Equal(bs.T(), bs.ctl.config.DefaultProposalDuration, bs.ctl.ProposalDuration())

	// if epoch fallback is triggered, we don't care about anything else
	if bs.ctl.epochFallbackTriggered {
		return
	}

	// should initialize epoch info
	epoch := bs.ctl.epochInfo
	expectedEndTime := bs.config.TargetTransition.inferTargetEndTime(time.Now(), epoch.fractionComplete(bs.initialView))
	assert.Equal(bs.T(), bs.curEpochFirstView, epoch.curEpochFirstView)
	assert.Equal(bs.T(), bs.curEpochFinalView, epoch.curEpochFinalView)
	assert.Equal(bs.T(), expectedEndTime, epoch.curEpochTargetEndTime)

	// if next epoch is setup, final view should be set
	if phase := bs.epochs.Phase(); phase > flow.EpochPhaseStaking {
		finalView, err := bs.epochs.Next().FinalView()
		require.NoError(bs.T(), err)
		assert.Equal(bs.T(), finalView, *epoch.nextEpochFinalView)
	} else {
		assert.Nil(bs.T(), epoch.nextEpochFinalView)
	}

	// should create an initial measurement
	lastMeasurement := bs.ctl.lastMeasurement
	assert.Equal(bs.T(), bs.initialView, lastMeasurement.view)
	assert.WithinDuration(bs.T(), time.Now(), lastMeasurement.time, time.Minute)
	// errors should be initialized to zero
	assert.Equal(bs.T(), float64(0), lastMeasurement.proportionalErr+lastMeasurement.integralErr+lastMeasurement.derivativeErr)
}

// SanityCheckSubsequentMeasurements checks that two consecutive measurements are different and broadly reasonable.
// It does not assert exact values, because part of the measurements depend on timing in the worker.
func (bs *BlockRateControllerSuite) SanityCheckSubsequentMeasurements(m1, m2 measurement) {
	// later measurements should have later times
	assert.True(bs.T(), m1.time.Before(m2.time))
	// new measurement should have different error
	assert.NotEqual(bs.T(), m1.proportionalErr, m2.proportionalErr)
	assert.NotEqual(bs.T(), m1.integralErr, m2.integralErr)
	assert.NotEqual(bs.T(), m1.derivativeErr, m2.derivativeErr)
}

// PrintMeasurement prints the current state of the controller and the last measurement.
func (bs *BlockRateControllerSuite) PrintMeasurement() {
	ctl := bs.ctl
	m := ctl.lastMeasurement
	fmt.Printf("v=%d\tt=%s\tu=%s\tPD=%s\te=%.3f\te_N=%.3f\tI_M=%.3f\t∆_N=%.3f\n",
		m.view, m.time, ctl.controllerOutput(), ctl.ProposalDuration(),
		m.instErr, m.proportionalErr, m.instErr, m.derivativeErr)
}

// TestStartStop tests that the component can be started and stopped gracefully.
func (bs *BlockRateControllerSuite) TestStartStop() {
	bs.CreateAndStartController()
	bs.StopController()
}

// TestInit_EpochStakingPhase tests initializing the component in the EpochStaking phase.
// Measurement and epoch info should be initialized, next epoch final view should be nil.
func (bs *BlockRateControllerSuite) TestInit_EpochStakingPhase() {
	bs.CreateAndStartController()
	defer bs.StopController()
	bs.AssertCorrectInitialization()
}

// TestInit_EpochStakingPhase tests initializing the component in the EpochSetup phase.
// Measurement and epoch info should be initialized, next epoch final view should be set.
func (bs *BlockRateControllerSuite) TestInit_EpochSetupPhase() {
	nextEpoch := mockprotocol.NewEpoch(bs.T())
	nextEpoch.On("Counter").Return(bs.epochCounter+1, nil)
	nextEpoch.On("FinalView").Return(bs.curEpochFinalView+100_000, nil)
	bs.epochs.Add(nextEpoch)

	bs.CreateAndStartController()
	defer bs.StopController()
	bs.AssertCorrectInitialization()
}

// TestInit_EpochFallbackTriggered tests initializing the component when epoch fallback is triggered.
// Default ProposalDuration should be set.
func (bs *BlockRateControllerSuite) TestInit_EpochFallbackTriggered() {
	bs.epochFallbackTriggered = true
	bs.CreateAndStartController()
	defer bs.StopController()
	bs.AssertCorrectInitialization()
}

// TestEpochFallbackTriggered tests epoch fallback:
//   - the ProposalDuration should revert to default
//   - duplicate events should be no-ops
func (bs *BlockRateControllerSuite) TestEpochFallbackTriggered() {
	bs.CreateAndStartController()
	defer bs.StopController()

	// update error so that ProposalDuration is non-default
	bs.ctl.lastMeasurement.instErr *= 1.1
	err := bs.ctl.measureViewDuration(bs.initialView+1, time.Now())
	require.NoError(bs.T(), err)
	assert.NotEqual(bs.T(), bs.config.DefaultProposalDuration, bs.ctl.ProposalDuration())

	// send the event
	bs.ctl.EpochEmergencyFallbackTriggered()
	// async: should revert to default ProposalDuration
	require.Eventually(bs.T(), func() bool {
		return bs.config.DefaultProposalDuration == bs.ctl.ProposalDuration()
	}, time.Second, time.Millisecond)

	// additional EpochEmergencyFallbackTriggered events should be no-ops
	// (send capacity+1 events to guarantee one is processed)
	for i := 0; i <= cap(bs.ctl.epochFallbacks); i++ {
		bs.ctl.EpochEmergencyFallbackTriggered()
	}
	// state should be unchanged
	assert.Equal(bs.T(), bs.config.DefaultProposalDuration, bs.ctl.ProposalDuration())

	// addition OnViewChange events should be no-ops
	for i := 0; i <= cap(bs.ctl.viewChanges); i++ {
		bs.ctl.OnViewChange(0, bs.initialView+1)
	}
	// wait for the channel to drain, since OnViewChange doesn't block on sending
	require.Eventually(bs.T(), func() bool {
		return len(bs.ctl.viewChanges) == 0
	}, time.Second, time.Millisecond)
	// state should be unchanged
	assert.Equal(bs.T(), bs.ctl.config.DefaultProposalDuration, bs.ctl.ProposalDuration())
}

// TestOnViewChange_UpdateProposalDelay tests that a new measurement is taken and
// ProposalDuration updated upon receiving an OnViewChange event.
func (bs *BlockRateControllerSuite) TestOnViewChange_UpdateProposalDelay() {
	bs.CreateAndStartController()
	defer bs.StopController()

	initialMeasurement := bs.ctl.lastMeasurement
	initialProposalDelay := bs.ctl.ProposalDuration()
	bs.ctl.OnViewChange(0, bs.initialView+1)
	require.Eventually(bs.T(), func() bool {
		return bs.ctl.lastMeasurement.view > bs.initialView
	}, time.Second, time.Millisecond)
	nextMeasurement := bs.ctl.lastMeasurement
	nextProposalDelay := bs.ctl.ProposalDuration()

	bs.SanityCheckSubsequentMeasurements(initialMeasurement, nextMeasurement)
	// new measurement should update ProposalDuration
	assert.NotEqual(bs.T(), initialProposalDelay, nextProposalDelay)

	// duplicate events should be no-ops
	for i := 0; i <= cap(bs.ctl.viewChanges); i++ {
		bs.ctl.OnViewChange(0, bs.initialView+1)
	}
	// wait for the channel to drain, since OnViewChange doesn't block on sending
	require.Eventually(bs.T(), func() bool {
		return len(bs.ctl.viewChanges) == 0
	}, time.Second, time.Millisecond)

	// state should be unchanged
	assert.Equal(bs.T(), nextMeasurement, bs.ctl.lastMeasurement)
	assert.Equal(bs.T(), nextProposalDelay, bs.ctl.ProposalDuration())
}

// TestOnViewChange_EpochTransition tests that a view change into the next epoch
// updates the local state to reflect the new epoch.
func (bs *BlockRateControllerSuite) TestOnViewChange_EpochTransition() {
	nextEpoch := mockprotocol.NewEpoch(bs.T())
	nextEpoch.On("Counter").Return(bs.epochCounter+1, nil)
	nextEpoch.On("FinalView").Return(bs.curEpochFinalView+100_000, nil)
	bs.epochs.Add(nextEpoch)
	bs.CreateAndStartController()
	defer bs.StopController()

	initialMeasurement := bs.ctl.lastMeasurement
	bs.epochs.Transition()
	bs.ctl.OnViewChange(0, bs.curEpochFinalView+1)
	require.Eventually(bs.T(), func() bool {
		return bs.ctl.lastMeasurement.view > bs.initialView
	}, time.Second, time.Millisecond)
	nextMeasurement := bs.ctl.lastMeasurement

	bs.SanityCheckSubsequentMeasurements(initialMeasurement, nextMeasurement)
	// epoch boundaries should be updated
	assert.Equal(bs.T(), bs.curEpochFinalView+1, bs.ctl.epochInfo.curEpochFirstView)
	assert.Equal(bs.T(), bs.ctl.epochInfo.curEpochFinalView, bs.curEpochFinalView+100_000)
	assert.Nil(bs.T(), bs.ctl.nextEpochFinalView)
}

// TestOnEpochSetupPhaseStarted ensures that the epoch info is updated when the next epoch is setup.
func (bs *BlockRateControllerSuite) TestOnEpochSetupPhaseStarted() {
	nextEpoch := mockprotocol.NewEpoch(bs.T())
	nextEpoch.On("Counter").Return(bs.epochCounter+1, nil)
	nextEpoch.On("FinalView").Return(bs.curEpochFinalView+100_000, nil)
	bs.epochs.Add(nextEpoch)
	bs.CreateAndStartController()
	defer bs.StopController()

	header := unittest.BlockHeaderFixture()
	bs.ctl.EpochSetupPhaseStarted(bs.epochCounter, header)
	require.Eventually(bs.T(), func() bool {
		return bs.ctl.nextEpochFinalView != nil
	}, time.Second, time.Millisecond)

	assert.Equal(bs.T(), bs.curEpochFinalView+100_000, *bs.ctl.nextEpochFinalView)

	// duplicate events should be no-ops
	for i := 0; i <= cap(bs.ctl.epochSetups); i++ {
		bs.ctl.EpochSetupPhaseStarted(bs.epochCounter, header)
	}
	assert.Equal(bs.T(), bs.curEpochFinalView+100_000, *bs.ctl.nextEpochFinalView)
}

// TestProposalDelay_AfterTargetTransitionTime tests the behaviour of the controller
// when we have passed the target end time for the current epoch.
// We should approach the min ProposalDuration (increase view rate as much as possible)
func (bs *BlockRateControllerSuite) TestProposalDelay_AfterTargetTransitionTime() {
	// we are near the end of the epoch in view terms
	bs.initialView = uint64(float64(bs.curEpochFinalView) * .95)
	bs.CreateAndStartController()
	defer bs.StopController()

	lastProposalDelay := bs.ctl.ProposalDuration()
	for view := bs.initialView + 1; view < bs.ctl.curEpochFinalView; view++ {
		// we have passed the target end time of the epoch
		enteredViewAt := bs.ctl.curEpochTargetEndTime.Add(time.Duration(view) * time.Second)
		err := bs.ctl.measureViewDuration(view, enteredViewAt)
		require.NoError(bs.T(), err)

		assert.LessOrEqual(bs.T(), bs.ctl.ProposalDuration(), lastProposalDelay)
		lastProposalDelay = bs.ctl.ProposalDuration()

		// transition views until the end of the epoch, or for 100 views
		if view-bs.initialView >= 100 {
			break
		}
	}
}

// TestProposalDelay_BehindSchedule tests the behaviour of the controller when the
// projected epoch switchover is LATER than the target switchover time (in other words,
// we are behind schedule.
// We should respond by lowering the ProposalDuration (increasing view rate)
func (bs *BlockRateControllerSuite) TestProposalDelay_BehindSchedule() {
	// we are 50% of the way through the epoch in view terms
	bs.initialView = uint64(float64(bs.curEpochFinalView) * .5)
	bs.CreateAndStartController()
	defer bs.StopController()

	lastProposalDelay := bs.ctl.ProposalDuration()
	idealEnteredViewTime := bs.ctl.curEpochTargetEndTime.Add(-epochLength / 2)
	// 1s behind of schedule
	enteredViewAt := idealEnteredViewTime.Add(time.Second)
	for view := bs.initialView + 1; view < bs.ctl.curEpochFinalView; view++ {
		// hold the instantaneous error constant for each view
		enteredViewAt = enteredViewAt.Add(bs.ctl.targetViewTime())
		err := bs.ctl.measureViewDuration(view, enteredViewAt)
		require.NoError(bs.T(), err)

		// decreasing ProposalDuration
		assert.LessOrEqual(bs.T(), bs.ctl.ProposalDuration(), lastProposalDelay)
		lastProposalDelay = bs.ctl.ProposalDuration()

		// transition views until the end of the epoch, or for 100 views
		if view-bs.initialView >= 100 {
			break
		}
	}
}

// TestProposalDelay_AheadOfSchedule tests the behaviour of the controller when the
// projected epoch switchover is EARLIER than the target switchover time (in other words,
// we are ahead of schedule.
// We should respond by increasing the ProposalDuration (lowering view rate)
func (bs *BlockRateControllerSuite) TestProposalDelay_AheadOfSchedule() {
	// we are 50% of the way through the epoch in view terms
	bs.initialView = uint64(float64(bs.curEpochFinalView) * .5)
	bs.CreateAndStartController()
	defer bs.StopController()

	lastProposalDelay := bs.ctl.ProposalDuration()
	idealEnteredViewTime := bs.ctl.curEpochTargetEndTime.Add(-epochLength / 2)
	// 1s ahead of schedule
	enteredViewAt := idealEnteredViewTime.Add(-time.Second)
	for view := bs.initialView + 1; view < bs.ctl.curEpochFinalView; view++ {
		// hold the instantaneous error constant for each view
		enteredViewAt = enteredViewAt.Add(bs.ctl.targetViewTime())
		err := bs.ctl.measureViewDuration(view, enteredViewAt)
		require.NoError(bs.T(), err)

		// increasing ProposalDuration
		assert.GreaterOrEqual(bs.T(), bs.ctl.ProposalDuration(), lastProposalDelay)
		lastProposalDelay = bs.ctl.ProposalDuration()

		// transition views until the end of the epoch, or for 100 views
		if view-bs.initialView >= 100 {
			break
		}
	}
}

// TestMetrics tests that correct metrics are tracked when expected.
func (bs *BlockRateControllerSuite) TestMetrics() {
	bs.metrics = mockmodule.NewCruiseCtlMetrics(bs.T())
	// should set metrics upon initialization
	bs.metrics.On("PIDError", float64(0), float64(0), float64(0)).Once()
	bs.metrics.On("TargetProposalDuration", bs.config.DefaultProposalDuration).Once()
	bs.metrics.On("ControllerOutput", time.Duration(0)).Once()
	bs.CreateAndStartController()
	defer bs.StopController()
	bs.metrics.AssertExpectations(bs.T())

	// we are at view 1 of the epoch, but the time is suddenly the target end time
	enteredViewAt := bs.ctl.curEpochTargetEndTime
	view := bs.initialView + 1
	// we should observe a large error
	bs.metrics.On("PIDError", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		p := args[0].(float64)
		i := args[1].(float64)
		d := args[2].(float64)
		assert.Greater(bs.T(), p, float64(0))
		assert.Greater(bs.T(), i, float64(0))
		assert.Greater(bs.T(), d, float64(0))
	}).Once()
	// should immediately use min proposal duration
	bs.metrics.On("TargetProposalDuration", bs.config.MinProposalDuration).Once()
	// should have a large negative controller output
	bs.metrics.On("ControllerOutput", mock.Anything).Run(func(args mock.Arguments) {
		output := args[0].(time.Duration)
		assert.Greater(bs.T(), output, time.Duration(0))
	}).Once()

	err := bs.ctl.measureViewDuration(view, enteredViewAt)
	require.NoError(bs.T(), err)
}
