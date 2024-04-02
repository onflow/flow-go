package epochs_test

import (
	"errors"
	"testing"

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
	protocol_statemock "github.com/onflow/flow-go/state/protocol/protocol_state/mock"
	"github.com/onflow/flow-go/storage/badger/transaction"
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
	epochStateDB                    *storagemock.ProtocolState
	setupsDB                        *storagemock.EpochSetups
	commitsDB                       *storagemock.EpochCommits
	globalParams                    *protocolmock.GlobalParams
	parentState                     *protocol_statemock.KVStoreReader
	parentEpochState                *flow.RichProtocolStateEntry
	mutator                         *protocol_statemock.KVStoreMutator
	happyPathStateMachine           *mock.StateMachine
	happyPathStateMachineFactory    *mock.StateMachineFactoryMethod
	fallbackPathStateMachineFactory *mock.StateMachineFactoryMethod
	candidate                       *flow.Header

	stateMachine *epochs.EpochStateMachine
}

func (s *EpochStateMachineSuite) SetupTest() {
	s.epochStateDB = storagemock.NewProtocolState(s.T())
	s.setupsDB = storagemock.NewEpochSetups(s.T())
	s.commitsDB = storagemock.NewEpochCommits(s.T())
	s.globalParams = protocolmock.NewGlobalParams(s.T())
	s.globalParams.On("EpochCommitSafetyThreshold").Return(uint64(1_000))
	s.parentState = protocol_statemock.NewKVStoreReader(s.T())
	s.parentEpochState = unittest.ProtocolStateFixture()
	s.mutator = protocol_statemock.NewKVStoreMutator(s.T())
	s.candidate = unittest.BlockHeaderFixture(unittest.HeaderWithView(s.parentEpochState.CurrentEpochSetup.FirstView + 1))
	s.happyPathStateMachine = mock.NewStateMachine(s.T())
	s.happyPathStateMachineFactory = mock.NewStateMachineFactoryMethod(s.T())
	s.fallbackPathStateMachineFactory = mock.NewStateMachineFactoryMethod(s.T())

	s.epochStateDB.On("ByBlockID", mocks.Anything).Return(func(_ flow.Identifier) *flow.RichProtocolStateEntry {
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
		s.candidate,
		s.globalParams,
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
	s.happyPathStateMachine.On("Build").Return(s.parentEpochState.ProtocolStateEntry, s.parentEpochState.ID(), false).Once()

	err := s.stateMachine.ProcessUpdate(nil)
	require.NoError(s.T(), err)

	indexTxDeferredUpdate := storagemock.NewDeferredDBUpdate(s.T())
	indexTxDeferredUpdate.On("Execute", mocks.Anything).Return(nil).Once()

	s.epochStateDB.On("Index", s.candidate.ID(), s.parentEpochState.ID()).Return(indexTxDeferredUpdate.Execute, nil).Once()
	s.mutator.On("SetEpochStateID", s.parentEpochState.ID()).Return(nil).Once()

	dbUpdates := s.stateMachine.Build()
	// in next loop we assert that we have received expected deferred db updates by executing them
	// and expecting that corresponding mock methods will be called
	tx := &transaction.Tx{}
	for _, dbUpdate := range dbUpdates {
		err := dbUpdate(tx)
		require.NoError(s.T(), err)
	}
}

// TestBuild_HappyPath tests that hierarchical epoch state machine maintains index of epoch states and commits
// as well as stores updated epoch state in respective storage when there were updates made to the epoch state.
// This test also ensures that updated state ID is committed in the KV store.
func (s *EpochStateMachineSuite) TestBuild_HappyPath() {
	s.happyPathStateMachine.On("ParentState").Return(s.parentEpochState)
	updatedState := unittest.ProtocolStateFixture().ProtocolStateEntry
	updatedStateID := updatedState.ID()
	s.happyPathStateMachine.On("Build").Return(updatedState, updatedStateID, true).Once()

	epochSetup := unittest.EpochSetupFixture()
	epochCommit := unittest.EpochCommitFixture()

	// expected both events to be processed
	s.happyPathStateMachine.On("ProcessEpochSetup", epochSetup).Return(true, nil).Once()
	s.happyPathStateMachine.On("ProcessEpochCommit", epochCommit).Return(true, nil).Once()

	// prepare a DB update for epoch setup
	storeEpochSetupTx := storagemock.NewDeferredDBUpdate(s.T())
	storeEpochSetupTx.On("Execute", mocks.Anything).Return(nil).Once()
	s.setupsDB.On("StoreTx", epochSetup).Return(storeEpochSetupTx.Execute, nil).Once()

	// prepare a DB update for epoch commit
	storeEpochCommitTx := storagemock.NewDeferredDBUpdate(s.T())
	storeEpochCommitTx.On("Execute", mocks.Anything).Return(nil).Once()
	s.commitsDB.On("StoreTx", epochCommit).Return(storeEpochCommitTx.Execute, nil).Once()

	err := s.stateMachine.ProcessUpdate([]flow.ServiceEvent{epochSetup.ServiceEvent(), epochCommit.ServiceEvent()})
	require.NoError(s.T(), err)

	// prepare a DB update for epoch state
	indexTxDeferredUpdate := storagemock.NewDeferredDBUpdate(s.T())
	indexTxDeferredUpdate.On("Execute", mocks.Anything).Return(nil).Once()
	storeTxDeferredUpdate := storagemock.NewDeferredDBUpdate(s.T())
	storeTxDeferredUpdate.On("Execute", mocks.Anything).Return(nil).Once()

	s.epochStateDB.On("Index", s.candidate.ID(), updatedStateID).Return(indexTxDeferredUpdate.Execute, nil).Once()
	s.epochStateDB.On("StoreTx", updatedStateID, updatedState).Return(storeTxDeferredUpdate.Execute, nil).Once()
	s.mutator.On("SetEpochStateID", updatedStateID).Return(nil).Once()

	dbUpdates := s.stateMachine.Build()
	// in next loop we assert that we have received expected deferred db updates by executing them
	// and expecting that corresponding mock methods will be called
	tx := &transaction.Tx{}
	for _, dbUpdate := range dbUpdates {
		err := dbUpdate(tx)
		require.NoError(s.T(), err)
	}
}

// TestEpochStateMachine_Constructor tests the behavior of the EpochStateMachine constructor.
// We expect the constructor to select the appropriate internal state machine constructor, and
// to handle (pass-through) exceptions from the internal state machine constructor.
func (s *EpochStateMachineSuite) TestEpochStateMachine_Constructor() {
	s.Run("EpochStaking phase", func() {
		// Since we are before the epoch commitment deadline, we should use the happy-path state machine
		s.Run("before commitment deadline", func() {
			happyPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())
			// expect to be called
			happyPathStateMachineFactory.On("Execute", s.candidate.View, s.parentEpochState).
				Return(s.happyPathStateMachine, nil).Once()
			// don't expect to be called
			fallbackPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())

			candidate := unittest.BlockHeaderFixture(unittest.HeaderWithView(s.parentEpochState.CurrentEpochSetup.FirstView + 1))
			stateMachine, err := epochs.NewEpochStateMachine(
				candidate,
				s.globalParams,
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
			happyPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())
			// expect to be called
			fallbackPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())

			candidate := unittest.BlockHeaderFixture(unittest.HeaderWithView(s.parentEpochState.CurrentEpochSetup.FinalView - 1))
			fallbackPathStateMachineFactory.On("Execute", candidate.View, s.parentEpochState).
				Return(s.happyPathStateMachine, nil).Once()
			stateMachine, err := epochs.NewEpochStateMachine(
				candidate,
				s.globalParams,
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
		s.parentEpochState = unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())
		s.parentEpochState.NextEpochCommit = nil
		s.parentEpochState.NextEpoch.CommitID = flow.ZeroID

		// Since we are before the epoch commitment deadline, we should use the happy-path state machine
		s.Run("before commitment deadline", func() {
			happyPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())
			// don't expect to be called
			fallbackPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())

			candidate := unittest.BlockHeaderFixture(unittest.HeaderWithView(s.parentEpochState.CurrentEpochSetup.FirstView + 1))
			// expect to be called
			happyPathStateMachineFactory.On("Execute", candidate.View, s.parentEpochState).
				Return(s.happyPathStateMachine, nil).Once()
			stateMachine, err := epochs.NewEpochStateMachine(
				candidate,
				s.globalParams,
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
			happyPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())
			fallbackPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())

			candidate := unittest.BlockHeaderFixture(unittest.HeaderWithView(s.parentEpochState.CurrentEpochSetup.FinalView - 1))
			// expect to be called
			fallbackPathStateMachineFactory.On("Execute", candidate.View, s.parentEpochState).
				Return(s.happyPathStateMachine, nil).Once()
			stateMachine, err := epochs.NewEpochStateMachine(
				candidate,
				s.globalParams,
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
		s.parentEpochState = unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())
		// Since we are before the epoch commitment deadline, we should use the happy-path state machine
		s.Run("before commitment deadline", func() {
			happyPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())
			// expect to be called
			happyPathStateMachineFactory.On("Execute", s.candidate.View, s.parentEpochState).
				Return(s.happyPathStateMachine, nil).Once()
			// don't expect to be called
			fallbackPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())

			candidate := unittest.BlockHeaderFixture(unittest.HeaderWithView(s.parentEpochState.CurrentEpochSetup.FirstView + 1))
			stateMachine, err := epochs.NewEpochStateMachine(
				candidate,
				s.globalParams,
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
			happyPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())
			// don't expect to be called
			fallbackPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())

			candidate := unittest.BlockHeaderFixture(unittest.HeaderWithView(s.parentEpochState.CurrentEpochSetup.FinalView - 1))
			// expect to be called
			happyPathStateMachineFactory.On("Execute", candidate.View, s.parentEpochState).
				Return(s.happyPathStateMachine, nil).Once()
			stateMachine, err := epochs.NewEpochStateMachine(
				candidate,
				s.globalParams,
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
			happyPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())
			happyPathStateMachineFactory.On("Execute", s.candidate.View, s.parentEpochState).Return(nil, exception).Once()
			fallbackPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())

			stateMachine, err := epochs.NewEpochStateMachine(
				s.candidate,
				s.globalParams,
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
			s.parentEpochState.InvalidEpochTransitionAttempted = true // ensure we use epoch-fallback state machine
			exception := irrecoverable.NewExceptionf("exception")
			happyPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())
			fallbackPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())
			fallbackPathStateMachineFactory.On("Execute", s.candidate.View, s.parentEpochState).Return(nil, exception).Once()

			stateMachine, err := epochs.NewEpochStateMachine(
				s.candidate,
				s.globalParams,
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

// TestProcessUpdate_InvalidEpochSetup tests that hierarchical state machine rejects invalid epoch setup event and
// replaces the happy path state machine with the fallback state machine.
func (s *EpochStateMachineSuite) TestProcessUpdate_InvalidEpochSetup() {
	s.Run("invalid-epoch-setup", func() {
		happyPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())
		happyPathStateMachineFactory.On("Execute", s.candidate.View, s.parentEpochState).Return(s.happyPathStateMachine, nil).Once()
		fallbackPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())
		stateMachine, err := epochs.NewEpochStateMachine(
			s.candidate,
			s.globalParams,
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
		fallbackStateMachine.On("ProcessEpochSetup", epochSetup).Return(false, nil).Once()
		fallbackPathStateMachineFactory.On("Execute", s.candidate.View, s.parentEpochState).Return(fallbackStateMachine, nil).Once()

		err = stateMachine.ProcessUpdate([]flow.ServiceEvent{epochSetup.ServiceEvent()})
		require.NoError(s.T(), err)
	})
	s.Run("process-epoch-setup-exception", func() {
		epochSetup := unittest.EpochSetupFixture()

		exception := errors.New("exception")
		s.happyPathStateMachine.On("ProcessEpochSetup", epochSetup).Return(false, exception).Once()

		err := s.stateMachine.ProcessUpdate([]flow.ServiceEvent{epochSetup.ServiceEvent()})
		require.Error(s.T(), err)
		require.False(s.T(), protocol.IsInvalidServiceEventError(err))
	})
}

// TestProcessUpdate_InvalidEpochCommit tests that hierarchical state machine rejects invalid epoch commit event and
// replaces the happy path state machine with the fallback state machine.
func (s *EpochStateMachineSuite) TestProcessUpdate_InvalidEpochCommit() {
	s.Run("invalid-epoch-commit", func() {
		happyPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())
		happyPathStateMachineFactory.On("Execute", s.candidate.View, s.parentEpochState).Return(s.happyPathStateMachine, nil).Once()
		fallbackPathStateMachineFactory := mock.NewStateMachineFactoryMethod(s.T())
		stateMachine, err := epochs.NewEpochStateMachine(
			s.candidate,
			s.globalParams,
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
		fallbackStateMachine.On("ProcessEpochCommit", epochCommit).Return(false, nil).Once()
		fallbackPathStateMachineFactory.On("Execute", s.candidate.View, s.parentEpochState).Return(fallbackStateMachine, nil).Once()

		err = stateMachine.ProcessUpdate([]flow.ServiceEvent{epochCommit.ServiceEvent()})
		require.NoError(s.T(), err)
	})
	s.Run("process-epoch-commit-exception", func() {
		epochCommit := unittest.EpochCommitFixture()

		exception := errors.New("exception")
		s.happyPathStateMachine.On("ProcessEpochCommit", epochCommit).Return(false, exception).Once()

		err := s.stateMachine.ProcessUpdate([]flow.ServiceEvent{epochCommit.ServiceEvent()})
		require.Error(s.T(), err)
		require.False(s.T(), protocol.IsInvalidServiceEventError(err))
	})
}

// TestProcessUpdateTransitionToNextEpoch tests that EpochStateMachine transitions to the next epoch
// when the epoch has been committed, and we are at the first block of the next epoch.
func (s *EpochStateMachineSuite) TestProcessUpdateTransitionToNextEpoch() {
	parentState := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())
	s.happyPathStateMachine.On("ParentState").Unset()
	s.happyPathStateMachine.On("ParentState").Return(parentState)
	// we are at the first block of the next epoch
	s.happyPathStateMachine.On("View").Return(parentState.CurrentEpochSetup.FinalView + 1)
	s.happyPathStateMachine.On("TransitionToNextEpoch").Return(nil).Once()
	err := s.stateMachine.ProcessUpdate(nil)
	require.NoError(s.T(), err)
}

// TestProcessUpdateTransitionToNextEpoch_Error tests that error that has been
// observed when transitioning to the next epoch and propagated to the caller.
func (s *EpochStateMachineSuite) TestProcessUpdateTransitionToNextEpoch_Error() {
	parentState := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())
	s.happyPathStateMachine.On("ParentState").Unset()
	s.happyPathStateMachine.On("ParentState").Return(parentState)
	// we are at the first block of the next epoch
	s.happyPathStateMachine.On("View").Return(parentState.CurrentEpochSetup.FinalView + 1)
	exception := errors.New("exception")
	s.happyPathStateMachine.On("TransitionToNextEpoch").Return(exception).Once()
	err := s.stateMachine.ProcessUpdate(nil)
	require.ErrorIs(s.T(), err, exception)
	require.False(s.T(), protocol.IsInvalidServiceEventError(err))
}

// TestProcessUpdate_EventsAreFiltered tests that EpochStateMachine filters out all events that are not expected.
func (s *EpochStateMachineSuite) TestProcessUpdate_EventsAreFiltered() {
	err := s.stateMachine.ProcessUpdate([]flow.ServiceEvent{
		unittest.ProtocolStateVersionUpgradeFixture().ServiceEvent(),
	})
	require.NoError(s.T(), err)
}
