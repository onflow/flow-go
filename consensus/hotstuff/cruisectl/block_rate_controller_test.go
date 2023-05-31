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
