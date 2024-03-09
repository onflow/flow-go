package state

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/state/protocol/protocol_state/epochs"
	protocolstatemock "github.com/onflow/flow-go/state/protocol/protocol_state/mock"
	"github.com/onflow/flow-go/storage/badger/transaction"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/rand"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProtocolStateMutator(t *testing.T) {
	suite.Run(t, new(StateMutatorSuite))
}

type StateMutatorSuite struct {
	suite.Suite
	protocolStateDB *storagemock.ProtocolState
	headersDB       *storagemock.Headers
	resultsDB       *storagemock.ExecutionResults
	setupsDB        *storagemock.EpochSetups
	commitsDB       *storagemock.EpochCommits
	globalParams    *protocolmock.GlobalParams
	parentState     *flow.RichProtocolStateEntry
	stateMachine    *protocolstatemock.ProtocolStateMachine
	candidateView   uint64

	mutator *stateMutator
}

func (s *StateMutatorSuite) SetupTest() {
	s.protocolStateDB = storagemock.NewProtocolState(s.T())
	s.headersDB = storagemock.NewHeaders(s.T())
	s.resultsDB = storagemock.NewExecutionResults(s.T())
	s.setupsDB = storagemock.NewEpochSetups(s.T())
	s.commitsDB = storagemock.NewEpochCommits(s.T())
	s.globalParams = protocolmock.NewGlobalParams(s.T())
	s.globalParams.On("EpochCommitSafetyThreshold").Return(uint64(1_000))
	s.parentState = unittest.ProtocolStateFixture()
	s.candidateView = s.parentState.CurrentEpochSetup.FirstView + 1
	s.stateMachine = protocolstatemock.NewProtocolStateMachine(s.T())

	var err error
	s.mutator, err = newStateMutator(
		s.headersDB,
		s.resultsDB,
		s.setupsDB,
		s.commitsDB,
		s.globalParams,
		s.candidateView,
		s.parentState,
		func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
			return s.stateMachine, nil
		},
		func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
			require.Fail(s.T(), "entering epoch fallback is not expected")
			return nil, fmt.Errorf("not expecting epoch fallback")
		},
	)
	require.NoError(s.T(), err)
}

// TestOnHappyPathNoDbChanges tests that stateMutator doesn't cache any db updates when there are no changes.
func (s *StateMutatorSuite) TestOnHappyPathNoDbChanges() {
	parentState := unittest.ProtocolStateFixture()
	s.stateMachine.On("ParentState").Return(parentState)
	s.stateMachine.On("Build").Return(parentState.ProtocolStateEntry, parentState.ID(), false)
	err := s.mutator.ApplyServiceEventsFromValidatedSeals([]*flow.Seal{})
	require.NoError(s.T(), err)
	hasChanges, updatedState, updatedStateID, dbUpdates := s.mutator.Build()
	require.False(s.T(), hasChanges)
	require.Equal(s.T(), parentState.ProtocolStateEntry, updatedState)
	require.Equal(s.T(), parentState.ID(), updatedStateID)
	require.Empty(s.T(), dbUpdates)
}

// TestHappyPathWithDbChanges tests that `stateMutator` returns cached db updates when building protocol state after applying service events.
// Whenever `stateMutator` successfully processes an epoch setup or epoch commit event, it has to create a deferred db update to store the event.
// Deferred db updates are cached in `stateMutator` and returned when building protocol state when calling `Build`.
func (s *StateMutatorSuite) TestHappyPathWithDbChanges() {
	parentState := unittest.ProtocolStateFixture()
	s.stateMachine.On("ParentState").Return(parentState)
	s.stateMachine.On("Build").Return(unittest.ProtocolStateFixture().ProtocolStateEntry,
		unittest.IdentifierFixture(), true)

	epochSetup := unittest.EpochSetupFixture()
	epochCommit := unittest.EpochCommitFixture()
	result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
		result.ServiceEvents = []flow.ServiceEvent{epochSetup.ServiceEvent(), epochCommit.ServiceEvent()}
	})

	block := unittest.BlockHeaderFixture()
	seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(block.ID()))
	s.headersDB.On("ByBlockID", seal.BlockID).Return(block, nil)
	s.resultsDB.On("ByID", seal.ResultID).Return(result, nil)

	epochSetupStored := mock.Mock{}
	epochSetupStored.On("EpochSetupStored").Return()
	s.stateMachine.On("ProcessEpochSetup", epochSetup).Return(true, nil).Once()
	s.setupsDB.On("StoreTx", epochSetup).Return(func(*transaction.Tx) error {
		epochSetupStored.MethodCalled("EpochSetupStored")
		return nil
	}).Once()

	epochCommitStored := mock.Mock{}
	epochCommitStored.On("EpochCommitStored").Return()
	s.stateMachine.On("ProcessEpochCommit", epochCommit).Return(true, nil).Once()
	s.commitsDB.On("StoreTx", epochCommit).Return(func(*transaction.Tx) error {
		epochCommitStored.MethodCalled("EpochCommitStored")
		return nil
	}).Once()

	err := s.mutator.ApplyServiceEventsFromValidatedSeals([]*flow.Seal{seal})
	require.NoError(s.T(), err)

	_, _, _, dbUpdates := s.mutator.Build()
	// in next loop we assert that we have received expected deferred db updates by executing them
	// and expecting that corresponding mock methods will be called
	tx := &transaction.Tx{}
	for _, dbUpdate := range dbUpdates {
		err := dbUpdate(tx)
		require.NoError(s.T(), err)
	}
	// make sure that mock methods were indeed called
	epochSetupStored.AssertExpectations(s.T())
	epochCommitStored.AssertExpectations(s.T())
}

// TestStateMutator_Constructor tests the behaviour of the StateMutator constructor.
// We expect the constructor to select the appropriate state machine constructor, and
// to handle (pass-through) exceptions from the state machine constructor.
func (s *StateMutatorSuite) TestStateMutator_Constructor() {
	s.Run("EpochStaking phase", func() {
		// Since we are before the epoch commitment deadline, we should use the happy-path state machine
		s.Run("before commitment deadline", func() {
			expectedConstructorCalled := false
			s.candidateView = s.parentState.CurrentEpochSetup.FirstView + 1
			mutator, err := newStateMutator(s.headersDB, s.resultsDB, s.setupsDB, s.commitsDB, s.globalParams, s.candidateView, s.parentState,
				func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
					expectedConstructorCalled = true // expect happy-path constructor
					return s.stateMachine, nil
				},
				func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
					s.T().Fail()
					return s.stateMachine, nil
				},
			)
			require.NoError(s.T(), err)
			assert.NotNil(s.T(), mutator)
			assert.True(s.T(), expectedConstructorCalled)
		})
		// Since we are past the epoch commitment deadline, and have not entered the EpochCommitted
		// phase, we should use the epoch fallback state machine.
		s.Run("past commitment deadline", func() {
			expectedConstructorCalled := false
			s.candidateView = s.parentState.CurrentEpochSetup.FinalView - 1
			mutator, err := newStateMutator(s.headersDB, s.resultsDB, s.setupsDB, s.commitsDB, s.globalParams, s.candidateView, s.parentState,
				func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
					s.T().Fail()
					return s.stateMachine, nil
				},
				func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
					expectedConstructorCalled = true // expect epoch-fallback state machine
					return s.stateMachine, nil
				},
			)
			require.NoError(s.T(), err)
			assert.NotNil(s.T(), mutator)
			assert.True(s.T(), expectedConstructorCalled)
		})
	})

	s.Run("EpochSetup phase", func() {
		s.parentState = unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())
		s.parentState.NextEpochCommit = nil
		s.parentState.NextEpoch.CommitID = flow.ZeroID

		// Since we are before the epoch commitment deadline, we should use the happy-path state machine
		s.Run("before commitment deadline", func() {
			expectedConstructorCalled := false
			s.candidateView = s.parentState.CurrentEpochSetup.FirstView + 1
			mutator, err := newStateMutator(s.headersDB, s.resultsDB, s.setupsDB, s.commitsDB, s.globalParams, s.candidateView, s.parentState,
				func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
					expectedConstructorCalled = true // expect happy-path constructor
					return s.stateMachine, nil
				},
				func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
					s.T().Fail()
					return s.stateMachine, nil
				},
			)
			require.NoError(s.T(), err)
			assert.NotNil(s.T(), mutator)
			assert.True(s.T(), expectedConstructorCalled)
		})
		// Since we are past the epoch commitment deadline, and have not entered the EpochCommitted
		// phase, we should use the epoch fallback state machine.
		s.Run("past commitment deadline", func() {
			expectedConstructorCalled := false
			s.candidateView = s.parentState.CurrentEpochSetup.FinalView - 1
			mutator, err := newStateMutator(s.headersDB, s.resultsDB, s.setupsDB, s.commitsDB, s.globalParams, s.candidateView, s.parentState,
				func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
					s.T().Fail()
					return s.stateMachine, nil
				},
				func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
					expectedConstructorCalled = true // expect epoch-fallback state machine
					return s.stateMachine, nil
				},
			)
			require.NoError(s.T(), err)
			assert.NotNil(s.T(), mutator)
			assert.True(s.T(), expectedConstructorCalled)
		})
	})

	s.Run("EpochCommitted phase", func() {
		s.parentState = unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())
		// Since we are before the epoch commitment deadline, we should use the happy-path state machine
		s.Run("before commitment deadline", func() {
			expectedConstructorCalled := false
			s.candidateView = s.parentState.CurrentEpochSetup.FirstView + 1
			mutator, err := newStateMutator(s.headersDB, s.resultsDB, s.setupsDB, s.commitsDB, s.globalParams, s.candidateView, s.parentState,
				func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
					expectedConstructorCalled = true // expect happy-path constructor
					return s.stateMachine, nil
				},
				func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
					s.T().Fail()
					return s.stateMachine, nil
				},
			)
			require.NoError(s.T(), err)
			assert.NotNil(s.T(), mutator)
			assert.True(s.T(), expectedConstructorCalled)
		})
		// Despite being past the epoch commitment deadline, since we are in the EpochCommitted phase
		// already, we should proceed with the happy-path state machine
		s.Run("past commitment deadline", func() {
			expectedConstructorCalled := false
			s.candidateView = s.parentState.CurrentEpochSetup.FinalView - 1
			mutator, err := newStateMutator(s.headersDB, s.resultsDB, s.setupsDB, s.commitsDB, s.globalParams, s.candidateView, s.parentState,
				func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
					expectedConstructorCalled = true // expect happy-path constructor
					return s.stateMachine, nil
				},
				func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
					s.T().Fail()
					return s.stateMachine, nil
				},
			)
			require.NoError(s.T(), err)
			assert.NotNil(s.T(), mutator)
			assert.True(s.T(), expectedConstructorCalled)
		})
	})

	// if a state machine constructor returns an error, the stateMutator constructor should fail
	// and propagate the error to the caller
	s.Run("state machine constructor returns error", func() {
		s.Run("happy-path", func() {
			exception := irrecoverable.NewExceptionf("exception")
			mutator, err := newStateMutator(s.headersDB, s.resultsDB, s.setupsDB, s.commitsDB, s.globalParams, s.candidateView, s.parentState,
				func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
					return nil, exception
				},
				func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
					s.T().Fail()
					return s.stateMachine, nil
				},
			)
			assert.Error(s.T(), err)
			assert.ErrorIs(s.T(), err, exception)
			assert.Nil(s.T(), mutator)
		})
		s.Run("epoch-fallback", func() {
			s.parentState.InvalidEpochTransitionAttempted = true // ensure we use epoch-fallback state machine
			exception := irrecoverable.NewExceptionf("exception")
			mutator, err := newStateMutator(s.headersDB, s.resultsDB, s.setupsDB, s.commitsDB, s.globalParams, s.candidateView, s.parentState,
				func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
					s.T().Fail()
					return s.stateMachine, nil
				},
				func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
					return nil, exception
				},
			)
			assert.Error(s.T(), err)
			assert.ErrorIs(s.T(), err, exception)
			assert.Nil(s.T(), mutator)
		})
	})
}

// TestApplyServiceEvents_InvalidEpochSetup tests that handleServiceEvents rejects invalid epoch setup event and sets
// InvalidEpochTransitionAttempted flag in protocol.ProtocolStateMachine.
func (s *StateMutatorSuite) TestApplyServiceEvents_InvalidEpochSetup() {
	s.Run("invalid-epoch-setup", func() {
		mutator, err := newStateMutator(
			s.headersDB,
			s.resultsDB,
			s.setupsDB,
			s.commitsDB,
			s.globalParams,
			s.candidateView,
			s.parentState,
			func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
				return s.stateMachine, nil
			},
			func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
				epochFallbackStateMachine := protocolstatemock.NewProtocolStateMachine(s.T())
				epochFallbackStateMachine.On("ProcessEpochSetup", mock.Anything).Return(false, nil)
				return epochFallbackStateMachine, nil
			},
		)
		require.NoError(s.T(), err)
		parentState := unittest.ProtocolStateFixture()
		s.stateMachine.On("ParentState").Return(parentState)

		epochSetup := unittest.EpochSetupFixture()
		result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
			result.ServiceEvents = []flow.ServiceEvent{epochSetup.ServiceEvent()}
		})

		block := unittest.BlockHeaderFixture()
		seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(block.ID()))
		s.headersDB.On("ByBlockID", seal.BlockID).Return(block, nil)
		s.resultsDB.On("ByID", seal.ResultID).Return(result, nil)

		s.stateMachine.On("ProcessEpochSetup", epochSetup).Return(false, protocol.NewInvalidServiceEventErrorf("")).Once()

		err = mutator.ApplyServiceEventsFromValidatedSeals([]*flow.Seal{seal})
		require.NoError(s.T(), err)
	})
	s.Run("process-epoch-setup-exception", func() {
		parentState := unittest.ProtocolStateFixture()
		s.stateMachine.On("ParentState").Return(parentState)

		epochSetup := unittest.EpochSetupFixture()
		result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
			result.ServiceEvents = []flow.ServiceEvent{epochSetup.ServiceEvent()}
		})

		block := unittest.BlockHeaderFixture()
		seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(block.ID()))
		s.headersDB.On("ByBlockID", seal.BlockID).Return(block, nil)
		s.resultsDB.On("ByID", seal.ResultID).Return(result, nil)

		exception := errors.New("exception")
		s.stateMachine.On("ProcessEpochSetup", epochSetup).Return(false, exception).Once()

		err := s.mutator.ApplyServiceEventsFromValidatedSeals([]*flow.Seal{seal})
		require.Error(s.T(), err)
		require.False(s.T(), protocol.IsInvalidServiceEventError(err))
	})
}

// TestApplyServiceEvents_InvalidEpochCommit tests that handleServiceEvents rejects invalid epoch commit event and sets
// InvalidEpochTransitionAttempted flag in protocol.ProtocolStateMachine.
func (s *StateMutatorSuite) TestApplyServiceEvents_InvalidEpochCommit() {
	s.Run("invalid-epoch-commit", func() {
		mutator, err := newStateMutator(
			s.headersDB,
			s.resultsDB,
			s.setupsDB,
			s.commitsDB,
			s.globalParams,
			s.candidateView,
			s.parentState,
			func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
				return s.stateMachine, nil
			},
			func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) {
				epochFallbackStateMachine := protocolstatemock.NewProtocolStateMachine(s.T())
				epochFallbackStateMachine.On("ProcessEpochCommit", mock.Anything).Return(false, nil)
				return epochFallbackStateMachine, nil
			},
		)
		require.NoError(s.T(), err)

		parentState := unittest.ProtocolStateFixture()
		s.stateMachine.On("ParentState").Return(parentState)

		epochCommit := unittest.EpochCommitFixture()
		result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
			result.ServiceEvents = []flow.ServiceEvent{epochCommit.ServiceEvent()}
		})

		block := unittest.BlockHeaderFixture()
		seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(block.ID()))
		s.headersDB.On("ByBlockID", seal.BlockID).Return(block, nil)
		s.resultsDB.On("ByID", seal.ResultID).Return(result, nil)

		s.stateMachine.On("ProcessEpochCommit", epochCommit).Return(false, protocol.NewInvalidServiceEventErrorf("")).Once()

		err = mutator.ApplyServiceEventsFromValidatedSeals([]*flow.Seal{seal})
		require.NoError(s.T(), err)
	})
	s.Run("process-epoch-commit-exception", func() {
		parentState := unittest.ProtocolStateFixture()
		s.stateMachine.On("ParentState").Return(parentState)

		epochCommit := unittest.EpochCommitFixture()
		result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
			result.ServiceEvents = []flow.ServiceEvent{epochCommit.ServiceEvent()}
		})

		block := unittest.BlockHeaderFixture()
		seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(block.ID()))
		s.headersDB.On("ByBlockID", seal.BlockID).Return(block, nil)
		s.resultsDB.On("ByID", seal.ResultID).Return(result, nil)

		exception := errors.New("exception")
		s.stateMachine.On("ProcessEpochCommit", epochCommit).Return(false, exception).Once()

		err := s.mutator.ApplyServiceEventsFromValidatedSeals([]*flow.Seal{seal})
		require.Error(s.T(), err)
		require.False(s.T(), protocol.IsInvalidServiceEventError(err))
	})
}

// TestApplyServiceEventsSealsOrdered tests that handleServiceEvents processes seals in order of block height.
func (s *StateMutatorSuite) TestApplyServiceEventsSealsOrdered() {
	parentState := unittest.ProtocolStateFixture()
	s.stateMachine.On("ParentState").Return(parentState)

	blocks := unittest.ChainFixtureFrom(10, unittest.BlockHeaderFixture())
	var seals []*flow.Seal
	resultByHeight := make(map[flow.Identifier]uint64)
	for _, block := range blocks {
		receipt, seal := unittest.ReceiptAndSealForBlock(block)
		resultByHeight[seal.ResultID] = block.Header.Height
		s.headersDB.On("ByBlockID", seal.BlockID).Return(block.Header, nil).Once()
		s.resultsDB.On("ByID", seal.ResultID).Return(&receipt.ExecutionResult, nil).Once()
		seals = append(seals, seal)
	}

	// shuffle seals to make sure they are not ordered in the payload, so `ApplyServiceEventsFromValidatedSeals` needs to explicitly sort them.
	require.NoError(s.T(), rand.Shuffle(uint(len(seals)), func(i, j uint) {
		seals[i], seals[j] = seals[j], seals[i]
	}))

	err := s.mutator.ApplyServiceEventsFromValidatedSeals(seals)
	require.NoError(s.T(), err)

	// assert that results were queried in order of executed block height
	// if seals were properly ordered before processing, then results should be ordered by block height
	lastExecutedBlockHeight := uint64(0)
	for _, call := range s.resultsDB.Calls {
		resultID := call.Arguments.Get(0).(flow.Identifier)
		executedBlockHeight, found := resultByHeight[resultID]
		require.True(s.T(), found)
		require.Less(s.T(), lastExecutedBlockHeight, executedBlockHeight, "seals must be ordered by block height")
	}
}

// TestApplyServiceEventsTransitionToNextEpoch tests that handleServiceEvents transitions to the next epoch
// when epoch has been committed, and we are at the first block of the next epoch.
func (s *StateMutatorSuite) TestApplyServiceEventsTransitionToNextEpoch() {
	parentState := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())
	s.stateMachine.On("ParentState").Return(parentState)
	// we are at the first block of the next epoch
	s.stateMachine.On("View").Return(parentState.CurrentEpochSetup.FinalView + 1)
	s.stateMachine.On("TransitionToNextEpoch").Return(nil).Once()
	err := s.mutator.ApplyServiceEventsFromValidatedSeals([]*flow.Seal{})
	require.NoError(s.T(), err)
}

// TestApplyServiceEventsTransitionToNextEpoch_Error tests that error that has been
// observed in handleServiceEvents when transitioning to the next epoch is propagated to the caller.
func (s *StateMutatorSuite) TestApplyServiceEventsTransitionToNextEpoch_Error() {
	parentState := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())

	s.stateMachine.On("ParentState").Return(parentState)
	// we are at the first block of the next epoch
	s.stateMachine.On("View").Return(parentState.CurrentEpochSetup.FinalView + 1)
	exception := errors.New("exception")
	s.stateMachine.On("TransitionToNextEpoch").Return(exception).Once()
	err := s.mutator.ApplyServiceEventsFromValidatedSeals([]*flow.Seal{})
	require.ErrorIs(s.T(), err, exception)
	require.False(s.T(), protocol.IsInvalidServiceEventError(err))
}
