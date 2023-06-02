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
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	mockmodule "github.com/onflow/flow-go/module/mock"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// BlockRateControllerSuite encapsulates tests for the BlockTimeController.
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

	ctl *BlockTimeController
}

func TestBlockRateController(t *testing.T) {
	suite.Run(t, new(BlockRateControllerSuite))
}

// SetupTest initializes mocks and default values.
func (bs *BlockRateControllerSuite) SetupTest() {
	bs.config = DefaultConfig()
	bs.config.Enabled.Store(true)
	bs.initialView = 0
	bs.epochCounter = uint64(0)
	bs.curEpochFirstView = uint64(0)
	bs.curEpochFinalView = uint64(604_800) // 1 view/sec
	bs.epochFallbackTriggered = false
	setupMocks(bs)
}

func setupMocks(bs *BlockRateControllerSuite) {
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
	bs.snapshot.On("Head").Return(unittest.BlockHeaderFixture(unittest.HeaderWithView(bs.initialView+11)), nil).Maybe()
	bs.snapshot.On("Epochs").Return(bs.epochs)
	bs.curEpoch.On("Counter").Return(bs.epochCounter, nil)
	bs.curEpoch.On("FirstView").Return(bs.curEpochFirstView, nil)
	bs.curEpoch.On("FinalView").Return(bs.curEpochFinalView, nil)
	bs.epochs.Add(bs.curEpoch)

	bs.ctx, bs.cancel = irrecoverable.NewMockSignalerContextWithCancel(bs.T(), context.Background())
}

// CreateAndStartController creates and starts the BlockTimeController.
// Should be called only once per test case.
func (bs *BlockRateControllerSuite) CreateAndStartController() {
	ctl, err := NewBlockTimeController(unittest.Logger(), bs.metrics, bs.config, bs.state, bs.initialView)
	require.NoError(bs.T(), err)
	bs.ctl = ctl
	bs.ctl.Start(bs.ctx)
	unittest.RequireCloseBefore(bs.T(), bs.ctl.Ready(), time.Second, "component did not start")
}

// StopController stops the BlockTimeController.
func (bs *BlockRateControllerSuite) StopController() {
	bs.cancel()
	unittest.RequireCloseBefore(bs.T(), bs.ctl.Done(), time.Second, "component did not stop")
}

// AssertCorrectInitialization checks that the controller is configured as expected after construction.
func (bs *BlockRateControllerSuite) AssertCorrectInitialization() {
	// at initialization, controller should be set up to release blocks without delay
	controllerTiming := bs.ctl.GetProposalTiming()
	now := time.Now().UTC()

	if bs.ctl.epochFallbackTriggered || !bs.ctl.config.Enabled.Load() {
		// if epoch fallback is triggered or controller is disabled, use fallback timing
		assert.Equal(bs.T(), now.Add(bs.ctl.config.FallbackProposalDelay.Load()), controllerTiming.TargetPublicationTime(7, now, unittest.IdentifierFixture()))
	} else {
		// otherwise should publish immediately
		assert.Equal(bs.T(), now, controllerTiming.TargetPublicationTime(7, now, unittest.IdentifierFixture()))
	}
	if bs.ctl.epochFallbackTriggered {
		return
	}

	// should initialize epoch info
	epoch := bs.ctl.epochInfo
	expectedEndTime := bs.config.TargetTransition.inferTargetEndTime(time.Now(), epoch.fractionComplete(bs.initialView))
	assert.Equal(bs.T(), bs.curEpochFirstView, epoch.curEpochFirstView)
	assert.Equal(bs.T(), bs.curEpochFinalView, epoch.curEpochFinalView)
	assert.Equal(bs.T(), expectedEndTime, epoch.curEpochTargetEndTime)

	// if next epoch is set up, final view should be set
	if phase := bs.epochs.Phase(); phase > flow.EpochPhaseStaking {
		finalView, err := bs.epochs.Next().FinalView()
		require.NoError(bs.T(), err)
		assert.Equal(bs.T(), finalView, *epoch.nextEpochFinalView)
	} else {
		assert.Nil(bs.T(), epoch.nextEpochFinalView)
	}

	// should create an initial measurement
	assert.Equal(bs.T(), bs.initialView, controllerTiming.ObservationView())
	assert.WithinDuration(bs.T(), time.Now(), controllerTiming.ObservationTime(), time.Minute)
	// errors should be initialized to zero
	assert.Equal(bs.T(), float64(0), bs.ctl.proportionalErr.Value())
	assert.Equal(bs.T(), float64(0), bs.ctl.integralErr.Value())
}

// SanityCheckSubsequentMeasurements checks that two consecutive states of the BlockTimeController are different or equal and
// broadly reasonable. It does not assert exact values, because part of the measurements depend on timing in the worker.
func (bs *BlockRateControllerSuite) SanityCheckSubsequentMeasurements(d1, d2 *controllerStateDigest, expectedEqual bool) {
	if expectedEqual {
		// later input should have left state invariant, including the Observation
		assert.Equal(bs.T(), d1.latestProposalTiming.ObservationTime(), d2.latestProposalTiming.ObservationTime())
		assert.Equal(bs.T(), d1.latestProposalTiming.ObservationView(), d2.latestProposalTiming.ObservationView())
		// new measurement should have same error
		assert.Equal(bs.T(), d1.proportionalErr.Value(), d2.proportionalErr.Value())
		assert.Equal(bs.T(), d1.integralErr.Value(), d2.integralErr.Value())
	} else {
		// later input should have caused a new Observation to be recorded
		assert.True(bs.T(), d1.latestProposalTiming.ObservationTime().Before(d2.latestProposalTiming.ObservationTime()))
		// new measurement should have different error
		assert.NotEqual(bs.T(), d1.proportionalErr.Value(), d2.proportionalErr.Value())
		assert.NotEqual(bs.T(), d1.integralErr.Value(), d2.integralErr.Value())
	}
}

// PrintMeasurement prints the current state of the controller and the last measurement.
func (bs *BlockRateControllerSuite) PrintMeasurement(parentBlockId flow.Identifier) {
	ctl := bs.ctl
	m := ctl.GetProposalTiming()
	tpt := m.TargetPublicationTime(m.ObservationView()+1, m.ObservationTime(), parentBlockId)
	fmt.Printf("v=%d\tt=%s\tPD=%s\te_N=%.3f\tI_M=%.3f\n",
		m.ObservationView(), m.ObservationTime(), tpt.Sub(m.ObservationTime()),
		ctl.proportionalErr.Value(), ctl.integralErr.Value())
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
// Default GetProposalTiming should be set.
func (bs *BlockRateControllerSuite) TestInit_EpochFallbackTriggered() {
	bs.epochFallbackTriggered = true
	bs.CreateAndStartController()
	defer bs.StopController()
	bs.AssertCorrectInitialization()
}

// TestEpochFallbackTriggered tests epoch fallback:
//   - the GetProposalTiming should revert to default
//   - duplicate events should be no-ops
func (bs *BlockRateControllerSuite) TestEpochFallbackTriggered() {
	bs.CreateAndStartController()
	defer bs.StopController()

	// update error so that GetProposalTiming is non-default
	bs.ctl.proportionalErr.AddObservation(20.0)
	bs.ctl.integralErr.AddObservation(20.0)
	err := bs.ctl.measureViewDuration(makeTimedBlock(bs.initialView+1, unittest.IdentifierFixture(), time.Now()))
	require.NoError(bs.T(), err)
	assert.NotEqual(bs.T(), bs.config.FallbackProposalDelay, bs.ctl.GetProposalTiming())

	// send the event
	bs.ctl.EpochEmergencyFallbackTriggered()
	// async: should revert to default GetProposalTiming
	require.Eventually(bs.T(), func() bool {
		now := time.Now().UTC()
		return now.Add(bs.config.FallbackProposalDelay.Load()) == bs.ctl.GetProposalTiming().TargetPublicationTime(7, now, unittest.IdentifierFixture())
	}, time.Second, time.Millisecond)

	// additional EpochEmergencyFallbackTriggered events should be no-ops
	// (send capacity+1 events to guarantee one is processed)
	for i := 0; i <= cap(bs.ctl.epochFallbacks); i++ {
		bs.ctl.EpochEmergencyFallbackTriggered()
	}
	// state should be unchanged
	now := time.Now().UTC()
	assert.Equal(bs.T(), now.Add(bs.config.FallbackProposalDelay.Load()), bs.ctl.GetProposalTiming().TargetPublicationTime(12, now, unittest.IdentifierFixture()))

	// additional OnBlockIncorporated events should be no-ops
	for i := 0; i <= cap(bs.ctl.incorporatedBlocks); i++ {
		header := unittest.BlockHeaderFixture(unittest.HeaderWithView(bs.initialView + 1 + uint64(i)))
		header.ParentID = unittest.IdentifierFixture()
		bs.ctl.OnBlockIncorporated(model.BlockFromFlow(header))
	}
	// wait for the channel to drain, since OnBlockIncorporated doesn't block on sending
	require.Eventually(bs.T(), func() bool {
		return len(bs.ctl.incorporatedBlocks) == 0
	}, time.Second, time.Millisecond)
	// state should be unchanged
	now = time.Now().UTC()
	assert.Equal(bs.T(), now.Add(bs.config.FallbackProposalDelay.Load()), bs.ctl.GetProposalTiming().TargetPublicationTime(17, now, unittest.IdentifierFixture()))
}

// TestOnBlockIncorporated_UpdateProposalDelay tests that a new measurement is taken and
// GetProposalTiming updated upon receiving an OnBlockIncorporated event.
func (bs *BlockRateControllerSuite) TestOnBlockIncorporated_UpdateProposalDelay() {
	bs.CreateAndStartController()
	defer bs.StopController()

	initialControllerState := captureControllerStateDigest(bs.ctl) // copy initial controller state
	initialProposalDelay := bs.ctl.GetProposalTiming()
	block := model.BlockFromFlow(unittest.BlockHeaderFixture(unittest.HeaderWithView(bs.initialView + 1)))
	bs.ctl.OnBlockIncorporated(block)
	require.Eventually(bs.T(), func() bool {
		return bs.ctl.GetProposalTiming().ObservationView() > bs.initialView
	}, time.Second, time.Millisecond)
	nextControllerState := captureControllerStateDigest(bs.ctl)
	nextProposalDelay := bs.ctl.GetProposalTiming()

	bs.SanityCheckSubsequentMeasurements(initialControllerState, nextControllerState, false)
	// new measurement should update GetProposalTiming
	now := time.Now().UTC()
	assert.NotEqual(bs.T(),
		initialProposalDelay.TargetPublicationTime(bs.initialView+2, now, unittest.IdentifierFixture()),
		nextProposalDelay.TargetPublicationTime(bs.initialView+2, now, block.BlockID))

	// duplicate events should be no-ops
	for i := 0; i <= cap(bs.ctl.incorporatedBlocks); i++ {
		bs.ctl.OnBlockIncorporated(block)
	}
	// wait for the channel to drain, since OnBlockIncorporated doesn't block on sending
	require.Eventually(bs.T(), func() bool {
		return len(bs.ctl.incorporatedBlocks) == 0
	}, time.Second, time.Millisecond)

	// state should be unchanged
	finalControllerState := captureControllerStateDigest(bs.ctl)
	bs.SanityCheckSubsequentMeasurements(nextControllerState, finalControllerState, true)
	assert.Equal(bs.T(), nextProposalDelay, bs.ctl.GetProposalTiming())
}

// TestEnableDisable tests that the controller responds to enabling and disabling.
func (bs *BlockRateControllerSuite) TestEnableDisable() {
	// start in a disabled state
	err := bs.config.SetEnabled(false)
	require.NoError(bs.T(), err)
	bs.CreateAndStartController()
	defer bs.StopController()

	now := time.Now()

	initialControllerState := captureControllerStateDigest(bs.ctl)
	initialProposalDelay := bs.ctl.GetProposalTiming()
	// the initial proposal timing should use fallback timing
	assert.Equal(bs.T(), now.Add(bs.ctl.config.FallbackProposalDelay.Load()), initialProposalDelay.TargetPublicationTime(bs.initialView+1, now, unittest.IdentifierFixture()))

	block := model.BlockFromFlow(unittest.BlockHeaderFixture(unittest.HeaderWithView(bs.initialView + 1)))
	bs.ctl.OnBlockIncorporated(block)
	require.Eventually(bs.T(), func() bool {
		return bs.ctl.GetProposalTiming().ObservationView() > bs.initialView
	}, time.Second, time.Millisecond)
	secondProposalDelay := bs.ctl.GetProposalTiming()

	// new measurement should not change GetProposalTiming
	assert.Equal(bs.T(),
		initialProposalDelay.TargetPublicationTime(bs.initialView+2, now, unittest.IdentifierFixture()),
		secondProposalDelay.TargetPublicationTime(bs.initialView+2, now, block.BlockID))

	// now, enable the controller
	err = bs.ctl.config.SetEnabled(true)
	require.NoError(bs.T(), err)

	// send another block
	block = model.BlockFromFlow(unittest.BlockHeaderFixture(unittest.HeaderWithView(bs.initialView + 2)))
	bs.ctl.OnBlockIncorporated(block)
	require.Eventually(bs.T(), func() bool {
		return bs.ctl.GetProposalTiming().ObservationView() > bs.initialView
	}, time.Second, time.Millisecond)

	thirdControllerState := captureControllerStateDigest(bs.ctl)
	thirdProposalDelay := bs.ctl.GetProposalTiming()

	// new measurement should change GetProposalTiming
	bs.SanityCheckSubsequentMeasurements(initialControllerState, thirdControllerState, false)
	assert.NotEqual(bs.T(),
		initialProposalDelay.TargetPublicationTime(bs.initialView+3, now, unittest.IdentifierFixture()),
		thirdProposalDelay.TargetPublicationTime(bs.initialView+3, now, block.BlockID))

}

// TestOnBlockIncorporated_EpochTransition_Enabled tests epoch transition with controller enabled.
func (bs *BlockRateControllerSuite) TestOnBlockIncorporated_EpochTransition_Enabled() {
	err := bs.ctl.config.SetEnabled(true)
	require.NoError(bs.T(), err)
	bs.testOnBlockIncorporated_EpochTransition()
}

// TestOnBlockIncorporated_EpochTransition_Disabled tests epoch transition with controller disabled.
func (bs *BlockRateControllerSuite) TestOnBlockIncorporated_EpochTransition_Disabled() {
	err := bs.ctl.config.SetEnabled(false)
	require.NoError(bs.T(), err)
	bs.testOnBlockIncorporated_EpochTransition()
}

// testOnBlockIncorporated_EpochTransition tests that a view change into the next epoch
// updates the local state to reflect the new epoch.
func (bs *BlockRateControllerSuite) testOnBlockIncorporated_EpochTransition() {
	nextEpoch := mockprotocol.NewEpoch(bs.T())
	nextEpoch.On("Counter").Return(bs.epochCounter+1, nil)
	nextEpoch.On("FinalView").Return(bs.curEpochFinalView+100_000, nil)
	bs.epochs.Add(nextEpoch)
	bs.CreateAndStartController()
	defer bs.StopController()

	initialControllerState := captureControllerStateDigest(bs.ctl)
	bs.epochs.Transition()
	timedBlock := makeTimedBlock(bs.curEpochFinalView+1, unittest.IdentifierFixture(), time.Now().UTC())
	err := bs.ctl.processIncorporatedBlock(timedBlock)
	require.True(bs.T(), bs.ctl.GetProposalTiming().ObservationView() > bs.initialView)
	require.NoError(bs.T(), err)
	nextControllerState := captureControllerStateDigest(bs.ctl)

	bs.SanityCheckSubsequentMeasurements(initialControllerState, nextControllerState, false)
	// epoch boundaries should be updated
	assert.Equal(bs.T(), bs.curEpochFinalView+1, bs.ctl.epochInfo.curEpochFirstView)
	assert.Equal(bs.T(), bs.ctl.epochInfo.curEpochFinalView, bs.curEpochFinalView+100_000)
	assert.Nil(bs.T(), bs.ctl.nextEpochFinalView)
}

// TestOnEpochSetupPhaseStarted ensures that the epoch info is updated when the next epoch is set up.
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
// We should approach the min GetProposalTiming (increase view rate as much as possible)
func (bs *BlockRateControllerSuite) TestProposalDelay_AfterTargetTransitionTime() {
	// we are near the end of the epoch in view terms
	bs.initialView = uint64(float64(bs.curEpochFinalView) * .95)
	bs.CreateAndStartController()
	defer bs.StopController()

	lastProposalDelay := time.Hour // start with large dummy value
	for view := bs.initialView + 1; view < bs.ctl.curEpochFinalView; view++ {
		// we have passed the target end time of the epoch
		receivedParentBlockAt := bs.ctl.curEpochTargetEndTime.Add(time.Duration(view) * time.Second)
		timedBlock := makeTimedBlock(view, unittest.IdentifierFixture(), receivedParentBlockAt)
		err := bs.ctl.measureViewDuration(timedBlock)
		require.NoError(bs.T(), err)

		// compute proposal delay:
		pubTime := bs.ctl.GetProposalTiming().TargetPublicationTime(view+1, time.Now().UTC(), timedBlock.Block.BlockID) // simulate building a child of `timedBlock`
		delay := pubTime.Sub(receivedParentBlockAt)

		assert.LessOrEqual(bs.T(), delay, lastProposalDelay)
		lastProposalDelay = delay

		// transition views until the end of the epoch, or for 100 views
		if view-bs.initialView >= 100 {
			break
		}
	}
}

// TestProposalDelay_BehindSchedule tests the behaviour of the controller when the
// projected epoch switchover is LATER than the target switchover time, i.e.
// we are behind schedule.
// We should respond by lowering the GetProposalTiming (increasing view rate)
func (bs *BlockRateControllerSuite) TestProposalDelay_BehindSchedule() {
	// we are 50% of the way through the epoch in view terms
	bs.initialView = uint64(float64(bs.curEpochFinalView) * .5)
	bs.CreateAndStartController()
	defer bs.StopController()

	lastProposalDelay := time.Hour // start with large dummy value
	idealEnteredViewTime := bs.ctl.curEpochTargetEndTime.Add(-epochLength / 2)

	// 1s behind of schedule
	receivedParentBlockAt := idealEnteredViewTime.Add(time.Second)
	for view := bs.initialView + 1; view < bs.ctl.curEpochFinalView; view++ {
		// hold the instantaneous error constant for each view
		receivedParentBlockAt = receivedParentBlockAt.Add(bs.ctl.targetViewTime())
		timedBlock := makeTimedBlock(view, unittest.IdentifierFixture(), receivedParentBlockAt)
		err := bs.ctl.measureViewDuration(timedBlock)
		require.NoError(bs.T(), err)

		// compute proposal delay:
		pubTime := bs.ctl.GetProposalTiming().TargetPublicationTime(view+1, time.Now().UTC(), timedBlock.Block.BlockID) // simulate building a child of `timedBlock`
		delay := pubTime.Sub(receivedParentBlockAt)
		// expecting decreasing GetProposalTiming
		assert.LessOrEqual(bs.T(), delay, lastProposalDelay)
		lastProposalDelay = delay

		// transition views until the end of the epoch, or for 100 views
		if view-bs.initialView >= 100 {
			break
		}
	}
}

// TestProposalDelay_AheadOfSchedule tests the behaviour of the controller when the
// projected epoch switchover is EARLIER than the target switchover time, i.e.
// we are ahead of schedule.
// We should respond by increasing the GetProposalTiming (lowering view rate)
func (bs *BlockRateControllerSuite) TestProposalDelay_AheadOfSchedule() {
	// we are 50% of the way through the epoch in view terms
	bs.initialView = uint64(float64(bs.curEpochFinalView) * .5)
	bs.CreateAndStartController()
	defer bs.StopController()

	lastProposalDelay := time.Duration(0) // start with large dummy value
	idealEnteredViewTime := bs.ctl.curEpochTargetEndTime.Add(-epochLength / 2)
	// 1s ahead of schedule
	receivedParentBlockAt := idealEnteredViewTime.Add(-time.Second)
	for view := bs.initialView + 1; view < bs.ctl.curEpochFinalView; view++ {
		// hold the instantaneous error constant for each view
		receivedParentBlockAt = receivedParentBlockAt.Add(bs.ctl.targetViewTime())
		timedBlock := makeTimedBlock(view, unittest.IdentifierFixture(), receivedParentBlockAt)
		err := bs.ctl.measureViewDuration(timedBlock)
		require.NoError(bs.T(), err)

		// compute proposal delay:
		pubTime := bs.ctl.GetProposalTiming().TargetPublicationTime(view+1, time.Now().UTC(), timedBlock.Block.BlockID) // simulate building a child of `timedBlock`
		delay := pubTime.Sub(receivedParentBlockAt)

		// expecting increasing GetProposalTiming
		assert.GreaterOrEqual(bs.T(), delay, lastProposalDelay)
		lastProposalDelay = delay

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
	bs.metrics.On("TargetProposalDuration", time.Duration(0)).Once()
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
	bs.metrics.On("TargetProposalDuration", bs.config.MinViewDuration.Load()).Once()
	// should have a large negative controller output
	bs.metrics.On("ControllerOutput", mock.Anything).Run(func(args mock.Arguments) {
		output := args[0].(time.Duration)
		assert.Greater(bs.T(), output, time.Duration(0))
	}).Once()

	timedBlock := makeTimedBlock(view, unittest.IdentifierFixture(), enteredViewAt)
	err := bs.ctl.measureViewDuration(timedBlock)
	require.NoError(bs.T(), err)
}

// Test_vs_PythonSimulation implements a regression test. We implemented the controller in python
// together with a statistical model for the view duration. We used the python implementation to tune
// the PID controller parameters which we are using here.
// In this test, we feed values pre-generated with the python simulation into the Go implementation
// and compare the outputs to the pre-generated outputs from the python controller implementation.
func (bs *BlockRateControllerSuite) Test_vs_PythonSimulation() {
	// PART 1: setup system to mirror python simulation
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	totalEpochViews := 483000
	bs.initialView = 0
	bs.curEpochFirstView, bs.curEpochFinalView = uint64(0), uint64(totalEpochViews-1) // views [0, .., totalEpochViews-1]
	bs.epochFallbackTriggered = false

	refT := time.Now().UTC()
	refT = time.Date(refT.Year(), refT.Month(), refT.Day(), refT.Hour(), refT.Minute(), 0, 0, time.UTC) // truncate to past minute
	bs.config = &Config{
		TimingConfig: TimingConfig{
			TargetTransition:      EpochTransitionTime{day: refT.Weekday(), hour: uint8(refT.Hour()), minute: uint8(refT.Minute())},
			FallbackProposalDelay: atomic.NewDuration(500 * time.Millisecond), // irrelevant for this test, as controller should never enter fallback mode
			MinViewDuration:       atomic.NewDuration(470 * time.Millisecond),
			MaxViewDuration:       atomic.NewDuration(2010 * time.Millisecond),
			Enabled:               atomic.NewBool(true),
		},
		ControllerParams: ControllerParams{KP: 2.0, KI: 0.06, KD: 3.0, N_ewma: 5, N_itg: 50},
	}

	setupMocks(bs)
	bs.CreateAndStartController()
	defer bs.StopController()

	// PART 2: timing generated from python simulation and corresponding controller response
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	ref := struct {
		// targetViewTime is the idealized view duration of a perfect system.
		// In Python simulation, this is the array `EpochSimulation.ideal_view_time`
		targetViewTime float64 // units: seconds

		// observedMinViewTimes[i] is the minimal time required time to execute the protocol for view i
		//  - Duration from the primary observing the parent block (indexed by i) to having its child proposal (block for view i+1) ready for publication.
		//  - This is the minimal time required to execute the protocol. Nodes can only delay their proposal but not progress any faster.
		//  - in Python simulation, this is the array `EpochSimulation.min_view_times + EpochSimulation.observation_noise`
		//    with is returned by function `EpochSimulation.current_view_observation()`
		// Note that this is generally different than the time it takes the committee as a whole to transition
		// through views. This is because the primary changes from view to view, and nodes observe blocks at slightly
		// different times (small noise term). The real world (as well as the simulation) depend on collective swarm
		// behaviour of the consensus committee, which is not observable by nodes individually.
		// In contrast, our `observedMinViewTimes` here contains an additional noise term, to emulate
		// the observations of a node in the real world.
		observedMinViewTimes []float64 // units: seconds

		// controllerTargetedViewDuration[i] is the duration targeted by the Python controller :
		// - measured from observing the parent until publishing the child block for view i+1
		controllerTargetedViewDuration []float64 // units: seconds

		// realWorldViewDuration[i] is the duration of the ith view for the entire committee.
		// This value occurs in response to the controller output and is not observable by nodes individually.
		//  - in Python simulation, this is the array `EpochSimulation.real_world_view_duration`
		//    with is recorded by the environment upon the call of `EpochSimulation.delay()`
		realWorldViewDuration []float64 // units: seconds
	}{
		targetViewTime:                 1.2521739130434784,
		observedMinViewTimes:           []float64{0.813911590736, 0.709385160859, 0.737005791341, 0.837805030561, 0.822187668544, 0.812909728953, 0.783581085421, 0.741921910413, 0.712233113961, 0.726364518340, 1.248139948411, 0.874190610541, 0.708212792956, 0.817596927201, 0.804068704889, 0.816333694093, 0.635439001868, 1.056889701512, 0.828365399550, 0.864982673883, 0.724916386430, 0.657269487910, 0.879699411727, 0.825153337009, 0.838359933382, 0.756176509107, 1.423953270626, 2.384840427116, 0.699779210474, 0.678315506502, 0.739714699940, 0.756860414442, 0.822439930995, 0.863509145860, 0.629256465669, 0.639977555985, 0.755185429454, 0.749303151321, 0.791698985094, 0.858487537677, 0.573302766541, 0.819061027162, 0.666408812358, 0.685689964194, 0.823590513610, 0.767398446433, 0.751476866817, 0.714594551857, 0.807687985979, 0.689084438887, 0.778230763867, 1.003159717190, 0.805687478957, 1.189467855468, 0.775150433563, 0.659834215924, 0.719878391611, 0.723118445283, 0.729128777217, 0.894115006528, 0.821659798706, 0.707477543689, 0.788637584400, 0.802871483919, 0.647385138470, 0.824723072863, 0.826836727024, 0.777618186343, 1.287034125297, 0.902203608710, 0.860847662156, 0.744839240209, 0.703066498578, 0.734337287980, 0.850177664684, 0.794996949347, 0.703085302264, 0.850633984420, 0.852003819504, 1.215923240337, 0.950100961928, 0.706303284366, 0.767606634563, 0.805098284495, 0.746037389780, 0.753114712715, 0.827655267273, 0.677763970869, 0.775983354906, 0.886163648660, 0.827260670102, 0.674219428445, 0.827001240891, 1.079979351239, 0.834371194195, 0.642493824065, 0.831472105803, 0.868759159974, 0.768113213916, 0.799327054954},
		realWorldViewDuration:          []float64{1.302444400800, 1.346129371535, 1.294863072697, 1.247327922614, 1.286795200594, 1.306740497700, 1.287569802153, 1.255674603370, 1.221066792868, 1.274421011086, 1.310455137252, 1.490561324031, 1.253388579993, 1.308204927322, 1.303354847496, 1.258878368832, 1.252442671947, 1.300931483899, 1.292864087733, 1.285202085499, 1.275787031401, 1.272867925078, 1.313112319334, 1.250448493684, 1.280932583567, 1.275154657095, 1.982478033877, 2.950000000000, 1.303987777503, 1.197058075247, 1.271351165257, 1.218997388610, 1.289408440486, 1.314624688597, 1.248543715838, 1.257252635970, 1.313520669301, 1.289733925464, 1.255731709280, 1.329280312510, 1.250944692406, 1.244618792038, 1.270799583742, 1.297864616235, 1.281392864743, 1.274370759435, 1.267866315564, 1.269626634709, 1.235201824673, 1.249630200456, 1.252256124260, 1.308797727248, 1.299471761557, 1.718929617405, 1.264606560958, 1.241614892746, 1.274645939739, 1.267738287029, 1.264086142881, 1.317338331667, 1.243233554137, 1.242636788130, 1.222948278859, 1.278447973385, 1.301907713623, 1.315027977476, 1.299297388065, 1.297119789433, 1.794676934418, 1.325065836105, 1.345177262841, 1.263644019312, 1.256720313054, 1.345587001430, 1.312697068641, 1.272879075749, 1.297816332013, 1.296976261782, 1.287733046449, 1.833154481870, 1.462021182671, 1.255799473395, 1.246753462604, 1.311201917909, 1.248542983295, 1.289491847469, 1.283822179928, 1.275478845872, 1.276979232592, 1.333513139323, 1.279939105944, 1.252640151610, 1.304614041834, 1.538352621208, 1.318414654543, 1.258316752763, 1.278344123076, 1.323632996025, 1.295038772886, 1.249799751997},
		controllerTargetedViewDuration: []float64{1.283911590736, 1.198887195866, 1.207005791341, 1.307805030561, 1.292187668544, 1.282909728953, 1.253581085421, 1.211921910413, 1.182233113961, 1.196364518340, 1.718139948411, 1.344190610541, 1.178212792956, 1.287596927201, 1.274068704889, 1.286333694093, 1.105439001868, 1.526889701512, 1.298365399550, 1.334982673883, 1.194916386430, 1.127269487910, 1.349699411727, 1.295153337009, 1.308359933382, 1.226176509107, 1.893953270626, 2.854840427116, 1.169779210474, 1.148315506502, 1.209714699940, 1.226860414442, 1.292439930995, 1.333509145860, 1.099256465669, 1.109977555985, 1.225185429454, 1.219303151321, 1.261698985094, 1.328487537677, 1.043302766541, 1.289061027162, 1.136408812358, 1.155689964194, 1.293590513610, 1.237398446433, 1.221476866817, 1.184594551857, 1.277687985979, 1.159084438887, 1.248230763867, 1.473159717190, 1.275687478957, 1.659467855468, 1.245150433563, 1.129834215924, 1.189878391611, 1.193118445283, 1.199128777217, 1.364115006528, 1.291659798706, 1.177477543689, 1.258637584400, 1.272871483919, 1.117385138470, 1.294723072863, 1.296836727024, 1.247618186343, 1.757034125297, 1.372203608710, 1.330847662156, 1.214839240209, 1.173066498578, 1.204337287980, 1.320177664684, 1.264996949347, 1.173085302264, 1.320633984420, 1.322003819504, 1.685923240337, 1.420100961928, 1.176303284366, 1.237606634563, 1.275098284495, 1.216037389780, 1.223114712715, 1.297655267273, 1.147763970869, 1.245983354906, 1.356163648660, 1.297260670102, 1.144219428445, 1.297001240891, 1.549979351239, 1.304371194195, 1.112493824065, 1.301472105803, 1.338759159974, 1.238113213916, 1.269327054954},
	}

	// PART 3: run controller and ensure output matches pre-generated controller response from python ref implementation
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// sanity checks:
	require.Equal(bs.T(), 604800.0, bs.ctl.curEpochTargetEndTime.UTC().Sub(refT).Seconds(), "Epoch should end 1 week from now, i.e. 604800s")
	require.InEpsilon(bs.T(), ref.targetViewTime, bs.ctl.targetViewTime().Seconds(), 1e-10) // ideal view time
	require.Equal(bs.T(), len(ref.observedMinViewTimes), len(ref.realWorldViewDuration))

	// Notes:
	// - We specifically make the first observation at when the full time of the epoch is left.
	//   The python simulation we compare with proceed exactly the same way.
	// - we first make an observation, before requesting the controller output. Thereby, we
	//   avoid artifacts of recalling a controller that was just initialized with fallback values.
	// - we call `measureViewDuration(..)` (_not_ `processIncorporatedBlock(..)`) to
	//   interfering with the deduplication logic. Here we want to test correct numerics.
	//   Correctness of the deduplication logic is verified in the different test.
	observationTime := refT

	for v := 0; v < len(ref.observedMinViewTimes); v++ {
		observedBlock := makeTimedBlock(uint64(v), unittest.IdentifierFixture(), observationTime)
		err := bs.ctl.measureViewDuration(observedBlock)
		require.NoError(bs.T(), err)
		proposalTiming := bs.ctl.GetProposalTiming()
		tpt := proposalTiming.TargetPublicationTime(uint64(v+1), time.Now(), observedBlock.Block.BlockID) // value for `timeViewEntered` should be irrelevant here
		controllerTargetedViewDuration := tpt.Sub(observedBlock.TimeObserved).Seconds()
		require.InEpsilon(bs.T(), ref.controllerTargetedViewDuration[v], controllerTargetedViewDuration, 1e-5, "implementations deviate for view %d", v) // ideal view time

		observationTime = observationTime.Add(time.Duration(int64(ref.realWorldViewDuration[v] * float64(time.Second))))
	}

}

func makeTimedBlock(view uint64, parentID flow.Identifier, time time.Time) TimedBlock {
	header := unittest.BlockHeaderFixture(unittest.HeaderWithView(view))
	header.ParentID = parentID
	return TimedBlock{
		Block:        model.BlockFromFlow(header),
		TimeObserved: time,
	}
}

type controllerStateDigest struct {
	proportionalErr Ewma
	integralErr     LeakyIntegrator

	// latestProposalTiming holds the ProposalTiming that the controller generated in response to processing the latest observation
	latestProposalTiming ProposalTiming
}

func captureControllerStateDigest(ctl *BlockTimeController) *controllerStateDigest {
	return &controllerStateDigest{
		proportionalErr:      ctl.proportionalErr,
		integralErr:          ctl.integralErr,
		latestProposalTiming: ctl.GetProposalTiming(),
	}
}
