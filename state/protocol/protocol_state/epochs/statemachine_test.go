package epochs_test

import (
	"errors"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/state/protocol/protocol_state/epochs"
	"github.com/onflow/flow-go/state/protocol/protocol_state/epochs/mock"
	mock_interfaces "github.com/onflow/flow-go/state/protocol/protocol_state/epochs/mock_interfaces/mock"
	protocol_statemock "github.com/onflow/flow-go/state/protocol/protocol_state/mock"
	protocol_mock_interfaces "github.com/onflow/flow-go/state/protocol/protocol_state/mock_interfaces/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEpochStateMachine(t *testing.T) {
	suite.Run(t, new(EpochStateMachineSuite))
}

// EpochStateMachineSuite is a dedicated test suite for testing hierarchical epoch state machine.
// All needed dependencies are mocked, including KV store as a whole, and all the necessary storages.
// Tests in this suite are designed to rely on automatic assertions when leaving the scope of the test.
type EpochStateMachineSuite struct {
	suite.Suite
	epochStateDB                    *storagemock.EpochProtocolStateEntries
	setupsDB                        *storagemock.EpochSetups
	commitsDB                       *storagemock.EpochCommits
	globalParams                    *protocolmock.GlobalParams
	parentState                     *protocolmock.KVStoreReader
	parentEpochState                *flow.RichEpochStateEntry
	mutator                         *protocol_statemock.KVStoreMutator
	happyPathStateMachine           *mock.StateMachine
	happyPathStateMachineFactory    *mock_interfaces.StateMachineFactoryMethod
	fallbackPathStateMachineFactory *mock_interfaces.StateMachineFactoryMethod
	candidate                       *flow.Header
	lockManager                     lockctx.Manager

	stateMachine *epochs.EpochStateMachine
}

func (s *EpochStateMachineSuite) SetupTest() {
	s.epochStateDB = storagemock.NewEpochProtocolStateEntries(s.T())
	s.setupsDB = storagemock.NewEpochSetups(s.T())
	s.commitsDB = storagemock.NewEpochCommits(s.T())
	s.parentState = protocolmock.NewKVStoreReader(s.T())
	s.parentState.On("GetFinalizationSafetyThreshold").Return(uint64(1_000))
	s.parentEpochState = unittest.EpochStateFixture()
	s.mutator = protocol_statemock.NewKVStoreMutator(s.T())
	s.candidate = unittest.BlockHeaderFixture(unittest.HeaderWithView(s.parentEpochState.CurrentEpochSetup.FirstView + 1))
	s.happyPathStateMachine = mock.NewStateMachine(s.T())
	s.happyPathStateMachineFactory = mock_interfaces.NewStateMachineFactoryMethod(s.T())
	s.fallbackPathStateMachineFactory = mock_interfaces.NewStateMachineFactoryMethod(s.T())
	s.lockManager = storage.NewTestingLockManager()

	s.epochStateDB.On("ByBlockID", mocks.Anything).Return(func(_ flow.Identifier) *flow.RichEpochStateEntry {
		return s.parentEpochState
	}, func(_ flow.Identifier) error {
		return nil
	})
	s.parentState.On("GetEpochStateID").Return(func() flow.Identifier {
		return s.parentEpochState.ID()
	})

	s.happyPathStateMachineFactory.On("Execute", s.candidate.View, s.parentEpochState).
		Return(s.happyPathStateMachine, nil).Once()

	s.happyPathStateMachine.On("ParentState").Return(s.parentEpochState).Maybe()

	var err error
	s.stateMachine, err = epochs.NewEpochStateMachine(
		s.candidate.View,
		s.candidate.ParentID,
		s.setupsDB,
		s.commitsDB,
		s.epochStateDB,
		s.parentState,
		s.mutator,
		s.happyPathStateMachineFactory.Execute,
		s.fallbackPathStateMachineFactory.Execute,
	)
	require.NoError(s.T(), err)
}

// TestBuild_NoChanges tests that hierarchical epoch state machine maintains index of epoch states and commits
// epoch state ID in the KV store even when there were no events to process.
func (s *EpochStateMachineSuite) TestBuild_NoChanges() {
	s.happyPathStateMachine.On("ParentState").Return(s.parentEpochState)
	s.happyPathStateMachine.On("Build").Return(s.parentEpochState.EpochStateEntry, s.parentEpochState.ID(), false).Once()

	err := s.stateMachine.EvolveState(nil)
	require.NoError(s.T(), err)

	rw := storagemock.NewReaderBatchWriter(s.T())

	// Create a proper lock context proof for the BatchIndex operation
	err = unittest.WithLock(s.T(), s.lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
		s.epochStateDB.On("BatchIndex", lctx, rw, s.candidate.ID(), s.parentEpochState.ID()).Return(nil).Once()
		s.mutator.On("SetEpochStateID", s.parentEpochState.ID()).Return(nil).Once()

		dbUpdates, err := s.stateMachine.Build()
		require.NoError(s.T(), err)

		// Storage operations are deferred, because block ID is not known when the block is newly constructed. Only at the
		// end after the block is fully constructed, its ID can be computed. We emulate this step here to verify that the
		// deferred `dbOps` have been correctly constructed. Thereby, the expected mock methods should be called,
		// which is asserted by the testify framework.
		blockID := s.candidate.ID()
		return dbUpdates.Execute(lctx, blockID, rw)
	})
	require.NoError(s.T(), err)
}

// TestBuild_HappyPath tests that hierarchical epoch state machine maintains index of epoch states and commits
// as well as stores updated epoch state in respective storage when there were updates made to the epoch state.
// This test also ensures that updated state ID is committed in the KV store.
func (s *EpochStateMachineSuite) TestBuild_HappyPath() {
	s.happyPathStateMachine.On("ParentState").Return(s.parentEpochState)
	updatedState := unittest.EpochStateFixture().EpochStateEntry
	updatedStateID := updatedState.ID()
	s.happyPathStateMachine.On("Build").Return(updatedState, updatedStateID, true).Once()

	epochSetup := unittest.EpochSetupFixture()
	epochCommit := unittest.EpochCommitFixture()

	// expected both events to be processed
	s.happyPathStateMachine.On("ProcessEpochSetup", epochSetup).Return(true, nil).Once()
	s.happyPathStateMachine.On("ProcessEpochCommit", epochCommit).Return(true, nil).Once()

	w := storagemock.NewWriter(s.T())
	rw := storagemock.NewReaderBatchWriter(s.T())
	rw.On("Writer").Return(w).Once() // called by epochStateDB.BatchStore
	// prepare a DB update for epoch setup
	s.setupsDB.On("BatchStore", rw, epochSetup).Return(nil).Once()

	// prepare a DB update for epoch commit
	s.commitsDB.On("BatchStore", rw, epochCommit).Return(nil).Once()

	err := s.stateMachine.EvolveState([]flow.ServiceEvent{epochSetup.ServiceEvent(), epochCommit.ServiceEvent()})
	require.NoError(s.T(), err)

	// Create a proper lock context proof for the BatchIndex operation
	err = unittest.WithLock(s.T(), s.lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
		// prepare a DB update for epoch state
		s.epochStateDB.On("BatchIndex", lctx, rw, s.candidate.ID(), updatedStateID).Return(nil).Once()
		s.epochStateDB.On("BatchStore", w, updatedStateID, updatedState.MinEpochStateEntry).Return(nil).Once()
		s.mutator.On("SetEpochStateID", updatedStateID).Return(nil).Once()

		dbUpdates, err := s.stateMachine.Build()
		require.NoError(s.T(), err)

		// Provide the blockID and execute the resulting `dbUpdates`. Thereby, the expected mock methods should be called,
		// which is asserted by the testify framework. The lock context proof is passed to verify that the BatchIndex
		// operation receives the proper lock context as required by the storage layer.
		blockID := s.candidate.ID()
		return dbUpdates.Execute(lctx, blockID, rw)
	})
	require.NoError(s.T(), err)
}

// TestEpochStateMachine_Constructor tests the behavior of the EpochStateMachine constructor.
// Specifically, we test the scenario, where the EpochCommit Service Event is still missing
// by the time we cross the `FinalizationSafetyThreshold`. We expect the constructor to select the
// appropriate internal state machine constructor (HappyPathStateMachine before the threshold
// and FallbackStateMachine when reaching or exceeding the view threshold).
// Any exceptions encountered when constructing the internal state machines should be passed up.
func (s *EpochStateMachineSuite) TestEpochStateMachine_Constructor() {
	s.Run("EpochStaking phase", func() {
		// Since we are before the epoch commitment deadline, we should instantiate a happy-path state machine
		s.Run("before commitment deadline", func() {
			happyPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())
			// expect to be called
			happyPathStateMachineFactory.On("Execute", s.candidate.View, s.parentEpochState).
				Return(s.happyPathStateMachine, nil).Once()
			// don't expect to be called
			fallbackPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())

			candidate := unittest.BlockHeaderFixture(unittest.HeaderWithView(s.parentEpochState.CurrentEpochSetup.FirstView + 1))
			stateMachine, err := epochs.NewEpochStateMachine(
				candidate.View,
				candidate.ParentID,
				s.setupsDB,
				s.commitsDB,
				s.epochStateDB,
				s.parentState,
				s.mutator,
				happyPathStateMachineFactory.Execute,
				fallbackPathStateMachineFactory.Execute,
			)
			require.NoError(s.T(), err)
			assert.NotNil(s.T(), stateMachine)
		})
		// Since we are past the epoch commitment deadline, and have not entered the EpochCommitted
		// phase, we should use the epoch fallback state machine.
		s.Run("past commitment deadline", func() {
			// don't expect to be called
			happyPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())
			// expect to be called
			fallbackPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())

			candidate := unittest.BlockHeaderFixture(unittest.HeaderWithView(s.parentEpochState.CurrentEpochSetup.FinalView - 1))
			fallbackPathStateMachineFactory.On("Execute", candidate.View, s.parentEpochState).
				Return(s.happyPathStateMachine, nil).Once()
			stateMachine, err := epochs.NewEpochStateMachine(
				candidate.View,
				candidate.ParentID,
				s.setupsDB,
				s.commitsDB,
				s.epochStateDB,
				s.parentState,
				s.mutator,
				happyPathStateMachineFactory.Execute,
				fallbackPathStateMachineFactory.Execute,
			)
			require.NoError(s.T(), err)
			assert.NotNil(s.T(), stateMachine)
		})
	})

	s.Run("EpochSetup phase", func() {
		s.parentEpochState = unittest.EpochStateFixture(unittest.WithNextEpochProtocolState())
		s.parentEpochState.NextEpochCommit = nil
		s.parentEpochState.NextEpoch.CommitID = flow.ZeroID

		// Since we are before the epoch commitment deadline, we should instantiate a happy-path state machine
		s.Run("before commitment deadline", func() {
			happyPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())
			// don't expect to be called
			fallbackPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())

			candidate := unittest.BlockHeaderFixture(unittest.HeaderWithView(s.parentEpochState.CurrentEpochSetup.FirstView + 1))
			// expect to be called
			happyPathStateMachineFactory.On("Execute", candidate.View, s.parentEpochState).
				Return(s.happyPathStateMachine, nil).Once()
			stateMachine, err := epochs.NewEpochStateMachine(
				candidate.View,
				candidate.ParentID,
				s.setupsDB,
				s.commitsDB,
				s.epochStateDB,
				s.parentState,
				s.mutator,
				happyPathStateMachineFactory.Execute,
				fallbackPathStateMachineFactory.Execute,
			)
			require.NoError(s.T(), err)
			assert.NotNil(s.T(), stateMachine)
		})
		// Since we are past the epoch commitment deadline, and have not entered the EpochCommitted
		// phase, we should use the epoch fallback state machine.
		s.Run("past commitment deadline", func() {
			// don't expect to be called
			happyPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())
			fallbackPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())

			candidate := unittest.BlockHeaderFixture(unittest.HeaderWithView(s.parentEpochState.CurrentEpochSetup.FinalView - 1))
			// expect to be called
			fallbackPathStateMachineFactory.On("Execute", candidate.View, s.parentEpochState).
				Return(s.happyPathStateMachine, nil).Once()
			stateMachine, err := epochs.NewEpochStateMachine(
				candidate.View,
				candidate.ParentID,
				s.setupsDB,
				s.commitsDB,
				s.epochStateDB,
				s.parentState,
				s.mutator,
				happyPathStateMachineFactory.Execute,
				fallbackPathStateMachineFactory.Execute,
			)
			require.NoError(s.T(), err)
			assert.NotNil(s.T(), stateMachine)
		})
	})

	s.Run("EpochCommitted phase", func() {
		s.parentEpochState = unittest.EpochStateFixture(unittest.WithNextEpochProtocolState())
		// Since we are before the epoch commitment deadline, we should instantiate a happy-path state machine
		s.Run("before commitment deadline", func() {
			happyPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())
			// expect to be called
			happyPathStateMachineFactory.On("Execute", s.candidate.View, s.parentEpochState).
				Return(s.happyPathStateMachine, nil).Once()
			// don't expect to be called
			fallbackPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())

			candidate := unittest.BlockHeaderFixture(unittest.HeaderWithView(s.parentEpochState.CurrentEpochSetup.FirstView + 1))
			stateMachine, err := epochs.NewEpochStateMachine(
				candidate.View,
				candidate.ParentID,
				s.setupsDB,
				s.commitsDB,
				s.epochStateDB,
				s.parentState,
				s.mutator,
				happyPathStateMachineFactory.Execute,
				fallbackPathStateMachineFactory.Execute,
			)
			require.NoError(s.T(), err)
			assert.NotNil(s.T(), stateMachine)
		})
		// Despite being past the epoch commitment deadline, since we are in the EpochCommitted phase
		// already, we should proceed with the happy-path state machine
		s.Run("past commitment deadline", func() {
			happyPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())
			// don't expect to be called
			fallbackPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())

			candidate := unittest.BlockHeaderFixture(unittest.HeaderWithView(s.parentEpochState.CurrentEpochSetup.FinalView - 1))
			// expect to be called
			happyPathStateMachineFactory.On("Execute", candidate.View, s.parentEpochState).
				Return(s.happyPathStateMachine, nil).Once()
			stateMachine, err := epochs.NewEpochStateMachine(
				candidate.View,
				candidate.ParentID,
				s.setupsDB,
				s.commitsDB,
				s.epochStateDB,
				s.parentState,
				s.mutator,
				happyPathStateMachineFactory.Execute,
				fallbackPathStateMachineFactory.Execute,
			)
			require.NoError(s.T(), err)
			assert.NotNil(s.T(), stateMachine)
		})
	})

	// if a state machine constructor returns an error, the stateMutator constructor should fail
	// and propagate the error to the caller
	s.Run("state machine constructor returns error", func() {
		s.Run("happy-path", func() {
			exception := irrecoverable.NewExceptionf("exception")
			happyPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())
			happyPathStateMachineFactory.On("Execute", s.candidate.View, s.parentEpochState).Return(nil, exception).Once()
			fallbackPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())

			stateMachine, err := epochs.NewEpochStateMachine(
				s.candidate.View,
				s.candidate.ParentID,
				s.setupsDB,
				s.commitsDB,
				s.epochStateDB,
				s.parentState,
				s.mutator,
				happyPathStateMachineFactory.Execute,
				fallbackPathStateMachineFactory.Execute,
			)
			assert.ErrorIs(s.T(), err, exception)
			assert.Nil(s.T(), stateMachine)
		})
		s.Run("epoch-fallback", func() {
			s.parentEpochState.EpochFallbackTriggered = true // ensure we use epoch-fallback state machine
			exception := irrecoverable.NewExceptionf("exception")
			happyPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())
			fallbackPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())
			fallbackPathStateMachineFactory.On("Execute", s.candidate.View, s.parentEpochState).Return(nil, exception).Once()

			stateMachine, err := epochs.NewEpochStateMachine(
				s.candidate.View,
				s.candidate.ParentID,
				s.setupsDB,
				s.commitsDB,
				s.epochStateDB,
				s.parentState,
				s.mutator,
				happyPathStateMachineFactory.Execute,
				fallbackPathStateMachineFactory.Execute,
			)
			assert.ErrorIs(s.T(), err, exception)
			assert.Nil(s.T(), stateMachine)
		})
	})
}

// TestEvolveState_InvalidEpochSetup tests that hierarchical state machine rejects invalid epoch setup events
// (indicated by `InvalidServiceEventError` sentinel error) and replaces the happy path state machine with the
// fallback state machine. Errors other than `InvalidServiceEventError` should be bubbled up as exceptions.
func (s *EpochStateMachineSuite) TestEvolveState_InvalidEpochSetup() {
	s.Run("invalid-epoch-setup", func() {
		happyPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())
		happyPathStateMachineFactory.On("Execute", s.candidate.View, s.parentEpochState).Return(s.happyPathStateMachine, nil).Once()
		fallbackPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())
		stateMachine, err := epochs.NewEpochStateMachine(
			s.candidate.View,
			s.candidate.ParentID,
			s.setupsDB,
			s.commitsDB,
			s.epochStateDB,
			s.parentState,
			s.mutator,
			happyPathStateMachineFactory.Execute,
			fallbackPathStateMachineFactory.Execute,
		)
		require.NoError(s.T(), err)

		epochSetup := unittest.EpochSetupFixture()

		s.happyPathStateMachine.On("ParentState").Return(s.parentEpochState)
		s.happyPathStateMachine.On("ProcessEpochSetup", epochSetup).
			Return(false, protocol.NewInvalidServiceEventErrorf("")).Once()

		fallbackStateMachine := mock.NewStateMachine(s.T())
		fallbackStateMachine.On("ParentState").Return(s.parentEpochState)
		fallbackStateMachine.On("ProcessEpochSetup", epochSetup).Return(false, nil).Once()
		fallbackPathStateMachineFactory.On("Execute", s.candidate.View, s.parentEpochState).Return(fallbackStateMachine, nil).Once()

		err = stateMachine.EvolveState([]flow.ServiceEvent{epochSetup.ServiceEvent()})
		require.NoError(s.T(), err)
	})
	s.Run("process-epoch-setup-exception", func() {
		epochSetup := unittest.EpochSetupFixture()

		exception := errors.New("exception")
		s.happyPathStateMachine.On("ProcessEpochSetup", epochSetup).Return(false, exception).Once()

		err := s.stateMachine.EvolveState([]flow.ServiceEvent{epochSetup.ServiceEvent()})
		require.Error(s.T(), err)
		require.False(s.T(), protocol.IsInvalidServiceEventError(err))
	})
}

// TestEvolveState_InvalidEpochCommit tests that hierarchical state machine rejects invalid epoch commit events
// (indicated by `InvalidServiceEventError` sentinel error) and replaces the happy path state machine with the
// fallback state machine. Errors other than `InvalidServiceEventError` should be bubbled up as exceptions.
func (s *EpochStateMachineSuite) TestEvolveState_InvalidEpochCommit() {
	s.Run("invalid-epoch-commit", func() {
		happyPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())
		happyPathStateMachineFactory.On("Execute", s.candidate.View, s.parentEpochState).Return(s.happyPathStateMachine, nil).Once()
		fallbackPathStateMachineFactory := mock_interfaces.NewStateMachineFactoryMethod(s.T())
		stateMachine, err := epochs.NewEpochStateMachine(
			s.candidate.View,
			s.candidate.ParentID,
			s.setupsDB,
			s.commitsDB,
			s.epochStateDB,
			s.parentState,
			s.mutator,
			happyPathStateMachineFactory.Execute,
			fallbackPathStateMachineFactory.Execute,
		)
		require.NoError(s.T(), err)

		epochCommit := unittest.EpochCommitFixture()

		s.happyPathStateMachine.On("ParentState").Return(s.parentEpochState)
		s.happyPathStateMachine.On("ProcessEpochCommit", epochCommit).
			Return(false, protocol.NewInvalidServiceEventErrorf("")).Once()

		fallbackStateMachine := mock.NewStateMachine(s.T())
		fallbackStateMachine.On("ParentState").Return(s.parentEpochState)
		fallbackStateMachine.On("ProcessEpochCommit", epochCommit).Return(false, nil).Once()
		fallbackPathStateMachineFactory.On("Execute", s.candidate.View, s.parentEpochState).Return(fallbackStateMachine, nil).Once()

		err = stateMachine.EvolveState([]flow.ServiceEvent{epochCommit.ServiceEvent()})
		require.NoError(s.T(), err)
	})
	s.Run("process-epoch-commit-exception", func() {
		epochCommit := unittest.EpochCommitFixture()

		exception := errors.New("exception")
		s.happyPathStateMachine.On("ProcessEpochCommit", epochCommit).Return(false, exception).Once()

		err := s.stateMachine.EvolveState([]flow.ServiceEvent{epochCommit.ServiceEvent()})
		require.Error(s.T(), err)
		require.False(s.T(), protocol.IsInvalidServiceEventError(err))
	})
}

// TestEvolveStateTransitionToNextEpoch tests that EpochStateMachine transitions to the next epoch
// when the epoch has been committed, and we are at the first block of the next epoch.
func (s *EpochStateMachineSuite) TestEvolveStateTransitionToNextEpoch() {
	parentState := unittest.EpochStateFixture(unittest.WithNextEpochProtocolState())
	s.happyPathStateMachine.On("ParentState").Unset()
	s.happyPathStateMachine.On("ParentState").Return(parentState)
	// we are at the first block of the next epoch
	s.happyPathStateMachine.On("View").Return(parentState.CurrentEpochSetup.FinalView + 1)
	s.happyPathStateMachine.On("TransitionToNextEpoch").Return(nil).Once()
	err := s.stateMachine.EvolveState(nil)
	require.NoError(s.T(), err)
}

// TestEvolveStateTransitionToNextEpoch_Error tests that error that has been
// observed when transitioning to the next epoch and propagated to the caller.
func (s *EpochStateMachineSuite) TestEvolveStateTransitionToNextEpoch_Error() {
	parentState := unittest.EpochStateFixture(unittest.WithNextEpochProtocolState())
	s.happyPathStateMachine.On("ParentState").Unset()
	s.happyPathStateMachine.On("ParentState").Return(parentState)
	// we are at the first block of the next epoch
	s.happyPathStateMachine.On("View").Return(parentState.CurrentEpochSetup.FinalView + 1)
	exception := errors.New("exception")
	s.happyPathStateMachine.On("TransitionToNextEpoch").Return(exception).Once()
	err := s.stateMachine.EvolveState(nil)
	require.Error(s.T(), err, exception)
	require.ErrorContains(s.T(), err, "[exception!]")
	require.False(s.T(), protocol.IsInvalidServiceEventError(err))
}

// TestEvolveState_EventsAreFiltered tests that EpochStateMachine filters out all events that are not expected.
func (s *EpochStateMachineSuite) TestEvolveState_EventsAreFiltered() {
	err := s.stateMachine.EvolveState([]flow.ServiceEvent{
		unittest.ProtocolStateVersionUpgradeFixture().ServiceEvent(),
	})
	require.NoError(s.T(), err)
}

// TestEvolveStateTransitionToNextEpoch_WithInvalidStateTransition tests that EpochStateMachine transitions to the next epoch
// if an invalid state transition has been detected in a block which triggers transitioning to the next epoch.
// In such situation, we still need to enter the next epoch (because it has already been committed), but persist in the
// state that we have entered Epoch fallback mode (`flow.MinEpochStateEntry.EpochFallbackTriggered` is set to `true`).
// This test ensures that we don't drop previously committed next epoch.
func (s *EpochStateMachineSuite) TestEvolveStateTransitionToNextEpoch_WithInvalidStateTransition() {
	s.parentEpochState = unittest.EpochStateFixture(unittest.WithNextEpochProtocolState())
	s.candidate.View = s.parentEpochState.NextEpochSetup.FirstView
	happyPathTelemetry := protocol_statemock.NewStateMachineTelemetryConsumer(s.T())
	fallbackPathTelemetry := protocol_statemock.NewStateMachineTelemetryConsumer(s.T())
	happyPathTelemetryFactory := protocol_mock_interfaces.NewStateMachineEventsTelemetryFactory(s.T())
	fallbackTelemetryFactory := protocol_mock_interfaces.NewStateMachineEventsTelemetryFactory(s.T())
	happyPathTelemetryFactory.On("Execute", s.candidate.View).Return(happyPathTelemetry).Once()
	fallbackTelemetryFactory.On("Execute", s.candidate.View).Return(fallbackPathTelemetry).Once()
	stateMachine, err := epochs.NewEpochStateMachineFactory(
		s.setupsDB,
		s.commitsDB,
		s.epochStateDB,
		happyPathTelemetryFactory.Execute,
		fallbackTelemetryFactory.Execute,
	).Create(s.candidate.View, s.candidate.ParentID, s.parentState, s.mutator)
	require.NoError(s.T(), err)

	invalidServiceEvent := unittest.EpochSetupFixture()
	happyPathTelemetry.On("OnServiceEventReceived", invalidServiceEvent.ServiceEvent()).Return().Once()
	happyPathTelemetry.On("OnInvalidServiceEvent", invalidServiceEvent.ServiceEvent(), mocks.Anything).Return().Once()
	fallbackPathTelemetry.On("OnServiceEventReceived", invalidServiceEvent.ServiceEvent()).Return().Once()
	fallbackPathTelemetry.On("OnInvalidServiceEvent", invalidServiceEvent.ServiceEvent(), mocks.Anything).Return().Once()
	err = stateMachine.EvolveState([]flow.ServiceEvent{invalidServiceEvent.ServiceEvent()})
	require.NoError(s.T(), err)

	// Create a proper lock context proof for the BatchIndex operation
	err = unittest.WithLock(s.T(), s.lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
		s.epochStateDB.On("BatchIndex", lctx, mocks.Anything, s.candidate.ID(), mocks.Anything).Return(nil).Once()

		expectedEpochState := &flow.MinEpochStateEntry{
			PreviousEpoch:          s.parentEpochState.CurrentEpoch.Copy(),
			CurrentEpoch:           *s.parentEpochState.NextEpoch.Copy(),
			NextEpoch:              nil,
			EpochFallbackTriggered: true,
		}

		s.epochStateDB.On("BatchStore", mocks.Anything, expectedEpochState.ID(), expectedEpochState).Return(nil).Once()
		s.mutator.On("SetEpochStateID", expectedEpochState.ID()).Return().Once()

		dbOps, err := stateMachine.Build()
		require.NoError(s.T(), err)

		w := storagemock.NewWriter(s.T())
		rw := storagemock.NewReaderBatchWriter(s.T())
		rw.On("Writer").Return(w).Once() // called by epochStateDB.BatchStore

		// Storage operations are deferred, because block ID is not known when the block is newly constructed. Only at the
		// end after the block is fully constructed, its ID can be computed. We emulate this step here to verify that the
		// deferred `dbOps` have been correctly constructed. Thereby, the expected mock methods should be called,
		// which is asserted by the testify framework.
		blockID := s.candidate.ID()
		return dbOps.Execute(lctx, blockID, rw)
	})
	require.NoError(s.T(), err)

}
