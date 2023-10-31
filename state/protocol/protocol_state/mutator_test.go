package protocol_state

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/mock"
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
	params          *mock.InstanceParams
	stateMachine    *protocolstatemock.ProtocolStateMachine

	mutator *stateMutator
}

func (s *StateMutatorSuite) SetupTest() {
	s.protocolStateDB = storagemock.NewProtocolState(s.T())
	s.headersDB = storagemock.NewHeaders(s.T())
	s.resultsDB = storagemock.NewExecutionResults(s.T())
	s.setupsDB = storagemock.NewEpochSetups(s.T())
	s.commitsDB = storagemock.NewEpochCommits(s.T())
	s.params = mock.NewInstanceParams(s.T())
	s.stateMachine = protocolstatemock.NewProtocolStateMachine(s.T())
	s.mutator = newStateMutator(s.headersDB, s.resultsDB, s.setupsDB, s.commitsDB, s.stateMachine, s.params)
}

// TestHappyPathNoChanges tests that stateMutator doesn't cache any db updates when there are no changes.
func (s *StateMutatorSuite) TestHappyPathNoChanges() {
	s.params.On("EpochFallbackTriggered").Return(false, nil)
	parentState := unittest.ProtocolStateFixture()
	s.stateMachine.On("ParentState").Return(parentState)
	s.stateMachine.On("Build").Return(parentState.ProtocolStateEntry, parentState.ID(), false)
	err := s.mutator.ApplyServiceEvents([]*flow.Seal{})
	require.NoError(s.T(), err)
	hasChanges, updatedState, updatedStateID, dbUpdates := s.mutator.Build()
	require.False(s.T(), hasChanges)
	require.Equal(s.T(), parentState.ProtocolStateEntry, updatedState)
	require.Equal(s.T(), parentState.ID(), updatedStateID)
	require.Empty(s.T(), dbUpdates)
}

// TestHappyPathHasChanges tests that stateMutator returns cached db updates when building protocol state after applying service events.
func (s *StateMutatorSuite) TestHappyPathHasChanges() {
	s.params.On("EpochFallbackTriggered").Return(false, nil)
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

	s.stateMachine.On("ProcessEpochSetup", epochSetup).Return(nil).Once()
	s.setupsDB.On("StoreTx", epochSetup).Return(func(*transaction.Tx) error { return nil }).Once()

	err := s.mutator.ApplyServiceEvents([]*flow.Seal{seal})
	require.NoError(s.T(), err)

	_, _, _, dbUpdates := s.mutator.Build()
	require.NoError(s.T(), err)
	require.Len(s.T(), dbUpdates, 1)
}

// TestApplyServiceEvents_InvalidEpochSetup tests that handleServiceEvents rejects invalid epoch setup event and sets
// InvalidStateTransitionAttempted flag in protocol.ProtocolStateMachine.
func (s *StateMutatorSuite) TestApplyServiceEvents_InvalidEpochSetup() {
	s.params.On("EpochFallbackTriggered").Return(false, nil)
	s.Run("invalid-epoch-setup", func() {
		parentState := unittest.ProtocolStateFixture()
		rootSetup := parentState.CurrentEpochSetup

		s.stateMachine.On("ParentState").Return(parentState)

		epochSetup := unittest.EpochSetupFixture(
			unittest.WithParticipants(rootSetup.Participants),
			unittest.SetupWithCounter(rootSetup.Counter+1),
			unittest.WithFinalView(rootSetup.FinalView+1000),
			unittest.WithFirstView(rootSetup.FinalView+1),
		)
		result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
			result.ServiceEvents = []flow.ServiceEvent{epochSetup.ServiceEvent()}
		})

		block := unittest.BlockHeaderFixture()
		seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(block.ID()))
		s.headersDB.On("ByBlockID", seal.BlockID).Return(block, nil)
		s.resultsDB.On("ByID", seal.ResultID).Return(result, nil)

		s.stateMachine.On("ProcessEpochSetup", epochSetup).Return(protocol.NewInvalidServiceEventErrorf("")).Once()
		s.stateMachine.On("SetInvalidStateTransitionAttempted").Return().Once()

		err := s.mutator.ApplyServiceEvents([]*flow.Seal{seal})
		require.NoError(s.T(), err)
	})
	s.Run("process-epoch-setup-exception", func() {
		parentState := unittest.ProtocolStateFixture()
		rootSetup := parentState.CurrentEpochSetup

		s.stateMachine.On("ParentState").Return(parentState)

		epochSetup := unittest.EpochSetupFixture(
			unittest.WithParticipants(rootSetup.Participants),
			unittest.SetupWithCounter(rootSetup.Counter+1),
			unittest.WithFinalView(rootSetup.FinalView+1000),
			unittest.WithFirstView(rootSetup.FinalView+1),
		)
		result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
			result.ServiceEvents = []flow.ServiceEvent{epochSetup.ServiceEvent()}
		})

		block := unittest.BlockHeaderFixture()
		seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(block.ID()))
		s.headersDB.On("ByBlockID", seal.BlockID).Return(block, nil)
		s.resultsDB.On("ByID", seal.ResultID).Return(result, nil)

		exception := errors.New("exception")
		s.stateMachine.On("ProcessEpochSetup", epochSetup).Return(exception).Once()

		err := s.mutator.ApplyServiceEvents([]*flow.Seal{seal})
		require.Error(s.T(), err)
		require.False(s.T(), protocol.IsInvalidServiceEventError(err))
	})
}

// TestApplyServiceEvents_InvalidEpochCommit tests that handleServiceEvents rejects invalid epoch commit event and sets
// InvalidStateTransitionAttempted flag in protocol.ProtocolStateMachine.
func (s *StateMutatorSuite) TestApplyServiceEvents_InvalidEpochCommit() {
	s.params.On("EpochFallbackTriggered").Return(false, nil)
	s.Run("invalid-epoch-commit", func() {
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

		s.stateMachine.On("ProcessEpochCommit", epochCommit).Return(protocol.NewInvalidServiceEventErrorf("")).Once()
		s.stateMachine.On("SetInvalidStateTransitionAttempted").Return().Once()

		err := s.mutator.ApplyServiceEvents([]*flow.Seal{seal})
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
		s.stateMachine.On("ProcessEpochCommit", epochCommit).Return(exception).Once()

		err := s.mutator.ApplyServiceEvents([]*flow.Seal{seal})
		require.Error(s.T(), err)
		require.False(s.T(), protocol.IsInvalidServiceEventError(err))
	})
}

// TestApplyServiceEventsSealsOrdered tests that handleServiceEvents processes seals in order of block height.
func (s *StateMutatorSuite) TestApplyServiceEventsSealsOrdered() {
	s.params.On("EpochFallbackTriggered").Return(false, nil)
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

	// shuffle seals to make sure we order them
	require.NoError(s.T(), rand.Shuffle(uint(len(seals)), func(i, j uint) {
		seals[i], seals[j] = seals[j], seals[i]
	}))

	err := s.mutator.ApplyServiceEvents(seals)
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
	s.params.On("EpochFallbackTriggered").Return(false, nil)
	parentState := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())
	s.stateMachine.On("ParentState").Return(parentState)
	// we are at the first block of the next epoch
	s.stateMachine.On("View").Return(parentState.CurrentEpochSetup.FinalView + 1)
	s.stateMachine.On("TransitionToNextEpoch").Return(nil).Once()
	err := s.mutator.ApplyServiceEvents([]*flow.Seal{})
	require.NoError(s.T(), err)
}

// TestApplyServiceEventsTransitionToNextEpoch_Error tests that error that has been
// observed in handleServiceEvents when transitioning to the next epoch is propagated to the caller.
func (s *StateMutatorSuite) TestApplyServiceEventsTransitionToNextEpoch_Error() {
	s.params.On("EpochFallbackTriggered").Return(false, nil)
	parentState := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())

	s.stateMachine.On("ParentState").Return(parentState)
	// we are at the first block of the next epoch
	s.stateMachine.On("View").Return(parentState.CurrentEpochSetup.FinalView + 1)
	exception := errors.New("exception")
	s.stateMachine.On("TransitionToNextEpoch").Return(exception).Once()
	err := s.mutator.ApplyServiceEvents([]*flow.Seal{})
	require.ErrorIs(s.T(), err, exception)
	require.False(s.T(), protocol.IsInvalidServiceEventError(err))
}

// TestApplyServiceEvents_EpochFallbackTriggered tests two scenarios when network is in epoch fallback mode.
// In first case we have observed a global flag that we have finalized fork with invalid event and network is in epoch fallback mode.
// In second case we have observed an InvalidStateTransitionAttempted flag which is part of some fork but has not been finalized yet.
// In both cases we shouldn't process any service events. This is asserted by using mocked state updater without any expected methods.
func (s *StateMutatorSuite) TestApplyServiceEvents_EpochFallbackTriggered() {
	s.Run("epoch-fallback-triggered", func() {
		s.params.On("EpochFallbackTriggered").Return(true, nil)
		parentState := unittest.ProtocolStateFixture()
		s.stateMachine.On("ParentState").Return(parentState)
		seals := unittest.Seal.Fixtures(2)
		err := s.mutator.ApplyServiceEvents(seals)
		require.NoError(s.T(), err)
	})
	s.Run("invalid-service-event-incorporated", func() {
		s.params.On("EpochFallbackTriggered").Return(false, nil)
		parentState := unittest.ProtocolStateFixture()
		parentState.InvalidStateTransitionAttempted = true
		s.stateMachine.On("ParentState").Return(parentState)
		seals := unittest.Seal.Fixtures(2)
		err := s.mutator.ApplyServiceEvents(seals)
		require.NoError(s.T(), err)
	})
}
