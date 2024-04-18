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

// BlockTimeControllerSuite encapsulates tests for the BlockTimeController.
type BlockTimeControllerSuite struct {
	suite.Suite

	initialView            uint64
	epochCounter           uint64
	curEpochFirstView      uint64
	curEpochFinalView      uint64
	curEpochTargetDuration uint64
	curEpochTargetEndTime  uint64
	epochFallbackTriggered bool

	metrics  mockmodule.CruiseCtlMetrics
	state    mockprotocol.State
	params   mockprotocol.Params
	snapshot mockprotocol.Snapshot
	epochs   mocks.EpochQuery
	curEpoch mockprotocol.Epoch

	config *Config
	ctx    irrecoverable.SignalerContext
	cancel context.CancelFunc
	ctl    *BlockTimeController
}

func TestBlockTimeController(t *testing.T) {
	suite.Run(t, new(BlockTimeControllerSuite))
}

// EpochDurationSeconds returns the number of seconds in the epoch (1hr).
func (bs *BlockTimeControllerSuite) EpochDurationSeconds() uint64 {
	return 60 * 60
}

// SetupTest initializes mocks and default values.
func (bs *BlockTimeControllerSuite) SetupTest() {
	bs.config = DefaultConfig()
	bs.config.Enabled.Store(true)
	bs.initialView = 0
	bs.epochCounter = uint64(0)
	bs.curEpochFirstView = uint64(0)
	bs.curEpochFinalView = bs.EpochDurationSeconds() // 1 view/sec for 1hr epoch
	bs.curEpochTargetDuration = bs.EpochDurationSeconds()
	bs.curEpochTargetEndTime = uint64(time.Now().Unix()) + bs.EpochDurationSeconds()
	bs.epochFallbackTriggered = false
	setupMocks(bs)
}

func setupMocks(bs *BlockTimeControllerSuite) {
	bs.metrics = *mockmodule.NewCruiseCtlMetrics(bs.T())
	bs.metrics.On("PIDError", mock.Anything, mock.Anything, mock.Anything).Maybe()
	bs.metrics.On("TargetProposalDuration", mock.Anything).Maybe()
	bs.metrics.On("ControllerOutput", mock.Anything).Maybe()

	bs.state = *mockprotocol.NewState(bs.T())
	bs.params = *mockprotocol.NewParams(bs.T())
	bs.snapshot = *mockprotocol.NewSnapshot(bs.T())
	bs.epochs = *mocks.NewEpochQuery(bs.T(), bs.epochCounter)
	bs.curEpoch = *mockprotocol.NewEpoch(bs.T())

	bs.state.On("Final").Return(&bs.snapshot)
	bs.state.On("AtHeight", mock.Anything).Return(&bs.snapshot).Maybe()
	bs.state.On("Params").Return(&bs.params)
	bs.params.On("EpochFallbackTriggered").Return(
		func() bool { return bs.epochFallbackTriggered },
		func() error { return nil })
	bs.snapshot.On("Phase").Return(
		func() flow.EpochPhase { return bs.epochs.Phase() },
		func() error { return nil })
	bs.snapshot.On("Head").Return(unittest.BlockHeaderFixture(unittest.HeaderWithView(bs.initialView+11)), nil).Maybe()
	bs.snapshot.On("Epochs").Return(&bs.epochs)
	bs.curEpoch.On("Counter").Return(bs.epochCounter, nil)
	bs.curEpoch.On("FirstView").Return(bs.curEpochFirstView, nil)
	bs.curEpoch.On("FinalView").Return(bs.curEpochFinalView, nil)
	bs.curEpoch.On("TargetDuration").Return(bs.curEpochTargetDuration, nil)
	bs.curEpoch.On("TargetEndTime").Return(bs.curEpochTargetEndTime, nil)
	bs.epochs.Add(&bs.curEpoch)

	bs.ctx, bs.cancel = irrecoverable.NewMockSignalerContextWithCancel(bs.T(), context.Background())
}

// CreateAndStartController creates and starts the BlockTimeController.
// Should be called only once per test case.
func (bs *BlockTimeControllerSuite) CreateAndStartController() {
	ctl, err := NewBlockTimeController(unittest.Logger(), &bs.metrics, bs.config, &bs.state, bs.initialView)
	require.NoError(bs.T(), err)
	bs.ctl = ctl
	bs.ctl.Start(bs.ctx)
	unittest.RequireCloseBefore(bs.T(), bs.ctl.Ready(), time.Second, "component did not start")
}

// StopController stops the BlockTimeController.
func (bs *BlockTimeControllerSuite) StopController() {
	bs.cancel()
	unittest.RequireCloseBefore(bs.T(), bs.ctl.Done(), time.Second, "component did not stop")
}

// AssertCorrectInitialization checks that the controller is configured as expected after construction.
func (bs *BlockTimeControllerSuite) AssertCorrectInitialization() {
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
	assert.Equal(bs.T(), bs.curEpochFirstView, epoch.curEpochFirstView)
	assert.Equal(bs.T(), bs.curEpochFinalView, epoch.curEpochFinalView)
	assert.Equal(bs.T(), bs.curEpochTargetDuration, epoch.curEpochTargetDuration)
	assert.Equal(bs.T(), bs.curEpochTargetEndTime, epoch.curEpochTargetEndTime)

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
func (bs *BlockTimeControllerSuite) SanityCheckSubsequentMeasurements(d1, d2 *controllerStateDigest, expectedEqual bool) {
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
func (bs *BlockTimeControllerSuite) PrintMeasurement(parentBlockId flow.Identifier) {
	ctl := bs.ctl
	m := ctl.GetProposalTiming()
	tpt := m.TargetPublicationTime(m.ObservationView()+1, m.ObservationTime(), parentBlockId)
	fmt.Printf("v=%d\tt=%s\tPD=%s\te_N=%.3f\tI_M=%.3f\n",
		m.ObservationView(), m.ObservationTime(), tpt.Sub(m.ObservationTime()),
		ctl.proportionalErr.Value(), ctl.integralErr.Value())
}

// TestStartStop tests that the component can be started and stopped gracefully.
func (bs *BlockTimeControllerSuite) TestStartStop() {
	bs.CreateAndStartController()
	bs.StopController()
}

// TestInit_EpochStakingPhase tests initializing the component in the EpochStaking phase.
// Measurement and epoch info should be initialized, next epoch final view should be nil.
func (bs *BlockTimeControllerSuite) TestInit_EpochStakingPhase() {
	bs.CreateAndStartController()
	defer bs.StopController()
	bs.AssertCorrectInitialization()
}

// TestInit_EpochStakingPhase tests initializing the component in the EpochSetup phase.
// Measurement and epoch info should be initialized, next epoch final view should be set.
func (bs *BlockTimeControllerSuite) TestInit_EpochSetupPhase() {
	nextEpoch := mockprotocol.NewEpoch(bs.T())
	nextEpoch.On("Counter").Return(bs.epochCounter+1, nil)
	nextEpoch.On("FinalView").Return(bs.curEpochFinalView*2, nil)
	nextEpoch.On("TargetDuration").Return(bs.EpochDurationSeconds(), nil)
	nextEpoch.On("TargetEndTime").Return(bs.curEpochTargetEndTime+bs.EpochDurationSeconds(), nil)
	bs.epochs.Add(nextEpoch)

	bs.CreateAndStartController()
	defer bs.StopController()
	bs.AssertCorrectInitialization()
}

// TestInit_EpochFallbackTriggered tests initializing the component when epoch fallback is triggered.
// Default GetProposalTiming should be set.
func (bs *BlockTimeControllerSuite) TestInit_EpochFallbackTriggered() {
	bs.epochFallbackTriggered = true
	bs.CreateAndStartController()
	defer bs.StopController()
	bs.AssertCorrectInitialization()
}

// TestEpochFallbackTriggered tests epoch fallback:
//   - the GetProposalTiming should revert to default
//   - duplicate events should be no-ops
func (bs *BlockTimeControllerSuite) TestEpochFallbackTriggered() {
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
func (bs *BlockTimeControllerSuite) TestOnBlockIncorporated_UpdateProposalDelay() {
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
func (bs *BlockTimeControllerSuite) TestEnableDisable() {
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
func (bs *BlockTimeControllerSuite) TestOnBlockIncorporated_EpochTransition_Enabled() {
	err := bs.ctl.config.SetEnabled(true)
	require.NoError(bs.T(), err)
	bs.testOnBlockIncorporated_EpochTransition()
}

// TestOnBlockIncorporated_EpochTransition_Disabled tests epoch transition with controller disabled.
func (bs *BlockTimeControllerSuite) TestOnBlockIncorporated_EpochTransition_Disabled() {
	err := bs.ctl.config.SetEnabled(false)
	require.NoError(bs.T(), err)
	bs.testOnBlockIncorporated_EpochTransition()
}

// testOnBlockIncorporated_EpochTransition tests that a view change into the next epoch
// updates the local state to reflect the new epoch.
func (bs *BlockTimeControllerSuite) testOnBlockIncorporated_EpochTransition() {
	nextEpoch := mockprotocol.NewEpoch(bs.T())
	nextEpoch.On("Counter").Return(bs.epochCounter+1, nil)
	nextEpoch.On("FinalView").Return(bs.curEpochFinalView*2, nil)
	nextEpoch.On("TargetDuration").Return(bs.EpochDurationSeconds(), nil) // 1s/view
	nextEpoch.On("TargetEndTime").Return(bs.curEpochTargetEndTime+bs.EpochDurationSeconds(), nil)
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
	assert.Equal(bs.T(), bs.ctl.epochInfo.curEpochFinalView, bs.curEpochFinalView*2)
	assert.Equal(bs.T(), bs.ctl.epochInfo.curEpochTargetEndTime, bs.curEpochTargetEndTime+bs.EpochDurationSeconds())
	assert.Nil(bs.T(), bs.ctl.nextEpochFinalView)
}

// TestOnEpochSetupPhaseStarted ensures that the epoch info is updated when the next epoch is set up.
func (bs *BlockTimeControllerSuite) TestOnEpochSetupPhaseStarted() {
	nextEpoch := mockprotocol.NewEpoch(bs.T())
	nextEpoch.On("Counter").Return(bs.epochCounter+1, nil)
	nextEpoch.On("FinalView").Return(bs.curEpochFinalView*2, nil)
	nextEpoch.On("TargetDuration").Return(bs.EpochDurationSeconds(), nil)
	nextEpoch.On("TargetEndTime").Return(bs.curEpochTargetEndTime+bs.EpochDurationSeconds(), nil)
	bs.epochs.Add(nextEpoch)
	bs.CreateAndStartController()
	defer bs.StopController()

	header := unittest.BlockHeaderFixture()
	bs.ctl.EpochSetupPhaseStarted(bs.epochCounter, header)
	require.Eventually(bs.T(), func() bool {
		return bs.ctl.nextEpochFinalView != nil
	}, time.Second, time.Millisecond)

	assert.Equal(bs.T(), bs.curEpochFinalView*2, *bs.ctl.nextEpochFinalView)
	assert.Equal(bs.T(), bs.curEpochTargetEndTime+bs.EpochDurationSeconds(), *bs.ctl.nextEpochTargetEndTime)

	// duplicate events should be no-ops
	for i := 0; i <= cap(bs.ctl.epochSetups); i++ {
		bs.ctl.EpochSetupPhaseStarted(bs.epochCounter, header)
	}
	assert.Equal(bs.T(), bs.curEpochFinalView*2, *bs.ctl.nextEpochFinalView)
	assert.Equal(bs.T(), bs.curEpochTargetEndTime+bs.EpochDurationSeconds(), *bs.ctl.nextEpochTargetEndTime)
}

// TestProposalDelay_AfterTargetTransitionTime tests the behaviour of the controller
// when we have passed the target end time for the current epoch.
// We should approach the min GetProposalTiming (increase view rate as much as possible)
func (bs *BlockTimeControllerSuite) TestProposalDelay_AfterTargetTransitionTime() {
	// we are near the end of the epoch in view terms
	bs.initialView = uint64(float64(bs.curEpochFinalView) * .95)
	bs.CreateAndStartController()
	defer bs.StopController()

	lastProposalDelay := float64(bs.EpochDurationSeconds()) // start with large dummy value
	for view := bs.initialView + 1; view < bs.ctl.curEpochFinalView; view++ {
		// we have passed the target end time of the epoch
		receivedParentBlockAt := unix2time(bs.ctl.curEpochTargetEndTime + view)
		timedBlock := makeTimedBlock(view, unittest.IdentifierFixture(), receivedParentBlockAt)
		err := bs.ctl.measureViewDuration(timedBlock)
		require.NoError(bs.T(), err)

		// compute proposal delay:
		pubTime := bs.ctl.GetProposalTiming().TargetPublicationTime(view+1, time.Now().UTC(), timedBlock.Block.BlockID) // simulate building a child of `timedBlock`
		delay := pubTime.Sub(receivedParentBlockAt)

		assert.LessOrEqual(bs.T(), delay.Seconds(), lastProposalDelay)
		lastProposalDelay = delay.Seconds()

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
func (bs *BlockTimeControllerSuite) TestProposalDelay_BehindSchedule() {
	// we are 50% of the way through the epoch in view terms
	bs.initialView = uint64(float64(bs.curEpochFinalView) * .5)
	bs.CreateAndStartController()
	defer bs.StopController()

	lastProposalDelay := float64(bs.EpochDurationSeconds()) // start with large dummy value
	idealEnteredViewTime := unix2time(bs.ctl.curEpochTargetEndTime - (bs.EpochDurationSeconds() / 2))

	// 1s behind of schedule
	receivedParentBlockAt := idealEnteredViewTime.Add(time.Second)
	for view := bs.initialView + 1; view < bs.ctl.curEpochFinalView; view++ {
		// hold the instantaneous error constant for each view
		receivedParentBlockAt = receivedParentBlockAt.Add(sec2dur(bs.ctl.targetViewTime()))
		timedBlock := makeTimedBlock(view, unittest.IdentifierFixture(), receivedParentBlockAt)
		err := bs.ctl.measureViewDuration(timedBlock)
		require.NoError(bs.T(), err)

		// compute proposal delay:
		pubTime := bs.ctl.GetProposalTiming().TargetPublicationTime(view+1, time.Now().UTC(), timedBlock.Block.BlockID) // simulate building a child of `timedBlock`
		delay := pubTime.Sub(receivedParentBlockAt)
		// expecting decreasing GetProposalTiming
		assert.LessOrEqual(bs.T(), delay.Seconds(), lastProposalDelay, "got non-decreasing delay on view %d (initial view: %d)", view, bs.initialView)
		lastProposalDelay = delay.Seconds()

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
func (bs *BlockTimeControllerSuite) TestProposalDelay_AheadOfSchedule() {
	// we are 50% of the way through the epoch in view terms
	bs.initialView = uint64(float64(bs.curEpochFinalView) * .5)
	bs.CreateAndStartController()
	defer bs.StopController()

	lastProposalDelay := time.Duration(0) // start with large dummy value
	idealEnteredViewTime := bs.ctl.curEpochTargetEndTime - (bs.EpochDurationSeconds() / 2)
	// 1s ahead of schedule
	receivedParentBlockAt := idealEnteredViewTime - 1
	for view := bs.initialView + 1; view < bs.ctl.curEpochFinalView; view++ {
		// hold the instantaneous error constant for each view
		receivedParentBlockAt = receivedParentBlockAt + uint64(bs.ctl.targetViewTime())
		timedBlock := makeTimedBlock(view, unittest.IdentifierFixture(), unix2time(receivedParentBlockAt))
		err := bs.ctl.measureViewDuration(timedBlock)
		require.NoError(bs.T(), err)

		// compute proposal delay:
		pubTime := bs.ctl.GetProposalTiming().TargetPublicationTime(view+1, time.Now().UTC(), timedBlock.Block.BlockID) // simulate building a child of `timedBlock`
		delay := pubTime.Sub(unix2time(receivedParentBlockAt))

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
func (bs *BlockTimeControllerSuite) TestMetrics() {
	bs.metrics = *mockmodule.NewCruiseCtlMetrics(bs.T())
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

	timedBlock := makeTimedBlock(view, unittest.IdentifierFixture(), unix2time(enteredViewAt))
	err := bs.ctl.measureViewDuration(timedBlock)
	require.NoError(bs.T(), err)
}

// Test_vs_PythonSimulation performs a regression test. We implemented the controller in python
// together with a statistical model for the view duration. We used the python implementation to tune
// the PID controller parameters which we are using here.
// In this test, we feed values pre-generated with the python simulation into the Go implementation
// and compare the outputs to the pre-generated outputs from the python controller implementation.
func (bs *BlockTimeControllerSuite) Test_vs_PythonSimulation() {
	// PART 1: setup system to mirror python simulation
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	refT := time.Now().UTC()
	refT = time.Date(refT.Year(), refT.Month(), refT.Day(), refT.Hour(), refT.Minute(), 0, 0, time.UTC) // truncate to past minute

	totalEpochViews := 483000
	bs.initialView = 0
	bs.curEpochFirstView, bs.curEpochFinalView = uint64(0), uint64(totalEpochViews-1) // views [0, .., totalEpochViews-1]
	bs.curEpochTargetDuration = 7 * 24 * 60 * 60                                      // 1 week in seconds
	bs.curEpochTargetEndTime = time2unix(refT) + bs.curEpochTargetDuration            // now + 1 week
	bs.epochFallbackTriggered = false

	bs.config = &Config{
		TimingConfig: TimingConfig{
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
		observedMinViewTimes:           []float64{0.8139115907362099, 0.7093851608587579, 0.7370057913407495, 0.8378050305605419, 0.8221876685439506, 0.8129097289534515, 0.7835810854212116, 0.7419219104134447, 0.7122331139614623, 0.7263645183403751, 1.2481399484109290, 0.8741906105412369, 0.7082127929564489, 0.8175969272012624, 0.8040687048886446, 0.8163336940928989, 0.6354390018677689, 1.0568897015119771, 0.8283653995502240, 0.8649826738831023, 0.7249163864295024, 0.6572694879104934, 0.8796994117267707, 0.8251533370085626, 0.8383599333817994, 0.7561765091071196, 1.4239532706257330, 2.3848404271162811, 0.6997792104740760, 0.6783155065018911, 0.7397146999404549, 0.7568604144415827, 0.8224399309953295, 0.8635091458596464, 0.6292564656694590, 0.6399775559845721, 0.7551854294536755, 0.7493031513209824, 0.7916989850940226, 0.8584875376770561, 0.5733027665412744, 0.8190610271623866, 0.6664088123579012, 0.6856899641942998, 0.8235905136098289, 0.7673984464333541, 0.7514768668170753, 0.7145945518569533, 0.8076879859786521, 0.6890844388873341, 0.7782307638665685, 1.0031597171903470, 0.8056874789572074, 1.1894678554682030, 0.7751504335630999, 0.6598342159237116, 0.7198783916113262, 0.7231184452829420, 0.7291287772166142, 0.8941150065282033, 0.8216597987064465, 0.7074775436893693, 0.7886375844003763, 0.8028714839193359, 0.6473851384702657, 0.8247230728633490, 0.8268367270238434, 0.7776181863431995, 1.2870341252966155, 0.9022036087098005, 0.8608476621564736, 0.7448392402085238, 0.7030664985775897, 0.7343372879803260, 0.8501776646839836, 0.7949969493471933, 0.7030853022640485, 0.8506339844198412, 0.8520038195041865, 1.2159232403369129, 0.9501009619276108, 0.7063032843664507, 0.7676066345629766, 0.8050982844953996, 0.7460373897798731, 0.7531147127154058, 0.8276552672727131, 0.6777639708691676, 0.7759833549063068, 0.8861636486602165, 0.8272606701022402, 0.6742194284453155, 0.8270012408910985, 1.0799793512385585, 0.8343711941947437, 0.6424938240651709, 0.8314721058034046, 0.8687591599744876, 0.7681132139163648, 0.7993270549538212},
		realWorldViewDuration:          []float64{1.2707067231074189, 1.3797713099533957, 1.1803368837187869, 1.0710943548975358, 1.3055277182347431, 1.3142312827952587, 1.2748087784689972, 1.2580713757160862, 1.2389594986278398, 1.2839951451881206, 0.8404551372521588, 1.7402295383244093, 1.2486807727203340, 1.1529076722170450, 1.2303564416007062, 1.1919067015405667, 1.4317417513319299, 0.8851802701506968, 1.4621618954558588, 1.2629599000198048, 1.3845528649513363, 1.3083813148510797, 1.0320875660949032, 1.2138806234836066, 1.2922205615230111, 1.3530469860253094, 1.5124780338765653, 2.4800000000000000, 0.8339877775027843, 0.7270580752471872, 0.8013511652567021, 0.7489973886099706, 0.9647668631144197, 1.4406086304771719, 1.6376005221775904, 1.3686144679115566, 1.2051140074616571, 1.2232170397428770, 1.1785015757024468, 1.2720488631325702, 1.4845607775546621, 1.0038608184511295, 1.4011693227324362, 1.2782420466946043, 1.0808595015305793, 1.2923716723984215, 1.2876404222029678, 1.3024029638718018, 1.1243308902566644, 1.3825311808461356, 1.1826028495527394, 1.0753560400260920, 1.4587594729770430, 1.3281281084314180, 1.1987898717701806, 1.3212567274973721, 1.2131355949220173, 1.2202213287069972, 1.2345177139086974, 1.1415707241388824, 1.2618615652263814, 1.3978228798726429, 1.1676202853133009, 1.2821402577607839, 1.4378331263208257, 1.0764974304705950, 1.1968636840861584, 1.3079197545950789, 1.3246769344178762, 1.0956265919521080, 1.3056225547363036, 1.3094504040915045, 1.2916519124885637, 1.2995343661957905, 1.0839793112463321, 1.2515453598485311, 1.3907042923175941, 1.1137329234266407, 1.2293962485228747, 1.4537855131563087, 1.1564260868809058, 1.2616419368628695, 1.1777963280146100, 1.2782540498222059, 1.2617698479511545, 1.2911000941099631, 1.1719344274281953, 1.3904853415093545, 1.1612440756337188, 1.1800751870755894, 1.2653752924717137, 1.3987404424771417, 1.1573292016433725, 1.2132227320045601, 1.2835627159341194, 1.3950341330597937, 1.0774862045842490, 1.2361956384863142, 1.3415505497959577, 1.1881870996394799},
		controllerTargetedViewDuration: []float64{1.2521739130434784, 1.2325291342837938, 1.0924796023620962, 1.1315714628442570, 1.3109201861848649, 1.2904005140483341, 1.2408200617376139, 1.2143186827596988, 1.2001258197216824, 1.2059386524427240, 1.1687014183641575, 1.5938588248347272, 1.1735049856838198, 1.1322996720968055, 1.2010702989934061, 1.2193620268012733, 1.2847380812524840, 1.1111384877632171, 1.4676632072726421, 1.3127404884038874, 1.3036822199799039, 1.1627828776831781, 1.0686746584877680, 1.2585854668086294, 1.3196479113378341, 1.3040688380370420, 1.2092520716891777, 0.9174437864843878, 0.4700000000000000, 0.4700000000000000, 0.4700000000000000, 0.4700000000000000, 0.9677983536241768, 1.4594930877396231, 1.4883132720086421, 1.2213393879261234, 1.1167787676139602, 1.1527862655996910, 1.1844688515164143, 1.2712560882996764, 1.2769188516898307, 1.0483030535756364, 1.2667785513482170, 1.1360673946540731, 1.0930571503977162, 1.2553993593963664, 1.2412509734564154, 1.2173708810202102, 1.1668170515618597, 1.2919854192770974, 1.1785774891590928, 1.2397180299682444, 1.4349751903776191, 1.2686663464941463, 1.1793337443757632, 1.2094760506747269, 1.1283680467942478, 1.1456014869605273, 1.1695603482439110, 1.1883473989997737, 1.3102878097954334, 1.3326636354319201, 1.2033095908546276, 1.2765637682955560, 1.2533105511679674, 1.0561925258579383, 1.1944030230453759, 1.2584181515051163, 1.2181701773236133, 1.1427643645565180, 1.2912929540520488, 1.2606456249879283, 1.2079980980125691, 1.1582846527456185, 1.0914599072895725, 1.2436632334468321, 1.2659732625682767, 1.1373906460646186, 1.2636670215783354, 1.3065542716228340, 1.1145058661373550, 1.1821457478344533, 1.1686494999739092, 1.2421504164081945, 1.2292642544361261, 1.2247229593559099, 1.1857675147732030, 1.2627704665069508, 1.1302481979483210, 1.2027256964130453, 1.2826968566299934, 1.2903197193121982, 1.1497164007008540, 1.2248494620352162, 1.2695192555858241, 1.2492112043621006, 1.1006141873118667, 1.2513218024356318, 1.2846249908259910, 1.2077144025965167},
	}

	// PART 3: run controller and ensure output matches pre-generated controller response from python ref implementation
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// sanity checks:
	require.Equal(bs.T(), uint64(604800), bs.ctl.curEpochTargetEndTime-time2unix(refT), "Epoch should end 1 week from now, i.e. 604800s")
	require.InEpsilon(bs.T(), ref.targetViewTime, bs.ctl.targetViewTime(), 1e-15) // ideal view time
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
		bs.T().Logf("%d: ctl=%f\tref=%f\tdiff=%f", v, controllerTargetedViewDuration, ref.controllerTargetedViewDuration[v], controllerTargetedViewDuration-ref.controllerTargetedViewDuration[v])
		require.InEpsilon(bs.T(), ref.controllerTargetedViewDuration[v], controllerTargetedViewDuration, 1e-5, "implementations deviate for view %d", v) // ideal view time

		observationTime = observationTime.Add(sec2dur(ref.realWorldViewDuration[v]))
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
