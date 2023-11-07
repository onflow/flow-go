package protocol_state

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
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
	stateMachine    *protocolstatemock.ProtocolStateMachine

	mutator *stateMutator
}

func (s *StateMutatorSuite) SetupTest() {
	s.protocolStateDB = storagemock.NewProtocolState(s.T())
	s.headersDB = storagemock.NewHeaders(s.T())
	s.resultsDB = storagemock.NewExecutionResults(s.T())
	s.setupsDB = storagemock.NewEpochSetups(s.T())
	s.commitsDB = storagemock.NewEpochCommits(s.T())
	s.stateMachine = protocolstatemock.NewProtocolStateMachine(s.T())

	s.mutator = newStateMutator(s.headersDB, s.resultsDB, s.setupsDB, s.commitsDB, s.stateMachine, func() ProtocolStateMachine {
		require.Fail(s.T(), "entering epoch fallback is not expected")
		return nil
	})
}

// TestHappyPathNoChanges tests that stateMutator doesn't cache any db updates when there are no changes.
func (s *StateMutatorSuite) TestHappyPathNoChanges() {
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

// TestHappyPathHasChanges tests that stateMutator returns cached db updates when building protocol state after applying service events.
func (s *StateMutatorSuite) TestHappyPathHasChanges() {
	parentState := unittest.ProtocolStateFixture()
	s.stateMachine.On("ParentState").Return(parentState)
	s.stateMachine.On("Build").Return(unittest.ProtocolStateFixture().ProtocolStateEntry,
		unittest.IdentifierFixture(), true)

	epochSetup := unittest.EpochSetupFixture()
	result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
		result.ServiceEvents = []flow.ServiceEvent{epochSetup.ServiceEvent()}
	})

	block := unittest.BlockHeaderFixture()
	seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(block.ID()))
	s.headersDB.On("ByBlockID", seal.BlockID).Return(block, nil)
	s.resultsDB.On("ByID", seal.ResultID).Return(result, nil)

	s.stateMachine.On("ProcessEpochSetup", epochSetup).Return(true, nil).Once()
	s.setupsDB.On("StoreTx", epochSetup).Return(func(*transaction.Tx) error { return nil }).Once()

	err := s.mutator.ApplyServiceEventsFromValidatedSeals([]*flow.Seal{seal})
	require.NoError(s.T(), err)

	_, _, _, dbUpdates := s.mutator.Build()
	require.NoError(s.T(), err)
	require.Len(s.T(), dbUpdates, 1)
}

// TestApplyServiceEvents_InvalidEpochSetup tests that handleServiceEvents rejects invalid epoch setup event and sets
// InvalidStateTransitionAttempted flag in protocol.ProtocolStateMachine.
func (s *StateMutatorSuite) TestApplyServiceEvents_InvalidEpochSetup() {
	s.Run("invalid-epoch-setup", func() {
		mutator := newStateMutator(s.headersDB, s.resultsDB, s.setupsDB, s.commitsDB, s.stateMachine, func() ProtocolStateMachine {
			epochFallbackStateMachine := protocolstatemock.NewProtocolStateMachine(s.T())
			epochFallbackStateMachine.On("ProcessEpochSetup", mock.Anything).Return(false, nil)
			return epochFallbackStateMachine
		})
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

		err := mutator.ApplyServiceEventsFromValidatedSeals([]*flow.Seal{seal})
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
// InvalidStateTransitionAttempted flag in protocol.ProtocolStateMachine.
func (s *StateMutatorSuite) TestApplyServiceEvents_InvalidEpochCommit() {
	s.Run("invalid-epoch-commit", func() {
		mutator := newStateMutator(s.headersDB, s.resultsDB, s.setupsDB, s.commitsDB, s.stateMachine, func() ProtocolStateMachine {
			epochFallbackStateMachine := protocolstatemock.NewProtocolStateMachine(s.T())
			epochFallbackStateMachine.On("ProcessEpochCommit", mock.Anything).Return(false, nil)
			return epochFallbackStateMachine
		})

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

		err := mutator.ApplyServiceEventsFromValidatedSeals([]*flow.Seal{seal})
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
