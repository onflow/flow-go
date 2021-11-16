package dkg_test

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine/consensus/dkg"
	dkgmodel "github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/flow"
	dkgmodule "github.com/onflow/flow-go/module/dkg"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// ReactorSuite is the test suite for the reactor engine.
type ReactorSuite struct {
	suite.Suite

	// config
	dkgStartView       uint64
	dkgPhase1FinalView uint64
	dkgPhase2FinalView uint64
	dkgPhase3FinalView uint64

	epochCounter       uint64            // current epoch counter
	myIndex            int               // my index in the DKG
	committee          flow.IdentityList // the DKG committee
	expectedPrivateKey crypto.PrivateKey
	firstBlock         *flow.Header
	blocksByView       map[uint64]*flow.Header

	local        *module.Local
	currentEpoch *protocol.Epoch
	nextEpoch    *protocol.Epoch
	epochQuery   *mocks.EpochQuery
	snapshot     *protocol.Snapshot
	state        *protocol.State
	viewEvents   *gadgets.Views

	dkgKeys    *storage.DKGKeys
	dkgState   *storage.DKGState
	controller *module.DKGController
	factory    *module.DKGControllerFactory

	engine *dkg.ReactorEngine
}

func (suite *ReactorSuite) NextEpochCounter() uint64 {
	return suite.epochCounter + 1
}

func TestReactorSuite(t *testing.T) {
	suite.Run(t, new(ReactorSuite))
}

// SetupTest prepares the DKG test.
//
// The EpochSetup event is received at view 100.
// The current epoch is configured with DKG phase transitions at views 150, 200,
// and 250. In between phase transitions, the controller calls the DKG
// smart-contract every 10 views.
//
// VIEWS
// setup      : 100
// polling    : 110 120 130 140 150
// Phase1Final: 150
// polling    : 160 170 180 190 200
// Phase2Final: 200
// polling    : 210 220 230 240 250
// Phase3Final: 250
func (suite *ReactorSuite) SetupTest() {

	suite.dkgStartView = 100
	suite.dkgPhase1FinalView = 150
	suite.dkgPhase2FinalView = 200
	suite.dkgPhase3FinalView = 250

	suite.epochCounter = rand.Uint64()
	suite.committee = unittest.IdentityListFixture(10)
	suite.myIndex = 5

	suite.local = new(module.Local)
	suite.local.On("NodeID").Return(suite.committee[suite.myIndex].NodeID)

	// create a block for each view of interest
	suite.blocksByView = make(map[uint64]*flow.Header)
	for view := suite.dkgStartView; view <= suite.dkgPhase3FinalView; view += dkg.DefaultPollStep {
		header := unittest.BlockHeaderFixture(unittest.HeaderWithView(view))
		suite.blocksByView[view] = &header
	}
	suite.firstBlock = suite.blocksByView[100]

	// expectedPrivKey is the expected private share produced by the dkg run. We
	// will mock the controller to return this value, and we will check it
	// against the value that gets inserted in the DB at the end.
	suite.expectedPrivateKey = unittest.PrivateKeyFixture(crypto.BLSBLS12381, 48)

	// mock protocol state
	suite.currentEpoch = new(protocol.Epoch)
	suite.currentEpoch.On("Counter").Return(suite.epochCounter, nil)
	suite.currentEpoch.On("DKGPhase1FinalView").Return(suite.dkgPhase1FinalView, nil)
	suite.currentEpoch.On("DKGPhase2FinalView").Return(suite.dkgPhase2FinalView, nil)
	suite.currentEpoch.On("DKGPhase3FinalView").Return(suite.dkgPhase3FinalView, nil)
	suite.nextEpoch = new(protocol.Epoch)
	suite.nextEpoch.On("Counter").Return(suite.NextEpochCounter(), nil)
	suite.nextEpoch.On("InitialIdentities").Return(suite.committee, nil)

	suite.epochQuery = mocks.NewEpochQuery(suite.T(), suite.epochCounter)
	suite.epochQuery.Add(suite.currentEpoch)
	suite.epochQuery.Add(suite.nextEpoch)
	suite.snapshot = new(protocol.Snapshot)
	suite.snapshot.On("Epochs").Return(suite.epochQuery)
	suite.state = new(protocol.State)
	suite.state.On("AtBlockID", suite.firstBlock.ID()).Return(suite.snapshot)

	// ensure that an attempt is made to insert the expected dkg private share
	// for the next epoch.
	suite.dkgKeys = new(storage.DKGKeys)
	suite.dkgKeys.On("InsertMyDKGPrivateInfo", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			epochCounter := args.Get(0).(uint64)
			suite.Require().Equal(suite.NextEpochCounter(), epochCounter)
			dkgPriv := args.Get(1).(*dkgmodel.DKGParticipantPriv)
			suite.Require().Equal(suite.local.NodeID(), dkgPriv.NodeID)
			suite.Require().Equal(suite.expectedPrivateKey, dkgPriv.RandomBeaconPrivKey.PrivateKey)
			suite.Require().Equal(suite.myIndex, dkgPriv.GroupIndex)
		}).
		Return(nil).
		Once()
	suite.dkgState.On("SetDKGStarted", suite.NextEpochCounter()).Return(nil).Once()

	// we will ensure that the controller state transitions get called appropriately
	suite.controller = new(module.DKGController)
	suite.controller.On("Run").Return(nil).Once()
	suite.controller.On("EndPhase1").Return(nil).Once()
	suite.controller.On("EndPhase2").Return(nil).Once()
	suite.controller.On("End").Return(nil).Once()
	suite.controller.On("Poll", mock.Anything).Return(nil).Times(15)
	suite.controller.On("GetArtifacts").Return(suite.expectedPrivateKey, nil, nil).Once()
	suite.controller.On("GetIndex").Return(suite.myIndex).Once()
	suite.controller.On("SubmitResult").Return(nil).Once()

	suite.factory = new(module.DKGControllerFactory)
	suite.factory.On("Create",
		dkgmodule.CanonicalInstanceID(suite.firstBlock.ChainID, suite.NextEpochCounter()),
		suite.committee,
		mock.Anything,
	).Return(suite.controller, nil)

	suite.viewEvents = gadgets.NewViews()
	suite.engine = dkg.NewReactorEngine(
		unittest.Logger(),
		suite.local,
		suite.state,
		suite.dkgKeys,
		suite.factory,
		suite.viewEvents,
	)
}

// TestRunDKG_PhaseTransition tests that the DKG is started and completed successfully
// after a phase transition from StakingPhase->SetupPhase.
func (suite *ReactorSuite) TestRunDKG_PhaseTransition() {

	// protocol event indicating the setup phase is starting
	suite.engine.EpochSetupPhaseStarted(suite.epochCounter, suite.firstBlock)
	// the dkg for this epoch has not been started
	suite.dkgState.On("GetDKGStarted", suite.NextEpochCounter()).Return(false, nil).Once()

	for view := uint64(100); view <= 250; view += dkg.DefaultPollStep {
		suite.viewEvents.BlockFinalized(suite.blocksByView[view])
	}

	// check that the appropriate callbacks were registered
	time.Sleep(50 * time.Millisecond)
	suite.controller.AssertExpectations(suite.T())
	suite.dkgKeys.AssertExpectations(suite.T())
}

// TestRunDKG_StartupInSetupPhase tests that the DKG is started and completed
// successfully when the engine starts up during the EpochSetup phase, and the
// DKG for this epoch has not been started previously. This is the case for
// consensus nodes joining the network at an epoch boundary.
//
func (suite *ReactorSuite) TestRunDKG_StartupInSetupPhase() {

	// we are in the EpochSetup phase
	suite.snapshot.On("Phase").Return(flow.EpochPhaseSetup, nil).Once()
	// the dkg for this epoch has not been started
	suite.dkgState.On("GetDKGStarted", suite.NextEpochCounter()).Return(false, nil).Once()

	// start up the engine
	unittest.AssertClosesBefore(suite.T(), suite.engine.Ready(), time.Second)

	for view := uint64(100); view <= 250; view += dkg.DefaultPollStep {
		suite.viewEvents.BlockFinalized(suite.blocksByView[view])
	}

	// check that the appropriate callbacks were registered
	time.Sleep(50 * time.Millisecond)
	suite.controller.AssertExpectations(suite.T())
	suite.dkgKeys.AssertExpectations(suite.T())
}

// TestRunDKG_StartupInSetupPhase_DKGAlreadyStarted tests that the DKG is NOT
// started, when the engine starts up during the EpochSetup phase, and the DKG
// for this epoch HAS been started previously. This will be the case for
// consensus nodes which restart during the DKG.
//
func (suite *ReactorSuite) TestRunDKG_StartupInSetupPhase_DKGAlreadyStarted() {

	// we are in the EpochSetup phase
	suite.snapshot.On("Phase").Return(flow.EpochPhaseSetup, nil).Once()
	// the dkg for this epoch has been started
	suite.dkgState.On("GetDKGStarted", suite.NextEpochCounter()).Return(true, nil).Once()

	// start up the engine
	unittest.AssertClosesBefore(suite.T(), suite.engine.Ready(), time.Second)

	// we should not have instantiated the DKG
	suite.factory.AssertNotCalled(suite.T(), "Create",
		dkgmodule.CanonicalInstanceID(suite.firstBlock.ChainID, suite.NextEpochCounter()),
		suite.committee,
		mock.Anything,
	)
}

// TestReactorEngine_EpochCommittedPhaseStarted ensures that we are logging
// a warning message whenever we have a mismatch between the locally produced DKG keys
// and the keys produced by the DKG smart contract.
func TestReactorEngine_EpochCommittedPhaseStarted(t *testing.T) {

	rand.Seed(time.Now().UnixNano())
	currentCounter := rand.Uint64()
	nextCounter := currentCounter + 1
	me := new(module.Local)

	// privKey represents private key generated by DKG for the next epoch.
	privKey := unittest.StakingPrivKeyFixture()

	// dkgParticipantPrivInfo.RandomBeaconPrivKey.PublicKey() will return a public key
	// that does not match the public key for the priv key generated above and cause a warning
	// to be logged.
	dkgParticipantPrivInfo := unittest.DKGParticipantPriv()

	keyStorage := new(storage.DKGKeys)
	keyStorage.On("RetrieveMyDKGPrivateInfo", currentCounter+1).Return(dkgParticipantPrivInfo, nil)
	factory := new(module.DKGControllerFactory)

	nextDKG := new(protocol.DKG)
	nextDKG.On("KeyShare", dkgParticipantPrivInfo.NodeID).Return(privKey.PublicKey(), nil)

	currentEpoch := new(protocol.Epoch)
	currentEpoch.On("Counter").Return(currentCounter, nil)

	nextEpoch := new(protocol.Epoch)
	nextEpoch.On("Counter").Return(nextCounter, nil)
	nextEpoch.On("DKG").Return(nextDKG, nil)

	epochQuery := mocks.NewEpochQuery(t, currentCounter)
	epochQuery.Add(currentEpoch)
	epochQuery.Add(nextEpoch)

	firstBlock := unittest.BlockHeaderFixture()
	firstBlock.View = 100

	snapshot := new(protocol.Snapshot)
	snapshot.On("Epochs").Return(epochQuery)

	state := new(protocol.State)
	state.On("Final").Return(snapshot)

	viewEvents := gadgets.NewViews()

	hookCalls := 0

	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.WarnLevel {
			hookCalls++
		}
	})
	logger := zerolog.New(os.Stdout).Level(zerolog.WarnLevel).Hook(hook)

	engine := dkg.NewReactorEngine(
		logger,
		me,
		state,
		keyStorage,
		factory,
		viewEvents,
	)

	engine.EpochCommittedPhaseStarted(currentCounter, &firstBlock)

	require.Equal(t, 1, hookCalls)
}
