package epochs_test

import (
	"errors"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol/protocol_state/epochs"
	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/state/protocol/protocol_state/epochs/mock"
	protocol_statemock "github.com/onflow/flow-go/state/protocol/protocol_state/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEpochStateMachine(t *testing.T) {
	suite.Run(t, new(EpochStateMachineSuite))
}

// EpochStateMachineSuite is a dedicated test suite for testing hierarchical epoch state machine.
// All needed dependencies are mocked, including KV store as a whole, and all the necessary storages.
type EpochStateMachineSuite struct {
	suite.Suite
	epochStateDB                    *storagemock.ProtocolState
	headersDB                       *storagemock.Headers
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

// TestStateMutator_Constructor tests the behaviour of the StateMutator constructor.
// We expect the constructor to select the appropriate state machine constructor, and
// to handle (pass-through) exceptions from the state machine constructor.
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

// TestApplyServiceEventsTransitionToNextEpoch tests that EpochStateMachine transitions to the next epoch
// when the epoch has been committed, and we are at the first block of the next epoch.
func (s *EpochStateMachineSuite) TestApplyServiceEventsTransitionToNextEpoch() {
	parentState := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())
	s.happyPathStateMachine.On("ParentState").Return(parentState)
	// we are at the first block of the next epoch
	s.happyPathStateMachine.On("View").Return(parentState.CurrentEpochSetup.FinalView + 1)
	s.happyPathStateMachine.On("TransitionToNextEpoch").Return(nil).Once()
	err := s.stateMachine.ProcessUpdate(nil)
	require.NoError(s.T(), err)
}

// TestApplyServiceEventsTransitionToNextEpoch_Error tests that error that has been
// observed when transitioning to the next epoch and propagated to the caller.
func (s *EpochStateMachineSuite) TestApplyServiceEventsTransitionToNextEpoch_Error() {
	parentState := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())

	s.happyPathStateMachine.On("ParentState").Return(parentState)
	// we are at the first block of the next epoch
	s.happyPathStateMachine.On("View").Return(parentState.CurrentEpochSetup.FinalView + 1)
	exception := errors.New("exception")
	s.happyPathStateMachine.On("TransitionToNextEpoch").Return(exception).Once()
	err := s.stateMachine.ProcessUpdate(nil)
	require.ErrorIs(s.T(), err, exception)
	require.False(s.T(), protocol.IsInvalidServiceEventError(err))
}
