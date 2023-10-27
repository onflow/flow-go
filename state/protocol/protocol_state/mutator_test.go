package protocol_state

import (
	"errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/state/protocol/mock"
	storerr "github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProtocolStateMutator(t *testing.T) {
	suite.Run(t, new(MutatorSuite))
}

type MutatorSuite struct {
	suite.Suite
	protocolStateDB *storagemock.ProtocolState
	headersDB       *storagemock.Headers
	resultsDB       *storagemock.ExecutionResults
	setupsDB        *storagemock.EpochSetups
	commitsDB       *storagemock.EpochCommits
	params          *mock.InstanceParams

	mutator *Mutator
}

func (s *MutatorSuite) SetupTest() {
	s.protocolStateDB = storagemock.NewProtocolState(s.T())
	s.headersDB = storagemock.NewHeaders(s.T())
	s.resultsDB = storagemock.NewExecutionResults(s.T())
	s.setupsDB = storagemock.NewEpochSetups(s.T())
	s.commitsDB = storagemock.NewEpochCommits(s.T())
	s.params = mock.NewInstanceParams(s.T())

	s.mutator = NewMutator(s.headersDB, s.resultsDB, s.setupsDB, s.commitsDB, s.protocolStateDB, s.params)
}

// TestCreateUpdaterForUnknownBlock tests that CreateUpdater returns an error if the parent protocol state is not found.
func (s *MutatorSuite) TestCreateUpdaterForUnknownBlock() {
	candidate := unittest.BlockHeaderFixture()
	s.protocolStateDB.On("ByBlockID", candidate.ParentID).Return(nil, storerr.ErrNotFound)
	updater, err := s.mutator.CreateUpdater(candidate.View, candidate.ParentID)
	require.ErrorIs(s.T(), err, storerr.ErrNotFound)
	require.Nil(s.T(), updater)
}

// TestMutatorHappyPathNoChanges tests that Mutator correctly indexes the protocol state when there are no changes.
func (s *MutatorSuite) TestMutatorHappyPathNoChanges() {
	parentState := unittest.ProtocolStateFixture()
	candidate := unittest.BlockHeaderFixture(unittest.HeaderWithView(parentState.CurrentEpochSetup.FirstView))
	s.protocolStateDB.On("ByBlockID", candidate.ParentID).Return(parentState, nil)
	updater, err := s.mutator.CreateUpdater(candidate.View, candidate.ParentID)
	require.NoError(s.T(), err)

	s.protocolStateDB.On("Index", candidate.ID(), parentState.ID()).Return(func(tx *transaction.Tx) error { return nil })

	dbOps, _ := s.mutator.CommitProtocolState(candidate.ID(), updater)
	err = dbOps(&transaction.Tx{})
	require.NoError(s.T(), err)
}

// TestMutatorHappyPathHasChanges tests that Mutator correctly persists and indexes the protocol state when there are changes.
func (s *MutatorSuite) TestMutatorHappyPathHasChanges() {
	parentState := unittest.ProtocolStateFixture()
	candidate := unittest.BlockHeaderFixture(unittest.HeaderWithView(parentState.CurrentEpochSetup.FirstView))
	s.protocolStateDB.On("ByBlockID", candidate.ParentID).Return(parentState, nil)
	updater, err := s.mutator.CreateUpdater(candidate.View, candidate.ParentID)
	require.NoError(s.T(), err)

	// update protocol state so it has some changes
	updater.SetInvalidStateTransitionAttempted()
	updatedState, updatedStateID, hasChanges := updater.Build()
	require.True(s.T(), hasChanges)

	s.protocolStateDB.On("StoreTx", updatedStateID, updatedState).Return(func(tx *transaction.Tx) error { return nil })
	s.protocolStateDB.On("Index", candidate.ID(), updatedStateID).Return(func(tx *transaction.Tx) error { return nil })

	dbOps, _ := s.mutator.CommitProtocolState(candidate.ID(), updater)
	err = dbOps(&transaction.Tx{})
	require.NoError(s.T(), err)
}

// TestMutatorApplyServiceEvents_InvalidEpochSetup tests that ApplyServiceEvents rejects invalid epoch setup event and sets
// InvalidStateTransitionAttempted flag in protocol.StateUpdater.
func (s *MutatorSuite) TestMutatorApplyServiceEvents_InvalidEpochSetup() {
	s.params.On("EpochFallbackTriggered").Return(false, nil)
	s.Run("invalid-counter", func() {
		parentState := unittest.ProtocolStateFixture()
		rootSetup := parentState.CurrentEpochSetup
		updater := mock.NewStateUpdater(s.T())
		updater.On("ParentState").Return(parentState)

		epochSetup := unittest.EpochSetupFixture(
			unittest.WithParticipants(rootSetup.Participants),
			unittest.SetupWithCounter(rootSetup.Counter+2), // invalid counter
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

		updater.On("SetInvalidStateTransitionAttempted").Return().Once()

		updates, err := s.mutator.ApplyServiceEvents(updater, []*flow.Seal{seal})
		require.NoError(s.T(), err)
		require.Empty(s.T(), updates)
	})
	s.Run("conflicts-with-protocol-state", func() {
		parentState := unittest.ProtocolStateFixture()
		rootSetup := parentState.CurrentEpochSetup
		updater := mock.NewStateUpdater(s.T())
		updater.On("ParentState").Return(parentState)

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

		updater.On("ProcessEpochSetup", epochSetup).Return(protocol.NewInvalidServiceEventErrorf("")).Once()
		updater.On("SetInvalidStateTransitionAttempted").Return().Once()

		updates, err := s.mutator.ApplyServiceEvents(updater, []*flow.Seal{seal})
		require.NoError(s.T(), err)
		require.Empty(s.T(), updates)
	})
	s.Run("process-epoch-setup-exception", func() {
		parentState := unittest.ProtocolStateFixture()
		rootSetup := parentState.CurrentEpochSetup
		updater := mock.NewStateUpdater(s.T())
		updater.On("ParentState").Return(parentState)

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
		updater.On("ProcessEpochSetup", epochSetup).Return(exception).Once()

		updates, err := s.mutator.ApplyServiceEvents(updater, []*flow.Seal{seal})
		require.Error(s.T(), err)
		require.False(s.T(), protocol.IsInvalidServiceEventError(err))
		require.Empty(s.T(), updates)
	})
}

// TestMutatorApplyServiceEvents_EpochFallbackTriggered tests two scenarios when network is in epoch fallback mode.
// In first case we have observed a global flag that we have finalized fork with invalid event and network is in epoch fallback mode.
// In second case we have observed an InvalidStateTransitionAttempted flag which is part of some fork but has not been finalized yet.
// In both cases we shouldn't process any service events. This is asserted by using mocked state updater without any expected methods.
func (s *MutatorSuite) TestMutatorApplyServiceEvents_EpochFallbackTriggered() {
	s.Run("epoch-fallback-triggered", func() {
		s.params.On("EpochFallbackTriggered").Return(true, nil)
		parentState := unittest.ProtocolStateFixture()
		updater := mock.NewStateUpdater(s.T())
		updater.On("ParentState").Return(parentState)
		seals := unittest.Seal.Fixtures(2)
		updates, err := s.mutator.ApplyServiceEvents(updater, seals)
		require.NoError(s.T(), err)
		require.Empty(s.T(), updates)
	})
	s.Run("invalid-service-event-incorporated", func() {
		s.params.On("EpochFallbackTriggered").Return(false, nil)
		parentState := unittest.ProtocolStateFixture()
		parentState.InvalidStateTransitionAttempted = true
		updater := mock.NewStateUpdater(s.T())
		updater.On("ParentState").Return(parentState)
		seals := unittest.Seal.Fixtures(2)
		updates, err := s.mutator.ApplyServiceEvents(updater, seals)
		require.NoError(s.T(), err)
		require.Empty(s.T(), updates)
	})
}
