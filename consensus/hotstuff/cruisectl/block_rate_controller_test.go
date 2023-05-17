package cruisectl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

type BlockRateControllerSuite struct {
	suite.Suite

	initialView       uint64
	epochCounter      uint64
	curEpochFirstView uint64
	curEpochFinalView uint64

	state    *mockprotocol.State
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
	bs.initialView = 0
	bs.epochCounter = uint64(0)
	bs.curEpochFirstView = uint64(0)
	bs.curEpochFinalView = uint64(100_000)

	bs.state = mockprotocol.NewState(bs.T())
	bs.snapshot = mockprotocol.NewSnapshot(bs.T())
	bs.epochs = mocks.NewEpochQuery(bs.T(), bs.epochCounter)
	bs.curEpoch = mockprotocol.NewEpoch(bs.T())

	bs.state.On("Final").Return(bs.snapshot)
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

func (bs *BlockRateControllerSuite) AssertCorrectInitialization() {
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

// TestStartStop tests that the component can be started and stopped gracefully.
func (bs *BlockRateControllerSuite) TestStartStop() {
	bs.CreateAndStartController()
	bs.StopController()
}

func (bs *BlockRateControllerSuite) TestInit_EpochStakingPhase() {
	bs.CreateAndStartController()
	defer bs.StopController()
	bs.AssertCorrectInitialization()
}

func (bs *BlockRateControllerSuite) TestInit_EpochSetupPhase() {
	nextEpoch := mockprotocol.NewEpoch(bs.T())
	nextEpoch.On("Counter").Return(bs.epochCounter+1, nil)
	nextEpoch.On("FinalView").Return(bs.curEpochFinalView+100_000, nil)
	bs.epochs.Add(nextEpoch)

	bs.CreateAndStartController()
	defer bs.StopController()
	bs.AssertCorrectInitialization()
}

//func (bs *BlockRateControllerSuite) TestEpochFallbackTriggered() {}

// test - epoch fallback triggered
//  - twice
//  - revert to default block rate

//func (bs *BlockRateControllerSuite) TestOnViewChange() {}

// test - new view
//  - epoch transition
//  - measurement is updated
//  - duplicate events are handled

//func (bs *BlockRateControllerSuite) TestOnEpochSetupPhaseStarted() {}

// test - epochsetup
//  - epoch info is updated
//  - duplicate events are handled
