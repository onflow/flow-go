package cruisectl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
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

func (bs *BlockRateControllerSuite) SetupTest() {
	bs.config = DefaultConfig()
	bs.config.KP = 1.0
	bs.config.KI = 1.0
	bs.config.KD = 1.0
	bs.initialView = 0
	bs.epochCounter = uint64(0)
	bs.curEpochFirstView = uint64(0)
	bs.curEpochFinalView = uint64(100_000)
	bs.epochFallbackTriggered = false

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

func (bs *BlockRateControllerSuite) CreateAndStartController() {
	ctl, err := NewBlockRateController(unittest.Logger(), bs.config, bs.state, bs.initialView)
	require.NoError(bs.T(), err)
	bs.ctl = ctl
	bs.ctl.Start(bs.ctx)
	unittest.RequireCloseBefore(bs.T(), bs.ctl.Ready(), time.Second, "component did not start")
}

func (bs *BlockRateControllerSuite) StopController() {
	bs.cancel()
	unittest.RequireCloseBefore(bs.T(), bs.ctl.Done(), time.Second, "component did not stop")
}

// AssertCorrectInitialization checks that the controller is configured as expected after construction.
func (bs *BlockRateControllerSuite) AssertCorrectInitialization() {
	// proposal delay should be initialized to default value
	assert.Equal(bs.T(), bs.config.DefaultProposalDelayMs(), bs.ctl.ProposalDelay())

	// if epoch fallback is triggered, we don't care about anything else
	if bs.ctl.epochFallbackTriggered.Load() {
		return
	}

	// should initialize epoch info
	epoch := bs.ctl.epochInfo
	expectedEndTime := bs.config.TargetTransition.inferTargetEndTime(bs.initialView, time.Now(), epoch)
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
	// measured view rates should be set to the target as an initial target
	assert.Equal(bs.T(), lastMeasurement.targetViewRate, lastMeasurement.viewRate)
	assert.Equal(bs.T(), lastMeasurement.targetViewRate, lastMeasurement.aveViewRate)
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
	// new measurement should observe a different view rate
	assert.NotEqual(bs.T(), m1.viewRate, m2.viewRate)
	assert.NotEqual(bs.T(), m1.aveViewRate, m2.aveViewRate)
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
// Default proposal delay should be set.
func (bs *BlockRateControllerSuite) TestInit_EpochFallbackTriggered() {
	bs.epochFallbackTriggered = true
	bs.CreateAndStartController()
	defer bs.StopController()
	bs.AssertCorrectInitialization()
}

// TestEpochFallbackTriggered tests epoch fallback:
//   - the proposal delay should revert to default
//   - duplicate events should be no-ops
func (bs *BlockRateControllerSuite) TestEpochFallbackTriggered() {
	bs.CreateAndStartController()
	defer bs.StopController()

	// update error so that proposal delay is non-default
	bs.ctl.lastMeasurement.aveViewRate *= 1.1
	err := bs.ctl.measureViewRate(bs.initialView+1, time.Now())
	require.NoError(bs.T(), err)
	assert.NotEqual(bs.T(), bs.config.DefaultProposalDelayMs(), bs.ctl.ProposalDelay())

	// send the event
	bs.ctl.EpochEmergencyFallbackTriggered()
	// async: should revert to default proposal delay
	require.Eventually(bs.T(), func() bool {
		return bs.config.DefaultProposalDelayMs() == bs.ctl.ProposalDelay()
	}, time.Second, time.Millisecond)

	// additional events should be no-ops
	// (send capacity+1 events to guarantee one is processed)
	for i := 0; i <= cap(bs.ctl.epochFallbacks); i++ {
		bs.ctl.EpochEmergencyFallbackTriggered()
	}
	assert.Equal(bs.T(), bs.config.DefaultProposalDelayMs(), bs.ctl.ProposalDelay())
}

// TestOnViewChange_UpdateProposalDelay tests that a new measurement is taken and
// proposal delay updated upon receiving an OnViewChange event.
func (bs *BlockRateControllerSuite) TestOnViewChange_UpdateProposalDelay() {
	bs.CreateAndStartController()
	defer bs.StopController()

	initialMeasurement := bs.ctl.lastMeasurement
	initialProposalDelay := bs.ctl.ProposalDelay()
	bs.ctl.OnViewChange(0, bs.initialView+1)
	require.Eventually(bs.T(), func() bool {
		return bs.ctl.lastMeasurement.view > bs.initialView
	}, time.Second, time.Millisecond)
	nextMeasurement := bs.ctl.lastMeasurement
	nextProposalDelay := bs.ctl.ProposalDelay()

	bs.SanityCheckSubsequentMeasurements(initialMeasurement, nextMeasurement)
	// new measurement should update proposal delay
	assert.NotEqual(bs.T(), initialProposalDelay, nextProposalDelay)

	// duplicate events should be no-ops
	for i := 0; i <= cap(bs.ctl.viewChanges); i++ {
		bs.ctl.OnViewChange(0, bs.initialView+1)
	}
	require.Eventually(bs.T(), func() bool {
		return len(bs.ctl.viewChanges) == 0
	}, time.Second, time.Millisecond)

	// state should be unchanged
	assert.Equal(bs.T(), nextMeasurement, bs.ctl.lastMeasurement)
	assert.Equal(bs.T(), nextProposalDelay, bs.ctl.ProposalDelay())
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
