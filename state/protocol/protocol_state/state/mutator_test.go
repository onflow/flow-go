package state

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	protocol_statemock "github.com/onflow/flow-go/state/protocol/protocol_state/mock"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/rand"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProtocolStateMutator(t *testing.T) {
	suite.Run(t, new(StateMutatorSuite))
}

// StateMutatorSuite is a test suite for the stateMutator, it holds the minimum mocked state to set up a stateMutator.
// Tests in this suite are designed to rely on automatic assertions when leaving the scope of the test.
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
		s.candidate.View,
		s.candidate.ParentID,
		s.parentState,
	)
	require.NoError(s.T(), err)
}

// TestBuild_HappyPath tests that `stateMutator` returns all updates from sub-state state machines and prepares updates to the KV store
// when building protocol state.
// In this test, we expect all state machines to return a single deferred db update that will be subsequently returned and executed.
// We also expect that the resulting state will be indexed and stored in the protocol KV store. To assert that, we mock the corresponding
// storage methods and expect them to be called when applying deferred updates in caller code.
func (s *StateMutatorSuite) TestBuild_HappyPath() {
	resultingStateID := unittest.IdentifierFixture()
	factories := make([]protocol_state.KeyValueStoreStateMachineFactory, 2)
	// setup factories, each factory will create a state machine that will return a single deferred db update
	for i := range factories {
		factory := protocol_statemock.NewKeyValueStoreStateMachineFactory(s.T())
		stateMachine := protocol_statemock.NewOrthogonalStoreStateMachine[protocol_state.KVStoreReader](s.T())
		deferredUpdate := storagemock.NewDeferredDBUpdate(s.T())
		deferredUpdate.On("Execute", mock.Anything).Return(nil).Once()
		deferredDBUpdates := protocol.DeferredDBUpdates{}
		deferredDBUpdates.AddBadgerUpdate(deferredUpdate.Execute)
		stateMachine.On("Build").Return(deferredDBUpdates)
		factory.On("Create", s.candidate.View, s.candidate.ParentID, s.parentState, s.replicatedState).Return(stateMachine, nil)
		factories[i] = factory
	}

	var err error
	s.mutator, err = newStateMutator(
		s.headersDB,
		s.resultsDB,
		s.protocolKVStoreDB,
		s.candidate.View,
		s.candidate.ParentID,
		s.parentState,
		factories...,
	)
	require.NoError(s.T(), err)

	// expect actual DB calls that take a badger transaction and apply deferred updates
	indexTxDeferredUpdate := storagemock.NewDeferredDBUpdate(s.T())
	indexTxDeferredUpdate.On("Execute", mock.Anything).Return(nil).Once()
	storeTxDeferredUpdate := storagemock.NewDeferredDBUpdate(s.T())
	storeTxDeferredUpdate.On("Execute", mock.Anything).Return(nil).Once()

	s.replicatedState.On("ID").Return(resultingStateID)
	stateBytes := unittest.RandomBytes(32)
	s.replicatedState.On("VersionedEncode").Return(s.latestProtocolVersion, stateBytes, nil).Once()

	// expect calls to prepare a deferred update for indexing and storing the resulting state
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
	for _, dbUpdate := range dbUpdates.Decorate(s.candidate.ID()) {
		err := dbUpdate(tx)
		require.NoError(s.T(), err)
	}
}

// TestBuild_NoChanges tests that `stateMutator` returns minimal needed updates when building protocol state even if there was no service events applied.
// The minimal needed updates are the index and store of the state that was previously replicated and possibly mutated.
func (s *StateMutatorSuite) TestBuild_NoChanges() {
	resultingStateID := unittest.IdentifierFixture()

	// expect actual DB calls that take a badger transaction and apply deferred updates
	indexTxDeferredUpdate := storagemock.NewDeferredDBUpdate(s.T())
	indexTxDeferredUpdate.On("Execute", mock.Anything).Return(nil).Once()
	storeTxDeferredUpdate := storagemock.NewDeferredDBUpdate(s.T())
	storeTxDeferredUpdate.On("Execute", mock.Anything).Return(nil).Once()

	s.replicatedState.On("ID").Return(resultingStateID)
	stateBytes := unittest.RandomBytes(32)
	s.replicatedState.On("VersionedEncode").Return(s.latestProtocolVersion, stateBytes, nil).Once()
	// expect calls to prepare a deferred update for indexing and storing the resulting state
	s.protocolKVStoreDB.On("IndexTx", s.candidate.ID(), resultingStateID).Return(indexTxDeferredUpdate.Execute).Once()
	s.protocolKVStoreDB.On("StoreTx", resultingStateID, &storage.KeyValueStoreData{
		Version: s.latestProtocolVersion,
		Data:    stateBytes,
	}).Return(storeTxDeferredUpdate.Execute).Once()

	_, dbUpdates, err := s.mutator.Build()
	require.NoError(s.T(), err)

	tx := &transaction.Tx{}
	for _, dbUpdate := range dbUpdates.Decorate(s.candidate.ID()) {
		err := dbUpdate(tx)
		require.NoError(s.T(), err)
	}
}

// TestBuild_EncodeFailed tests that `stateMutator` returns an exception when encoding the resulting state fails.
func (s *StateMutatorSuite) TestBuild_EncodeFailed() {
	resultingStateID := unittest.IdentifierFixture()

	s.replicatedState.On("ID").Return(resultingStateID)
	exception := errors.New("exception")
	s.replicatedState.On("VersionedEncode").Return(uint64(0), []byte{}, exception).Once()

	_, dbUpdates, err := s.mutator.Build()
	require.ErrorIs(s.T(), err, exception)
	require.Empty(s.T(), dbUpdates)
}

// TestStateMutator_Constructor tests the behavior of the stateMutator constructor.
// The constructor must make a series of operations among them:
// - Check if there is a version upgrade available.
// - Replicate the parent state to the actual version.
// - Create a state machine for each sub-state of the Dynamic Protocol State.
func (s *StateMutatorSuite) TestStateMutator_Constructor() {
	s.Run("no-upgrade", func() {
		mutator, err := newStateMutator(
			s.headersDB,
			s.resultsDB,
			s.protocolKVStoreDB,
			s.candidate.View,
			s.candidate.ParentID,
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
			s.candidate.View,
			s.candidate.ParentID,
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
			s.candidate.View,
			s.candidate.ParentID,
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
			s.candidate.View,
			s.candidate.ParentID,
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
			factory.On("Create", s.candidate.View, s.candidate.ParentID, s.parentState, s.replicatedState).Run(func(_ mock.Arguments) {
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
			s.candidate.View,
			s.candidate.ParentID,
			s.parentState,
			factories...,
		)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), mutator)
	})
	s.Run("create-state-machine-exception", func() {
		factory := protocol_statemock.NewKeyValueStoreStateMachineFactory(s.T())
		exception := errors.New("exception")
		factory.On("Create", s.candidate.View, s.candidate.ParentID, s.parentState, s.replicatedState).Return(nil, exception)

		mutator, err := newStateMutator(
			s.headersDB,
			s.resultsDB,
			s.protocolKVStoreDB,
			s.candidate.View,
			s.candidate.ParentID,
			s.parentState,
			factory,
		)
		require.ErrorIs(s.T(), err, exception)
		require.Nil(s.T(), mutator)
	})
}

// TestApplyServiceEventsFromValidatedSeals tests that stateMutator delivers updates to each of the injected state machines.
// In case of an exception from a state machine, the exception is propagated to the caller.
func (s *StateMutatorSuite) TestApplyServiceEventsFromValidatedSeals() {
	s.Run("happy-path", func() {
		factories := make([]protocol_state.KeyValueStoreStateMachineFactory, 2)
		for i := range factories {
			factory := protocol_statemock.NewKeyValueStoreStateMachineFactory(s.T())
			stateMachine := protocol_statemock.NewOrthogonalStoreStateMachine[protocol_state.KVStoreReader](s.T())
			stateMachine.On("ProcessUpdate", mock.Anything).Return(nil).Once()
			factory.On("Create", s.candidate.View, s.candidate.ParentID, s.parentState, s.replicatedState).Return(stateMachine, nil)
			factories[i] = factory
		}

		mutator, err := newStateMutator(
			s.headersDB,
			s.resultsDB,
			s.protocolKVStoreDB,
			s.candidate.View,
			s.candidate.ParentID,
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
		factory.On("Create", s.candidate.View, s.candidate.ParentID, s.parentState, s.replicatedState).Return(stateMachine, nil)

		mutator, err := newStateMutator(
			s.headersDB,
			s.resultsDB,
			s.protocolKVStoreDB,
			s.candidate.View,
			s.candidate.ParentID,
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

// TestApplyServiceEventsSealsOrdered tests that ApplyServiceEventsFromValidatedSeals processes seals in order of block height.
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
