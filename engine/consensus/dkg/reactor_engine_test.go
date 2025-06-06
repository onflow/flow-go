package dkg_test

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/engine/consensus/dkg"
	"github.com/onflow/flow-go/model/flow"
	dkgmodule "github.com/onflow/flow-go/module/dkg"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// ReactorEngineSuite_SetupPhase is a test suite for the Reactor engine which encompasses
// test cases from when the DKG is instantiated to when it terminates locally.
//
// For tests of the Reactor engine's reaction to the global end of the DKG, see
// ReactorEngineSuite_CommittedPhase.
type ReactorEngineSuite_SetupPhase struct {
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

	// track how many warn-level logs are logged
	warnsLogged int
	logger      zerolog.Logger

	local        *module.Local
	currentEpoch *protocol.CommittedEpoch
	nextEpoch    *protocol.TentativeEpoch
	epochQuery   *mocks.EpochQuery
	snapshot     *protocol.Snapshot
	state        *protocol.State
	viewEvents   *gadgets.Views

	dkgState   *storage.DKGState
	controller *module.DKGController
	factory    *module.DKGControllerFactory

	engine *dkg.ReactorEngine
}

func (suite *ReactorEngineSuite_SetupPhase) NextEpochCounter() uint64 {
	return suite.epochCounter + 1
}

func TestReactorEngineSuite_SetupPhase(t *testing.T) {
	suite.Run(t, new(ReactorEngineSuite_SetupPhase))
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
func (suite *ReactorEngineSuite_SetupPhase) SetupTest() {

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
		suite.blocksByView[view] = header
	}
	suite.firstBlock = suite.blocksByView[100]

	// expectedPrivKey is the expected private share produced by the dkg run. We
	// will mock the controller to return this value, and we will check it
	// against the value that gets inserted in the DB at the end.
	suite.expectedPrivateKey = unittest.PrivateKeyFixture(crypto.BLSBLS12381)

	// mock protocol state
	suite.currentEpoch = new(protocol.CommittedEpoch)
	suite.currentEpoch.On("Counter").Return(suite.epochCounter)
	suite.currentEpoch.On("DKGPhase1FinalView").Return(suite.dkgPhase1FinalView)
	suite.currentEpoch.On("DKGPhase2FinalView").Return(suite.dkgPhase2FinalView)
	suite.currentEpoch.On("DKGPhase3FinalView").Return(suite.dkgPhase3FinalView)
	suite.nextEpoch = new(protocol.TentativeEpoch)
	suite.nextEpoch.On("Counter").Return(suite.NextEpochCounter())
	suite.nextEpoch.On("InitialIdentities").Return(suite.committee.ToSkeleton())

	suite.epochQuery = mocks.NewEpochQuery(suite.T(), suite.epochCounter)
	suite.epochQuery.AddCommitted(suite.currentEpoch)
	suite.epochQuery.AddTentative(suite.nextEpoch)
	suite.snapshot = new(protocol.Snapshot)
	suite.snapshot.On("Epochs").Return(suite.epochQuery)
	suite.snapshot.On("Head").Return(suite.firstBlock, nil)
	suite.state = new(protocol.State)
	suite.state.On("AtBlockID", suite.firstBlock.ID()).Return(suite.snapshot)
	suite.state.On("Final").Return(suite.snapshot)

	// ensure that an attempt is made to insert the expected dkg private share
	// for the next epoch.
	suite.dkgState = new(storage.DKGState)
	suite.dkgState.On("SetDKGState", suite.NextEpochCounter(), flow.DKGStateStarted).Return(nil).Once()
	suite.dkgState.On("InsertMyBeaconPrivateKey", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			epochCounter := args.Get(0).(uint64)
			require.Equal(suite.T(), suite.NextEpochCounter(), epochCounter)
			dkgPriv := args.Get(1).(crypto.PrivateKey)
			require.Equal(suite.T(), suite.expectedPrivateKey, dkgPriv)
		}).
		Return(nil).
		Once()

	// we will ensure that the controller state transitions get called appropriately
	suite.controller = new(module.DKGController)
	suite.controller.On("Run").Return(nil).Once()
	suite.controller.On("EndPhase1").Return(nil).Once()
	suite.controller.On("EndPhase2").Return(nil).Once()
	suite.controller.On("End").Return(nil).Once()
	suite.controller.On("Poll", mock.Anything).Return(nil).Times(15)
	suite.controller.On("GetArtifacts").Return(suite.expectedPrivateKey, nil, nil).Once()
	suite.controller.On("SubmitResult").Return(nil).Once()

	suite.factory = new(module.DKGControllerFactory)
	suite.factory.On("Create",
		dkgmodule.CanonicalInstanceID(suite.firstBlock.ChainID, suite.NextEpochCounter()),
		suite.committee.ToSkeleton(),
		mock.Anything,
	).Return(suite.controller, nil)

	suite.warnsLogged = 0
	suite.logger = hookedLogger(&suite.warnsLogged)

	suite.viewEvents = gadgets.NewViews()
	suite.engine = dkg.NewReactorEngine(
		suite.logger,
		suite.local,
		suite.state,
		suite.dkgState,
		suite.factory,
		suite.viewEvents,
	)
}

// TestRunDKG_PhaseTransition tests that the DKG is started and completed successfully
// after a phase transition from StakingPhase->SetupPhase.
func (suite *ReactorEngineSuite_SetupPhase) TestRunDKG_PhaseTransition() {

	// the dkg for this epoch has not been started
	suite.dkgState.On("IsDKGStarted", suite.NextEpochCounter()).Return(false, nil).Once()
	// protocol event indicating the setup phase is starting
	suite.engine.EpochSetupPhaseStarted(suite.epochCounter, suite.firstBlock)

	for view := uint64(100); view <= 250; view += dkg.DefaultPollStep {
		suite.viewEvents.BlockFinalized(suite.blocksByView[view])
	}

	// check that the appropriate callbacks were registered
	time.Sleep(50 * time.Millisecond)
	suite.controller.AssertExpectations(suite.T())
	suite.dkgState.AssertExpectations(suite.T())
	// happy path - no warn logs expected
	suite.Assert().Equal(0, suite.warnsLogged)
}

// TestRunDKG_StartupInSetupPhase tests that the DKG is started and completed
// successfully when the engine starts up during the EpochSetup phase, and the
// DKG for this epoch has not been started previously. This is the case for
// consensus nodes joining the network at an epoch boundary.
func (suite *ReactorEngineSuite_SetupPhase) TestRunDKG_StartupInSetupPhase() {

	// we are in the EpochSetup phase
	suite.snapshot.On("EpochPhase").Return(flow.EpochPhaseSetup, nil).Once()
	// the dkg for this epoch has not been started
	suite.dkgState.On("IsDKGStarted", suite.NextEpochCounter()).Return(false, nil).Once()

	// start up the engine
	unittest.AssertClosesBefore(suite.T(), suite.engine.Ready(), time.Second)

	// keyStorage := new(storage.DKGKeys)
	// keyStorage.On("RetrieveMyDKGPrivateInfo", currentCounter+1).Return(dkgParticipantPrivInfo, true, nil)
	// factory := new(module.DKGControllerFactory)
	for view := uint64(100); view <= 250; view += dkg.DefaultPollStep {
		suite.viewEvents.BlockFinalized(suite.blocksByView[view])
	}

	// check that the appropriate callbacks were registered
	time.Sleep(50 * time.Millisecond)
	suite.controller.AssertExpectations(suite.T())
	suite.dkgState.AssertExpectations(suite.T())
	// happy path - no warn logs expected
	suite.Assert().Equal(0, suite.warnsLogged)
}

// TestRunDKG_StartupInSetupPhase_DKGAlreadyStarted tests that the DKG is NOT
// started, when the engine starts up during the EpochSetup phase, and the DKG
// for this epoch HAS been started previously. This will be the case for
// consensus nodes which restart during the DKG.
func (suite *ReactorEngineSuite_SetupPhase) TestRunDKG_StartupInSetupPhase_DKGAlreadyStarted() {

	// we are in the EpochSetup phase
	suite.snapshot.On("EpochPhase").Return(flow.EpochPhaseSetup, nil).Once()
	// the dkg for this epoch has been started
	suite.dkgState.On("IsDKGStarted", suite.NextEpochCounter()).Return(true, nil).Once()

	// start up the engine
	unittest.AssertClosesBefore(suite.T(), suite.engine.Ready(), time.Second)

	// we should not have instantiated the DKG
	suite.factory.AssertNotCalled(suite.T(), "Create",
		dkgmodule.CanonicalInstanceID(suite.firstBlock.ChainID, suite.NextEpochCounter()),
		suite.committee,
		mock.Anything,
	)

	// we should log a warning that the DKG has already started
	suite.Assert().Equal(1, suite.warnsLogged)
}

// ReactorEngineSuite_CommittedPhase tests the Reactor engine's operation
// during the transition to the EpochCommitted phase, after the DKG has
// completed locally, and we are comparing our local results to the
// canonical DKG results.
type ReactorEngineSuite_CommittedPhase struct {
	suite.Suite

	epochCounter         uint64            // current epoch counter
	myLocalBeaconKey     crypto.PrivateKey // my locally computed beacon key
	myGlobalBeaconPubKey crypto.PublicKey  // my public key, as dictated by global DKG
	DKGState             flow.DKGState     // backend for DGKState.
	firstBlock           *flow.Header      // first block of EpochCommitted phase
	warnsLogged          int               // count # of warn-level logs

	me       *module.Local
	dkgState *storage.DKGState
	state    *protocol.State
	snap     *protocol.Snapshot
	factory  *module.DKGControllerFactory

	engine *dkg.ReactorEngine
}

func TestReactorEngineSuite_CommittedPhase(t *testing.T) {
	suite.Run(t, new(ReactorEngineSuite_CommittedPhase))
}

func (suite *ReactorEngineSuite_CommittedPhase) NextEpochCounter() uint64 {
	return suite.epochCounter + 1
}

func (suite *ReactorEngineSuite_CommittedPhase) SetupTest() {

	suite.epochCounter = rand.Uint64()
	suite.DKGState = flow.DKGStateCompleted // we start with the completed state since we are going to test the transition to committed
	suite.me = new(module.Local)

	id := unittest.IdentifierFixture()
	suite.me.On("NodeID").Return(id)

	// by default we seed the test suite with consistent keys
	suite.myLocalBeaconKey = unittest.RandomBeaconPriv().PrivateKey
	suite.myGlobalBeaconPubKey = suite.myLocalBeaconKey.PublicKey()

	suite.dkgState = new(storage.DKGState)
	suite.dkgState.On("UnsafeRetrieveMyBeaconPrivateKey", suite.NextEpochCounter()).Return(
		func(_ uint64) crypto.PrivateKey { return suite.myLocalBeaconKey },
		func(_ uint64) error {
			if suite.myLocalBeaconKey == nil {
				return storerr.ErrNotFound
			}
			return nil
		},
	)
	suite.dkgState.On("SetDKGState", suite.NextEpochCounter(), mock.Anything).
		Run(func(args mock.Arguments) {
			assert.Equal(suite.T(), flow.DKGStateCompleted, suite.DKGState) // must be equal to the initial state of the test
			endState := args[1].(flow.DKGState)
			suite.DKGState = endState
		}).
		Return(nil)
	suite.dkgState.On("GetDKGState", suite.NextEpochCounter()).Return(
		func(_ uint64) flow.DKGState { return suite.DKGState },
		func(_ uint64) error {
			if suite.DKGState == flow.DKGStateUninitialized {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	currentEpoch := new(protocol.CommittedEpoch)
	currentEpoch.On("Counter").Return(suite.epochCounter, nil)

	nextDKG := new(protocol.DKG)
	nextDKG.On("KeyShare", id).Return(
		func(_ flow.Identifier) crypto.PublicKey { return suite.myGlobalBeaconPubKey },
		func(_ flow.Identifier) error { return nil },
	)

	nextEpoch := new(protocol.CommittedEpoch)
	nextEpoch.On("Counter").Return(suite.NextEpochCounter(), nil)
	nextEpoch.On("DKG").Return(nextDKG, nil)

	epochQuery := mocks.NewEpochQuery(suite.T(), suite.epochCounter)
	epochQuery.AddCommitted(currentEpoch)
	epochQuery.AddCommitted(nextEpoch)

	firstBlock := unittest.BlockHeaderFixture(unittest.HeaderWithView(100))
	suite.firstBlock = firstBlock

	suite.snap = new(protocol.Snapshot)
	suite.snap.On("Epochs").Return(epochQuery)
	suite.snap.On("Head").Return(firstBlock, nil)

	suite.state = new(protocol.State)
	suite.state.On("AtBlockID", firstBlock.ID()).Return(suite.snap)
	suite.state.On("Final").Return(suite.snap)

	// count number of warn-level logs
	suite.warnsLogged = 0
	logger := hookedLogger(&suite.warnsLogged)

	suite.factory = new(module.DKGControllerFactory)
	viewEvents := gadgets.NewViews()

	suite.engine = dkg.NewReactorEngine(
		logger,
		suite.me,
		suite.state,
		suite.dkgState,
		suite.factory,
		viewEvents,
	)
}

// TestDKGSuccess tests the path where we are checking the global DKG
// results and observe our key is consistent.
// We should:
// * set the DKG end state to Success
func (suite *ReactorEngineSuite_CommittedPhase) TestDKGSuccess() {

	entry := unittest.EpochStateFixture(unittest.WithNextEpochProtocolState(), func(entry *flow.RichEpochStateEntry) {
		entry.NextEpochCommit.Counter = suite.NextEpochCounter()
		entry.NextEpoch.CommitID = entry.NextEpochCommit.ID()
	})
	epochProtocolState := protocol.NewEpochProtocolState(suite.T())
	epochProtocolState.On("Entry").Return(entry)
	suite.snap.On("EpochProtocolState").Return(epochProtocolState, nil)
	suite.dkgState.On("CommitMyBeaconPrivateKey", suite.NextEpochCounter(), entry.NextEpochCommit).Return(nil).Once()
	suite.engine.EpochCommittedPhaseStarted(suite.epochCounter, suite.firstBlock)
	suite.Require().Equal(0, suite.warnsLogged)
	// ensure we commit my beacon private key
	suite.dkgState.AssertCalled(suite.T(), "CommitMyBeaconPrivateKey", suite.NextEpochCounter(), entry.NextEpochCommit)
}

// TestInconsistentKey tests the path where we are checking the global DKG
// results and observe that our locally computed key is inconsistent.
// We should:
// * log a warning
// * set the DKG end state accordingly
func (suite *ReactorEngineSuite_CommittedPhase) TestInconsistentKey() {

	// set our global pub key to a random value
	suite.myGlobalBeaconPubKey = unittest.RandomBeaconPriv().PublicKey()

	suite.engine.EpochCommittedPhaseStarted(suite.epochCounter, suite.firstBlock)
	suite.Require().Equal(1, suite.warnsLogged)
	suite.Assert().Equal(flow.DKGStateFailure, suite.DKGState)
}

// TestMissingKey tests the path where we are checking the global DKG results
// and observe that we have not stored a locally computed key.
// We should:
// * log a warning
// * set the DKG end state accordingly
func (suite *ReactorEngineSuite_CommittedPhase) TestMissingKey() {

	// remove our key
	suite.myLocalBeaconKey = nil

	suite.engine.EpochCommittedPhaseStarted(suite.epochCounter, suite.firstBlock)
	suite.Require().Equal(1, suite.warnsLogged)
	suite.Assert().Equal(flow.DKGStateFailure, suite.DKGState)
}

// TestLocalDKGFailure tests the path where we are checking the global DKG
// results and observe that we have already set the DKG end state as a failure.
// We should:
// * log a warning
// * keep the dkg end state as it is
func (suite *ReactorEngineSuite_CommittedPhase) TestLocalDKGFailure() {

	// set dkg end state as failure
	suite.DKGState = flow.DKGStateFailure

	suite.engine.EpochCommittedPhaseStarted(suite.epochCounter, suite.firstBlock)
	suite.Require().Equal(1, suite.warnsLogged)
	suite.Assert().Equal(flow.DKGStateFailure, suite.DKGState)
}

// TestStartupInCommittedPhase_DKGSuccess tests that the dkg end state is correctly
// set when starting in EpochCommitted phase and a successful DKG
func (suite *ReactorEngineSuite_CommittedPhase) TestStartupInCommittedPhase_DKGSuccess() {

	// we are in the EpochSetup phase
	suite.snap.On("EpochPhase").Return(flow.EpochPhaseCommitted, nil).Once()
	// the dkg for this epoch has been started but not ended
	suite.dkgState.On("IsDKGStarted", suite.NextEpochCounter()).Return(true, nil).Once()
	suite.DKGState = flow.DKGStateCompleted

	entry := unittest.EpochStateFixture(unittest.WithNextEpochProtocolState(), func(entry *flow.RichEpochStateEntry) {
		entry.NextEpochCommit.Counter = suite.NextEpochCounter()
		entry.NextEpoch.CommitID = entry.NextEpochCommit.ID()
	})
	epochProtocolState := protocol.NewEpochProtocolState(suite.T())
	epochProtocolState.On("Entry").Return(entry)
	suite.snap.On("EpochProtocolState").Return(epochProtocolState, nil)
	suite.dkgState.On("CommitMyBeaconPrivateKey", suite.NextEpochCounter(), entry.NextEpochCommit).Return(nil).Once()

	// start up the engine
	unittest.AssertClosesBefore(suite.T(), suite.engine.Ready(), time.Second)

	// we should not have instantiated the DKG
	suite.factory.AssertNotCalled(suite.T(), "Create",
		dkgmodule.CanonicalInstanceID(suite.firstBlock.ChainID, suite.NextEpochCounter()),
		mock.Anything,
		mock.Anything,
	)
	// ensure we commit my beacon private key
	suite.dkgState.AssertCalled(suite.T(), "CommitMyBeaconPrivateKey", suite.NextEpochCounter(), entry.NextEpochCommit)
}

// TestStartupInCommittedPhase_DKGSuccess tests that the dkg end state is correctly
// set when starting in EpochCommitted phase and the DKG end state is already set.
func (suite *ReactorEngineSuite_CommittedPhase) TestStartupInCommittedPhase_DKGStateAlreadySet() {

	// we are in the Epoch Commit phase
	suite.snap.On("EpochPhase").Return(flow.EpochPhaseCommitted, nil).Once()
	// the dkg for this epoch has been started and ended
	suite.dkgState.On("IsDKGStarted", suite.NextEpochCounter()).Return(true, nil).Once()
	suite.DKGState = flow.DKGStateFailure

	// start up the engine
	unittest.AssertClosesBefore(suite.T(), suite.engine.Ready(), time.Second)

	// we should not have instantiated the DKG
	suite.factory.AssertNotCalled(suite.T(), "Create",
		dkgmodule.CanonicalInstanceID(suite.firstBlock.ChainID, suite.NextEpochCounter()),
		mock.Anything,
		mock.Anything,
	)
}

// TestStartupInCommittedPhase_InconsistentKey tests that the dkg end state is correctly
// set when starting in EpochCommitted phase and we have stored an inconsistent key.
func (suite *ReactorEngineSuite_CommittedPhase) TestStartupInCommittedPhase_InconsistentKey() {

	// we are in the EpochSetup phase
	suite.snap.On("EpochPhase").Return(flow.EpochPhaseCommitted, nil).Once()
	// the dkg for this epoch has been started but not ended
	suite.dkgState.On("IsDKGStarted", suite.NextEpochCounter()).Return(true, nil).Once()
	suite.dkgState.On("GetDKGState", suite.NextEpochCounter()).Return(flow.DKGStateUninitialized, storerr.ErrNotFound).Once()

	// set our global pub key to a random value
	suite.myGlobalBeaconPubKey = unittest.RandomBeaconPriv().PublicKey()

	// start up the engine
	unittest.AssertClosesBefore(suite.T(), suite.engine.Ready(), time.Second)

	// we should not have instantiated the DKG
	suite.factory.AssertNotCalled(suite.T(), "Create",
		dkgmodule.CanonicalInstanceID(suite.firstBlock.ChainID, suite.NextEpochCounter()),
		mock.Anything,
		mock.Anything,
	)
	// should set DKG end state
	suite.Assert().Equal(flow.DKGStateFailure, suite.DKGState)
}

// TestStartupInCommittedPhase_MissingKey tests that the dkg end state is correctly
// set when starting in EpochCommitted phase and we have not stored any key.
func (suite *ReactorEngineSuite_CommittedPhase) TestStartupInCommittedPhase_MissingKey() {

	// we are in the EpochSetup phase
	suite.snap.On("EpochPhase").Return(flow.EpochPhaseCommitted, nil).Once()
	// the dkg for this epoch has been started but not ended
	suite.dkgState.On("IsDKGStarted", suite.NextEpochCounter()).Return(true, nil).Once()
	suite.dkgState.On("GetDKGState", suite.NextEpochCounter()).Return(flow.DKGStateUninitialized, storerr.ErrNotFound).Once()

	// remove our key
	suite.myLocalBeaconKey = nil

	// start up the engine
	unittest.AssertClosesBefore(suite.T(), suite.engine.Ready(), time.Second)

	// we should not have instantiated the DKG
	suite.factory.AssertNotCalled(suite.T(), "Create",
		dkgmodule.CanonicalInstanceID(suite.firstBlock.ChainID, suite.NextEpochCounter()),
		mock.Anything,
		mock.Anything,
	)
	// should set DKG end state
	suite.Assert().Equal(flow.DKGStateFailure, suite.DKGState)
}

// utility function to track the number of warn-level calls to a logger
func hookedLogger(calls *int) zerolog.Logger {
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.WarnLevel {
			*calls++
		}
	})
	return zerolog.New(os.Stdout).Level(zerolog.InfoLevel).Hook(hook)
}
