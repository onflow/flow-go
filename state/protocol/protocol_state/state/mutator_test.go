package state

import (
	"errors"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/storage"
	"github.com/stretchr/testify/mock"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	protocol_statemock "github.com/onflow/flow-go/state/protocol/protocol_state/mock"
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

	headersDB             *storagemock.Headers
	resultsDB             *storagemock.ExecutionResults
	protocolKVStoreDB     *storagemock.ProtocolKVStore
	parentState           *protocol_statemock.KVStoreAPI
	replicatedState       *protocol_statemock.KVStoreMutator
	candidate             *flow.Header
	latestProtocolVersion uint64

	mutator *stateMutator
}

func (s *StateMutatorSuite) SetupTest() {
	s.headersDB = storagemock.NewHeaders(s.T())
	s.resultsDB = storagemock.NewExecutionResults(s.T())
	s.protocolKVStoreDB = storagemock.NewProtocolKVStore(s.T())
	s.parentState = protocol_statemock.NewKVStoreAPI(s.T())
	s.replicatedState = protocol_statemock.NewKVStoreMutator(s.T())
	s.candidate = unittest.BlockHeaderFixture(unittest.HeaderWithView(1000))

	s.latestProtocolVersion = 0
	s.parentState.On("GetProtocolStateVersion").Return(s.latestProtocolVersion)
	s.parentState.On("GetVersionUpgrade").Return(nil) // no version upgrade by default
	s.parentState.On("Replicate", s.latestProtocolVersion).Return(s.replicatedState, nil)

	var err error
	s.mutator, err = newStateMutator(
		s.headersDB,
		s.resultsDB,
		s.protocolKVStoreDB,
		s.candidate,
		s.parentState,
	)
	require.NoError(s.T(), err)
}

// TestHappyPathWithDbChanges tests that `stateMutator` returns cached db updates when building protocol state after applying service events.
// Whenever `stateMutator` successfully processes an epoch setup or epoch commit event, it has to create a deferred db update to store the event.
// Deferred db updates are cached in `stateMutator` and returned when building protocol state when calling `Build`.
func (s *StateMutatorSuite) TestBuild_HappyPath() {
	resultingStateID := unittest.IdentifierFixture()
	factories := make([]protocol_state.KeyValueStoreStateMachineFactory, 2)
	for i := range factories {
		factory := protocol_statemock.NewKeyValueStoreStateMachineFactory(s.T())
		stateMachine := protocol_statemock.NewOrthogonalStoreStateMachine[protocol_state.KVStoreReader](s.T())
		deferredUpdate := storagemock.NewDeferredDBUpdate(s.T())
		deferredUpdate.On("Execute", mock.Anything).Return(nil).Once()
		stateMachine.On("Build").Return([]transaction.DeferredDBUpdate{
			deferredUpdate.Execute,
		})
		factory.On("Create", s.candidate, s.parentState, s.replicatedState).Return(stateMachine, nil)
		factories[i] = factory
	}

	var err error
	s.mutator, err = newStateMutator(
		s.headersDB,
		s.resultsDB,
		s.protocolKVStoreDB,
		s.candidate,
		s.parentState,
		factories...,
	)
	require.NoError(s.T(), err)

	indexTxDeferredUpdate := storagemock.NewDeferredDBUpdate(s.T())
	indexTxDeferredUpdate.On("Execute", mock.Anything).Return(nil).Once()
	storeTxDeferredUpdate := storagemock.NewDeferredDBUpdate(s.T())
	storeTxDeferredUpdate.On("Execute", mock.Anything).Return(nil).Once()

	s.replicatedState.On("ID").Return(resultingStateID)
	stateBytes := unittest.RandomBytes(32)
	s.replicatedState.On("VersionedEncode").Return(s.latestProtocolVersion, stateBytes, nil).Once()
	s.protocolKVStoreDB.On("IndexTx", s.candidate.ID(), resultingStateID).Return(indexTxDeferredUpdate.Execute).Once()
	s.protocolKVStoreDB.On("StoreTx", resultingStateID, &storage.KeyValueStoreData{
		Version: s.latestProtocolVersion,
		Data:    stateBytes,
	}).Return(storeTxDeferredUpdate.Execute).Once()

	_, dbUpdates, err := s.mutator.Build()
	require.NoError(s.T(), err)

	// in next loop we assert that we have received expected deferred db updates by executing them
	// and expecting that corresponding mock methods will be called
	tx := &transaction.Tx{}
	for _, dbUpdate := range dbUpdates {
		err := dbUpdate(tx)
		require.NoError(s.T(), err)
	}
}

func (s *StateMutatorSuite) TestBuild_NoChanges() {
	resultingStateID := unittest.IdentifierFixture()

	indexTxDeferredUpdate := storagemock.NewDeferredDBUpdate(s.T())
	indexTxDeferredUpdate.On("Execute", mock.Anything).Return(nil).Once()
	storeTxDeferredUpdate := storagemock.NewDeferredDBUpdate(s.T())
	storeTxDeferredUpdate.On("Execute", mock.Anything).Return(nil).Once()

	s.replicatedState.On("ID").Return(resultingStateID)
	stateBytes := unittest.RandomBytes(32)
	s.replicatedState.On("VersionedEncode").Return(s.latestProtocolVersion, stateBytes, nil).Once()
	s.protocolKVStoreDB.On("IndexTx", s.candidate.ID(), resultingStateID).Return(indexTxDeferredUpdate.Execute).Once()
	s.protocolKVStoreDB.On("StoreTx", resultingStateID, &storage.KeyValueStoreData{
		Version: s.latestProtocolVersion,
		Data:    stateBytes,
	}).Return(storeTxDeferredUpdate.Execute).Once()

	_, dbUpdates, err := s.mutator.Build()
	require.NoError(s.T(), err)

	tx := &transaction.Tx{}
	for _, dbUpdate := range dbUpdates {
		err := dbUpdate(tx)
		require.NoError(s.T(), err)
	}
}

func (s *StateMutatorSuite) TestBuild_EncodeFailed() {
	resultingStateID := unittest.IdentifierFixture()

	s.replicatedState.On("ID").Return(resultingStateID)
	exception := errors.New("exception")
	s.replicatedState.On("VersionedEncode").Return(uint64(0), []byte{}, exception).Once()

	_, dbUpdates, err := s.mutator.Build()
	require.ErrorIs(s.T(), err, exception)
	require.Empty(s.T(), dbUpdates)
}

// TestStateMutator_Constructor tests the behaviour of the StateMutator constructor.
// We expect the constructor to select the appropriate state machine constructor, and
// to handle (pass-through) exceptions from the state machine constructor.
func (s *StateMutatorSuite) TestStateMutator_Constructor() {
	s.Run("no-upgrade", func() {
		mutator, err := newStateMutator(
			s.headersDB,
			s.resultsDB,
			s.protocolKVStoreDB,
			s.candidate,
			s.parentState,
		)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), mutator)
	})
	s.Run("upgrade-available", func() {
		newVersion := s.latestProtocolVersion + 1
		parentState := protocol_statemock.NewKVStoreAPI(s.T())
		parentState.On("GetProtocolStateVersion").Return(s.latestProtocolVersion).Once()
		parentState.On("GetVersionUpgrade").Return(&protocol_state.ViewBasedActivator[uint64]{
			Data:           newVersion,
			ActivationView: s.candidate.View,
		}).Once()
		replicatedState := protocol_statemock.NewKVStoreMutator(s.T())
		parentState.On("Replicate", newVersion).Return(replicatedState, nil).Once()

		mutator, err := newStateMutator(
			s.headersDB,
			s.resultsDB,
			s.protocolKVStoreDB,
			s.candidate,
			parentState,
		)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), mutator)
	})
	s.Run("outdated-upgrade", func() {
		parentState := protocol_statemock.NewKVStoreAPI(s.T())
		parentState.On("GetProtocolStateVersion").Return(s.latestProtocolVersion).Once()
		parentState.On("GetVersionUpgrade").Return(&protocol_state.ViewBasedActivator[uint64]{
			Data:           s.latestProtocolVersion,
			ActivationView: s.candidate.View - 1,
		}).Once()
		replicatedState := protocol_statemock.NewKVStoreMutator(s.T())
		parentState.On("Replicate", s.latestProtocolVersion).Return(replicatedState, nil).Once()

		mutator, err := newStateMutator(
			s.headersDB,
			s.resultsDB,
			s.protocolKVStoreDB,
			s.candidate,
			parentState,
		)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), mutator)
	})
	s.Run("replicate-failure", func() {
		parentState := protocol_statemock.NewKVStoreAPI(s.T())
		parentState.On("GetProtocolStateVersion").Return(s.latestProtocolVersion)
		parentState.On("GetVersionUpgrade").Return(nil).Once()
		exception := errors.New("exception")
		parentState.On("Replicate", s.latestProtocolVersion).Return(nil, exception).Once()

		mutator, err := newStateMutator(
			s.headersDB,
			s.resultsDB,
			s.protocolKVStoreDB,
			s.candidate,
			parentState,
		)
		require.ErrorIs(s.T(), err, exception)
		require.Nil(s.T(), mutator)
	})
	s.Run("multiple-state-machines", func() {
		factories := make([]protocol_state.KeyValueStoreStateMachineFactory, 2)
		lastCalledIdx := -1
		for i := range factories {
			factory := protocol_statemock.NewKeyValueStoreStateMachineFactory(s.T())
			stateMachine := protocol_statemock.NewOrthogonalStoreStateMachine[protocol_state.KVStoreReader](s.T())
			calledIndex := i
			factory.On("Create", s.candidate, s.parentState, s.replicatedState).Run(func(_ mock.Arguments) {
				if lastCalledIdx >= calledIndex {
					require.Failf(s.T(), "state machine factories must be called in order",
						"expected %d, got %d", lastCalledIdx, calledIndex)
				}
				lastCalledIdx = calledIndex
			}).Return(stateMachine, nil)
			factories[i] = factory
		}

		mutator, err := newStateMutator(
			s.headersDB,
			s.resultsDB,
			s.protocolKVStoreDB,
			s.candidate,
			s.parentState,
			factories...,
		)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), mutator)
	})
	s.Run("create-state-machine-exception", func() {
		factory := protocol_statemock.NewKeyValueStoreStateMachineFactory(s.T())
		exception := errors.New("exception")
		factory.On("Create", s.candidate, s.parentState, s.replicatedState).Return(nil, exception)

		mutator, err := newStateMutator(
			s.headersDB,
			s.resultsDB,
			s.protocolKVStoreDB,
			s.candidate,
			s.parentState,
			factory,
		)
		require.ErrorIs(s.T(), err, exception)
		require.Nil(s.T(), mutator)
	})
}

// TestApplyServiceEvents_InvalidEpochSetup tests that handleServiceEvents rejects invalid epoch setup event and sets
// InvalidEpochTransitionAttempted flag in protocol.StateMachine.
func (s *StateMutatorSuite) TestApplyServiceEventsFromValidatedSeals() {
	s.Run("happy-path", func() {
		factories := make([]protocol_state.KeyValueStoreStateMachineFactory, 2)
		for i := range factories {
			factory := protocol_statemock.NewKeyValueStoreStateMachineFactory(s.T())
			stateMachine := protocol_statemock.NewOrthogonalStoreStateMachine[protocol_state.KVStoreReader](s.T())
			stateMachine.On("ProcessUpdate", mock.Anything).Return(nil).Once()
			factory.On("Create", s.candidate, s.parentState, s.replicatedState).Return(stateMachine, nil)
			factories[i] = factory
		}

		mutator, err := newStateMutator(
			s.headersDB,
			s.resultsDB,
			s.protocolKVStoreDB,
			s.candidate,
			s.parentState,
			factories...,
		)
		require.NoError(s.T(), err)

		epochSetup := unittest.EpochSetupFixture()
		result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
			result.ServiceEvents = []flow.ServiceEvent{epochSetup.ServiceEvent()}
		})

		block := unittest.BlockHeaderFixture()
		seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(block.ID()))
		s.headersDB.On("ByBlockID", seal.BlockID).Return(block, nil)
		s.resultsDB.On("ByID", seal.ResultID).Return(result, nil)

		err = mutator.ApplyServiceEventsFromValidatedSeals([]*flow.Seal{seal})
		require.NoError(s.T(), err)
	})
	s.Run("invalid-service-event", func() {
		successStateMachine := protocol_statemock.NewOrthogonalStoreStateMachine[protocol_state.KVStoreReader](s.T())
		successStateMachine.On("ProcessUpdate", mock.Anything).Return(nil)

		invalidEventStateMachine := protocol_statemock.NewOrthogonalStoreStateMachine[protocol_state.KVStoreReader](s.T())
		invalidEventStateMachine.On("ProcessUpdate", mock.Anything).Return(protocol.NewInvalidServiceEventErrorf("invalid event"))

		stateMachines := []*protocol_statemock.OrthogonalStoreStateMachine[protocol_state.KVStoreReader]{
			successStateMachine,
			invalidEventStateMachine,
		}
		factories := make([]protocol_state.KeyValueStoreStateMachineFactory, 0, len(stateMachines))
		for _, stateMachine := range stateMachines {
			factory := protocol_statemock.NewKeyValueStoreStateMachineFactory(s.T())
			factory.On("Create", s.candidate, s.parentState, s.replicatedState).Return(stateMachine, nil)
			factories = append(factories, factory)
		}

		mutator, err := newStateMutator(
			s.headersDB,
			s.resultsDB,
			s.protocolKVStoreDB,
			s.candidate,
			s.parentState,
			factories...,
		)
		require.NoError(s.T(), err)

		epochSetup := unittest.EpochSetupFixture()
		result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
			result.ServiceEvents = []flow.ServiceEvent{epochSetup.ServiceEvent()}
		})

		block := unittest.BlockHeaderFixture()
		seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(block.ID()))
		s.headersDB.On("ByBlockID", seal.BlockID).Return(block, nil)
		s.resultsDB.On("ByID", seal.ResultID).Return(result, nil)

		err = mutator.ApplyServiceEventsFromValidatedSeals([]*flow.Seal{seal})
		require.NoError(s.T(), err)
	})
	s.Run("process-update-exception", func() {
		factory := protocol_statemock.NewKeyValueStoreStateMachineFactory(s.T())
		stateMachine := protocol_statemock.NewOrthogonalStoreStateMachine[protocol_state.KVStoreReader](s.T())
		exception := errors.New("exception")
		stateMachine.On("ProcessUpdate", mock.Anything).Return(exception).Once()
		factory.On("Create", s.candidate, s.parentState, s.replicatedState).Return(stateMachine, nil)

		mutator, err := newStateMutator(
			s.headersDB,
			s.resultsDB,
			s.protocolKVStoreDB,
			s.candidate,
			s.parentState,
			factory,
		)
		require.NoError(s.T(), err)

		epochSetup := unittest.EpochSetupFixture()
		result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
			result.ServiceEvents = []flow.ServiceEvent{epochSetup.ServiceEvent()}
		})

		block := unittest.BlockHeaderFixture()
		seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(block.ID()))
		s.headersDB.On("ByBlockID", seal.BlockID).Return(block, nil)
		s.resultsDB.On("ByID", seal.ResultID).Return(result, nil)

		err = mutator.ApplyServiceEventsFromValidatedSeals([]*flow.Seal{seal})
		require.ErrorIs(s.T(), err, exception)
		require.False(s.T(), protocol.IsInvalidServiceEventError(err))
	})
}

// TestApplyServiceEventsSealsOrdered tests that handleServiceEvents processes seals in order of block height.
func (s *StateMutatorSuite) TestApplyServiceEventsSealsOrdered() {
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
