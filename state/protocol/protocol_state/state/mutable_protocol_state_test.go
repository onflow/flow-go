package state

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	psmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	protocol_statemock "github.com/onflow/flow-go/state/protocol/protocol_state/mock"
	"github.com/onflow/flow-go/storage/badger/transaction"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProtocolStateMutator(t *testing.T) {
	suite.Run(t, new(StateMutatorSuite))
}

// StateMutatorSuite is a test suite for the stateMutator, it holds the minimum mocked state to set up a stateMutator.
// Tests in this suite are designed to rely on automatic assertions when leaving the scope of the test.
type StateMutatorSuite struct {
	suite.Suite

	// sub-components injected into MutableProtocolState
	headersDB               storagemock.Headers
	resultsDB               storagemock.ExecutionResults
	protocolKVStoreDB       protocol_statemock.ProtocolKVStore
	epochProtocolStateDB    storagemock.ProtocolState
	globalParams            psmock.GlobalParams
	kvStateMachines         []protocol_statemock.OrthogonalStoreStateMachine[protocol.KVStoreReader]
	kvStateMachineFactories []protocol_state.KeyValueStoreStateMachineFactory

	// basic setup for happy path test
	parentState           protocol_statemock.KVStoreAPI // Protocol state of `candidate`s parent block
	candidate             flow.Header                   // candidate block, potentially still under construction
	evolvingState         protocol_statemock.KVStoreMutator
	latestProtocolVersion uint64

	mutableState *MutableProtocolState
}

func (s *StateMutatorSuite) SetupTest() {
	s.epochProtocolStateDB = *storagemock.NewProtocolState(s.T())
	s.protocolKVStoreDB = *protocol_statemock.NewProtocolKVStore(s.T())
	s.globalParams = *psmock.NewGlobalParams(s.T())
	s.headersDB = *storagemock.NewHeaders(s.T())
	s.resultsDB = *storagemock.NewExecutionResults(s.T())

	// Basic happy-path test scenario:
	// - candidate block:
	s.latestProtocolVersion = 1
	s.candidate = *unittest.BlockHeaderFixture(unittest.HeaderWithView(1000))

	// - protocol state as of `candidate`s parent
	s.parentState = *protocol_statemock.NewKVStoreAPI(s.T())
	s.protocolKVStoreDB.On("ByBlockID", s.candidate.ParentID).Return(&s.parentState, nil)
	s.parentState.On("GetProtocolStateVersion").Return(s.latestProtocolVersion)
	s.parentState.On("GetVersionUpgrade").Return(nil) // no version upgrade by default
	s.parentState.On("ID").Return(unittest.IdentifierFixture(), nil)
	s.parentState.On("Replicate", s.latestProtocolVersion).Return(&s.evolvingState, nil)

	// state replicated from the parent state; by default exactly the same as the parent state
	s.evolvingState = *protocol_statemock.NewKVStoreMutator(s.T())
	s.evolvingState.On("ID").Return(s.parentState.ID(), nil)

	// Factories for the state machines expect `s.parentState` as parent state and `s.replicatedState` as target state.
	// CAUTION: the behaviour of each state machine has to be defined by the tests.
	s.kvStateMachines = make([]protocol_statemock.OrthogonalStoreStateMachine[protocol.KVStoreReader], 2)
	s.kvStateMachineFactories = make([]protocol_state.KeyValueStoreStateMachineFactory, len(s.kvStateMachines))
	for i := range s.kvStateMachines {
		factory := protocol_statemock.NewKeyValueStoreStateMachineFactory(s.T())
		factory.On("Create", s.candidate.View, s.candidate.ParentID, &s.parentState, &s.evolvingState).Return(&s.kvStateMachines[i], nil)
		s.kvStateMachineFactories[i] = factory
	}

	s.mutableState = newMutableProtocolState(
		&s.epochProtocolStateDB,
		&s.protocolKVStoreDB,
		&s.globalParams,
		&s.headersDB,
		&s.resultsDB,
		s.kvStateMachineFactories,
	)
}

// Test_HappyPath_StateInvariant tests that `MutableProtocolState.EvolveState` returns all updates from sub-state state machines and
// prepares updates to the KV store, when building protocol state. Here, we focus on the path, where the *state remains invariant*.
//
// All mocked state machines return a single deferred db update that will be subsequently returned and executed.
// We also expect that the resulting state will be indexed but *not* stored in the protocol KV store (as there are no changes). To
// assert that, we mock the corresponding storage methods and expect them to be called when applying deferred updates in caller code.
func (s *StateMutatorSuite) Test_HappyPath_StateInvariant() {
	parentProtocolStateID := s.parentState.ID()

	// this is the main logic of the test, which essentially tests that `EvolveState`
	//  - indexes the protocol state for the new candidate block, but does not persist it, because it is the same as for the parent
	testEvolveState := func(seals []*flow.Seal) {
		// expect actual DB calls that take a badger transaction and apply deferred updates
		indexTxDeferredUpdate := storagemock.NewDeferredDBUpdate(s.T())
		indexTxDeferredUpdate.On("Execute", mock.Anything).Return(nil).Once()

		// expect calls to prepare a deferred update for indexing the resulting state:
		// as state has not changed, we expect the parent blocks protocol state ID
		s.protocolKVStoreDB.On("IndexTx", s.candidate.ID(), parentProtocolStateID).Return(indexTxDeferredUpdate.Execute).Once()

		resultingStateID, dbUpdates, err := s.mutableState.EvolveState(s.candidate.ParentID, s.candidate.View, seals)
		require.NoError(s.T(), err)
		require.Equal(s.T(), parentProtocolStateID, resultingStateID)

		// Provide the blockID and execute the resulting `DeferredDBUpdate`. Thereby,
		// the expected mock methods should be called, which is asserted by the testify framework
		err = dbUpdates.Pending().WithBlock(s.candidate.ID())(&transaction.Tx{})
		require.NoError(s.T(), err)

		s.protocolKVStoreDB.AssertExpectations(s.T())
		indexTxDeferredUpdate.AssertExpectations(s.T())
	}

	s.Run("no seals, hence no service events", func() {
		for i := range s.kvStateMachines {
			s.kvStateMachines[i] = s.stateMachine().ServiceEventsMatch(emptySlice[flow.ServiceEvent]()).Mock()
		}

		testEvolveState([]*flow.Seal{})
	})

	s.Run("seals without service events", func() {
		sealedBlock := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(s.candidate.View - 10))
		sealedResult := unittest.ExecutionResultFixture()
		seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(sealedBlock.ID()))
		s.headersDB.On("ByBlockID", seal.BlockID).Return(sealedBlock, nil)
		s.resultsDB.On("ByID", seal.ResultID).Return(sealedResult, nil)

		for i := range s.kvStateMachines {
			s.kvStateMachines[i] = s.stateMachine().ServiceEventsMatch(emptySlice[flow.ServiceEvent]()).Mock()
		}

		testEvolveState([]*flow.Seal{seal})
	})

	s.Run("seals with service events", func() {
		sealedBlock := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(s.candidate.View - 10))
		serviceEvents := []flow.ServiceEvent{unittest.EpochSetupFixture().ServiceEvent()}
		sealedResult := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
			result.ServiceEvents = serviceEvents
		})
		seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(sealedBlock.ID()))
		s.headersDB.On("ByBlockID", seal.BlockID).Return(sealedBlock, nil)
		s.resultsDB.On("ByID", seal.ResultID).Return(sealedResult, nil)

		for i := range s.kvStateMachines {
			s.kvStateMachines[i] = s.stateMachine().ExpectedServiceEvents(serviceEvents).Mock()
		}

		testEvolveState([]*flow.Seal{seal})
	})
}

// Test_HappyPath_StateChange tests that `MutableProtocolState.EvolveState` returns all updates from sub-state state machines and
// prepares updates to the KV store, when building protocol state. Here, we focus on the path, where the *state is modified*.
//
// All mocked state machines return a single deferred db update that will be subsequently returned and executed.
// We also expect that the resulting state will be indexed but *not* stored in the protocol KV store (as there are no changes). To
// assert that, we mock the corresponding storage methods and expect them to be called when applying deferred updates in caller code.
func (s *StateMutatorSuite) Test_HappyPath_StateChange() {

	// this is the main logic of the test, which essentially tests that `EvolveState`
	//  - indexes the protocol state for the new candidate block and persists it (as the state changed)
	testEvolveState := func(seals []*flow.Seal, expectedResultingStateID flow.Identifier) {
		// expect actual DB calls that take a badger transaction and apply deferred updates
		indexTxDeferredUpdate := storagemock.NewDeferredDBUpdate(s.T())
		indexTxDeferredUpdate.On("Execute", mock.Anything).Return(nil).Once()
		storeTxDeferredUpdate := storagemock.NewDeferredDBUpdate(s.T())
		storeTxDeferredUpdate.On("Execute", mock.Anything).Return(nil).Once()

		// expect calls to prepare a deferred update for indexing and storing the resulting state:
		// as state has not changed, we expect the parent blocks protocol state ID
		s.protocolKVStoreDB.On("IndexTx", s.candidate.ID(), expectedResultingStateID).Return(indexTxDeferredUpdate.Execute).Once()
		s.protocolKVStoreDB.On("StoreTx", expectedResultingStateID, &s.evolvingState).Return(storeTxDeferredUpdate.Execute).Once()

		resultingStateID, dbUpdates, err := s.mutableState.EvolveState(s.candidate.ParentID, s.candidate.View, seals)
		require.NoError(s.T(), err)
		require.Equal(s.T(), expectedResultingStateID, resultingStateID)

		// Provide the blockID and execute the resulting `DeferredDBUpdate`. Thereby,
		// the expected mock methods should be called, which is asserted by the testify framework
		err = dbUpdates.Pending().WithBlock(s.candidate.ID())(&transaction.Tx{})
		require.NoError(s.T(), err)

		s.protocolKVStoreDB.AssertExpectations(s.T())
		indexTxDeferredUpdate.AssertExpectations(s.T())
		storeTxDeferredUpdate.AssertExpectations(s.T())
	}

	s.Run("no seals, hence no service events", func() {
		s.evolvingState = *protocol_statemock.NewKVStoreMutator(s.T())
		expectedResultingStateID := unittest.IdentifierFixture()
		modifyState := func(_ mock.Arguments) {
			s.evolvingState.On("ID").Return(expectedResultingStateID, nil).Once()
		}
		s.kvStateMachines[0] = s.stateMachine().ServiceEventsMatch(emptySlice[flow.ServiceEvent]()).DuringEvolveState(modifyState).Mock()
		s.kvStateMachines[1] = s.stateMachine().ServiceEventsMatch(emptySlice[flow.ServiceEvent]()).Mock()

		testEvolveState([]*flow.Seal{}, expectedResultingStateID)
	})

	s.Run("seals without service events", func() {
		sealedBlock := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(s.candidate.View - 10))
		sealedResult := unittest.ExecutionResultFixture()
		seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(sealedBlock.ID()))
		s.headersDB.On("ByBlockID", seal.BlockID).Return(sealedBlock, nil)
		s.resultsDB.On("ByID", seal.ResultID).Return(sealedResult, nil)

		expectedResultingStateID := unittest.IdentifierFixture()
		modifyState := func(_ mock.Arguments) {
			s.evolvingState.On("ID").Return(expectedResultingStateID, nil).Once()
		}
		s.kvStateMachines[0] = s.stateMachine().ServiceEventsMatch(emptySlice[flow.ServiceEvent]()).DuringEvolveState(modifyState).Mock()
		s.kvStateMachines[1] = s.stateMachine().ServiceEventsMatch(emptySlice[flow.ServiceEvent]()).Mock()

		testEvolveState([]*flow.Seal{seal}, expectedResultingStateID)
	})

	s.Run("seals with service events", func() {
		sealedBlock := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(s.candidate.View - 10))
		serviceEvents := []flow.ServiceEvent{unittest.EpochSetupFixture().ServiceEvent()}
		sealedResult := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
			result.ServiceEvents = serviceEvents
		})
		seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(sealedBlock.ID()))
		s.headersDB.On("ByBlockID", seal.BlockID).Return(sealedBlock, nil)
		s.resultsDB.On("ByID", seal.ResultID).Return(sealedResult, nil)

		expectedResultingStateID := unittest.IdentifierFixture()
		modifyState := func(_ mock.Arguments) {
			s.evolvingState.On("ID").Return(expectedResultingStateID, nil).Once()
		}
		s.kvStateMachines[0] = s.stateMachine().ExpectedServiceEvents(serviceEvents).DuringEvolveState(modifyState).Mock()
		s.kvStateMachines[1] = s.stateMachine().ExpectedServiceEvents(serviceEvents).Mock()

		testEvolveState([]*flow.Seal{seal}, expectedResultingStateID)
	})
}

// TODO: add tests that service events are ordered (if needed)

//// TestBuild_EncodeFailed tests that `stateMutator` returns an exception when encoding the resulting state fails.
//func (s *StateMutatorSuite) TestBuild_EncodeFailed() {
//	resultingStateID := unittest.IdentifierFixture()
//
//	s.replicatedState.On("ID").Return(resultingStateID)
//	exception := errors.New("exception")
//	s.replicatedState.On("VersionedEncode").Return(uint64(0), []byte{}, exception).Once()
//
//	_, dbUpdates, err := s.mutator.Build()
//	require.ErrorIs(s.T(), err, exception)
//	require.True(s.T(), dbUpdates.IsEmpty())
//}
//
//// TestStateMutator_Constructor tests the behavior of the stateMutator constructor.
//// The constructor must make a series of operations among them:
//// - Check if there is a version upgrade available.
//// - Replicate the parent state to the actual version.
//// - Create a state machine for each sub-state of the Dynamic Protocol State.
//func (s *StateMutatorSuite) TestStateMutator_Constructor() {
//	s.Run("no-upgrade", func() {
//		mutator, err := newStateMutator(
//			s.headersDB,
//			s.resultsDB,
//			s.protocolKVStoreDB,
//			s.candidate.View,
//			s.candidate.ParentID,
//			s.parentState,
//		)
//		require.NoError(s.T(), err)
//		require.NotNil(s.T(), mutator)
//	})
//	s.Run("upgrade-available", func() {
//		newVersion := s.latestProtocolVersion + 1
//		parentState := protocol_statemock.NewKVStoreAPI(s.T())
//		parentState.On("GetProtocolStateVersion").Return(s.latestProtocolVersion).Once()
//		parentState.On("GetVersionUpgrade").Return(&protocol.ViewBasedActivator[uint64]{
//			Data:           newVersion,
//			ActivationView: s.candidate.View,
//		}).Once()
//		replicatedState := protocol_statemock.NewKVStoreMutator(s.T())
//		parentState.On("Replicate", newVersion).Return(replicatedState, nil).Once()
//
//		mutator, err := newStateMutator(
//			s.headersDB,
//			s.resultsDB,
//			s.protocolKVStoreDB,
//			s.candidate.View,
//			s.candidate.ParentID,
//			parentState,
//		)
//		require.NoError(s.T(), err)
//		require.NotNil(s.T(), mutator)
//	})
//	s.Run("outdated-upgrade", func() {
//		parentState := protocol_statemock.NewKVStoreAPI(s.T())
//		parentState.On("GetProtocolStateVersion").Return(s.latestProtocolVersion).Once()
//		parentState.On("GetVersionUpgrade").Return(&protocol.ViewBasedActivator[uint64]{
//			Data:           s.latestProtocolVersion,
//			ActivationView: s.candidate.View - 1,
//		}).Once()
//		replicatedState := protocol_statemock.NewKVStoreMutator(s.T())
//		parentState.On("Replicate", s.latestProtocolVersion).Return(replicatedState, nil).Once()
//
//		mutator, err := newStateMutator(
//			s.headersDB,
//			s.resultsDB,
//			s.protocolKVStoreDB,
//			s.candidate.View,
//			s.candidate.ParentID,
//			parentState,
//		)
//		require.NoError(s.T(), err)
//		require.NotNil(s.T(), mutator)
//	})
//	s.Run("replicate-failure", func() {
//		parentState := protocol_statemock.NewKVStoreAPI(s.T())
//		parentState.On("GetProtocolStateVersion").Return(s.latestProtocolVersion)
//		parentState.On("GetVersionUpgrade").Return(nil).Once()
//		exception := errors.New("exception")
//		parentState.On("Replicate", s.latestProtocolVersion).Return(nil, exception).Once()
//
//		mutator, err := newStateMutator(
//			s.headersDB,
//			s.resultsDB,
//			s.protocolKVStoreDB,
//			s.candidate.View,
//			s.candidate.ParentID,
//			parentState,
//		)
//		require.ErrorIs(s.T(), err, exception)
//		require.Nil(s.T(), mutator)
//	})
//	s.Run("multiple-state-machines", func() {
//		factories := make([]protocol_state.KeyValueStoreStateMachineFactory, 2)
//		lastCalledIdx := -1
//		for i := range factories {
//			factory := protocol_statemock.NewKeyValueStoreStateMachineFactory(s.T())
//			stateMachine := protocol_statemock.NewOrthogonalStoreStateMachine[protocol.KVStoreReader](s.T())
//			calledIndex := i
//			factory.On("Create", s.candidate.View, s.candidate.ParentID, s.parentState, s.replicatedState).Run(func(_ mock.Arguments) {
//				if lastCalledIdx >= calledIndex {
//					require.Failf(s.T(), "state machine factories must be called in order",
//						"expected %d, got %d", lastCalledIdx, calledIndex)
//				}
//				lastCalledIdx = calledIndex
//			}).Return(stateMachine, nil)
//			factories[i] = factory
//		}
//
//		mutator, err := newStateMutator(
//			s.headersDB,
//			s.resultsDB,
//			s.protocolKVStoreDB,
//			s.candidate.View,
//			s.candidate.ParentID,
//			s.parentState,
//			factories...,
//		)
//		require.NoError(s.T(), err)
//		require.NotNil(s.T(), mutator)
//	})
//	s.Run("create-state-machine-exception", func() {
//		factory := protocol_statemock.NewKeyValueStoreStateMachineFactory(s.T())
//		exception := errors.New("exception")
//		factory.On("Create", s.candidate.View, s.candidate.ParentID, s.parentState, s.replicatedState).Return(nil, exception)
//
//		mutator, err := newStateMutator(
//			s.headersDB,
//			s.resultsDB,
//			s.protocolKVStoreDB,
//			s.candidate.View,
//			s.candidate.ParentID,
//			s.parentState,
//			factory,
//		)
//		require.ErrorIs(s.T(), err, exception)
//		require.Nil(s.T(), mutator)
//	})
//}
//
//// TestEvolveState tests that stateMutator delivers updates to each of the injected state machines.
//// In case of an exception from a state machine, the exception is propagated to the caller.
//func (s *StateMutatorSuite) TestEvolveState() {
//	s.Run("happy-path", func() {
//		factories := make([]protocol_state.KeyValueStoreStateMachineFactory, 2)
//		for i := range factories {
//			factory := protocol_statemock.NewKeyValueStoreStateMachineFactory(s.T())
//			stateMachine := protocol_statemock.NewOrthogonalStoreStateMachine[protocol.KVStoreReader](s.T())
//			stateMachine.On("EvolveState", mock.Anything).Return(nil).Once()
//			factory.On("Create", s.candidate.View, s.candidate.ParentID, s.parentState, s.replicatedState).Return(stateMachine, nil)
//			factories[i] = factory
//		}
//
//		mutator, err := newStateMutator(
//			s.headersDB,
//			s.resultsDB,
//			s.protocolKVStoreDB,
//			s.candidate.View,
//			s.candidate.ParentID,
//			s.parentState,
//			factories...,
//		)
//		require.NoError(s.T(), err)
//
//		epochSetup := unittest.EpochSetupFixture()
//		result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
//			result.ServiceEvents = []flow.ServiceEvent{epochSetup.ServiceEvent()}
//		})
//
//		block := unittest.BlockHeaderFixture()
//		seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(block.ID()))
//		s.headersDB.On("ByBlockID", seal.BlockID).Return(block, nil)
//		s.resultsDB.On("ByID", seal.ResultID).Return(result, nil)
//
//		err = mutator.EvolveState([]*flow.Seal{seal})
//		require.NoError(s.T(), err)
//	})
//	s.Run("process-update-exception", func() {
//		factory := protocol_statemock.NewKeyValueStoreStateMachineFactory(s.T())
//		stateMachine := protocol_statemock.NewOrthogonalStoreStateMachine[protocol.KVStoreReader](s.T())
//		exception := errors.New("exception")
//		stateMachine.On("EvolveState", mock.Anything).Return(exception).Once()
//		factory.On("Create", s.candidate.View, s.candidate.ParentID, s.parentState, s.replicatedState).Return(stateMachine, nil)
//
//		mutator, err := newStateMutator(
//			s.headersDB,
//			s.resultsDB,
//			s.protocolKVStoreDB,
//			s.candidate.View,
//			s.candidate.ParentID,
//			s.parentState,
//			factory,
//		)
//		require.NoError(s.T(), err)
//
//		epochSetup := unittest.EpochSetupFixture()
//		result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
//			result.ServiceEvents = []flow.ServiceEvent{epochSetup.ServiceEvent()}
//		})
//
//		block := unittest.BlockHeaderFixture()
//		seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(block.ID()))
//		s.headersDB.On("ByBlockID", seal.BlockID).Return(block, nil)
//		s.resultsDB.On("ByID", seal.ResultID).Return(result, nil)
//
//		err = mutator.EvolveState([]*flow.Seal{seal})
//		require.ErrorIs(s.T(), err, exception)
//		require.False(s.T(), protocol.IsInvalidServiceEventError(err))
//	})
//}
//
//// TestEnforceEvolveState verifies that stateMutator is enforcing `EvolveState` call before `Build`
//// TODO: This is a temporary shortcut until `EvolveState` and `Build` are merged. Then, this test can be removed
//func (s *StateMutatorSuite) TestEnforceEvolveState() {
//	factories := make([]protocol_state.KeyValueStoreStateMachineFactory, 2)
//	for i := range factories {
//		factory := protocol_statemock.NewKeyValueStoreStateMachineFactory(s.T())
//		stateMachine := protocol_statemock.NewOrthogonalStoreStateMachine[protocol.KVStoreReader](s.T())
//		stateMachine.On("EvolveState", mock.Anything).Return(nil).Once()
//		factory.On("Create", s.candidate.View, s.candidate.ParentID, s.parentState, s.replicatedState).Return(stateMachine, nil)
//		factories[i] = factory
//	}
//	mutator, err := newStateMutator(
//		s.headersDB,
//		s.resultsDB,
//		s.protocolKVStoreDB,
//		s.candidate.View,
//		s.candidate.ParentID,
//		s.parentState,
//		factories...,
//	)
//	require.NoError(s.T(), err)
//
//	_, _, err = mutator.Build()
//	require.Error(s.T(), err)
//}
//
//// TestApplyServiceEventsSealsOrdered tests that EvolveState processes seals in order of block height.
//func (s *StateMutatorSuite) TestApplyServiceEventsSealsOrdered() {
//	blocks := unittest.ChainFixtureFrom(10, unittest.BlockHeaderFixture())
//	var seals []*flow.Seal
//	resultByHeight := make(map[flow.Identifier]uint64)
//	for _, block := range blocks {
//		receipt, seal := unittest.ReceiptAndSealForBlock(block)
//		resultByHeight[seal.ResultID] = block.Header.Height
//		s.headersDB.On("ByBlockID", seal.BlockID).Return(block.Header, nil).Once()
//		s.resultsDB.On("ByID", seal.ResultID).Return(&receipt.ExecutionResult, nil).Once()
//		seals = append(seals, seal)
//	}
//
//	// shuffle seals to make sure they are not ordered in the payload, so `EvolveState` needs to explicitly sort them.
//	require.NoError(s.T(), rand.Shuffle(uint(len(seals)), func(i, j uint) {
//		seals[i], seals[j] = seals[j], seals[i]
//	}))
//
//	err := s.mutator.EvolveState(seals)
//	require.NoError(s.T(), err)
//
//	// assert that results were queried in order of executed block height
//	// if seals were properly ordered before processing, then results should be ordered by block height
//	lastExecutedBlockHeight := uint64(0)
//	for _, call := range s.resultsDB.Calls {
//		resultID := call.Arguments.Get(0).(flow.Identifier)
//		executedBlockHeight, found := resultByHeight[resultID]
//		require.True(s.T(), found)
//		require.Less(s.T(), lastExecutedBlockHeight, executedBlockHeight, "seals must be ordered by block height")
//	}
//}

/* *************************************************** utility methods *************************************************** */

func hasID(id flow.Identifier) func(arg interface{}) bool {
	type IDable interface {
		ID() flow.Identifier
	}
	return func(arg interface{}) bool {
		return arg.(IDable).ID() == id
	}
}

func emptySlice[T any]() func(arg interface{}) bool {
	return func(arg interface{}) bool {
		slice := arg.([]T)
		return len(slice) == 0
	}
}

func (s *StateMutatorSuite) stateMachine() *stateMachineMocker {
	return &stateMachineMocker{t: s.T()}
}

type stateMachineMocker struct {
	t                     *testing.T
	expectedServiceEvents interface{}
	runInEvolveState      func(_ mock.Arguments)
}

func (m *stateMachineMocker) ExpectedServiceEvents(es []flow.ServiceEvent) *stateMachineMocker {
	m.expectedServiceEvents = es
	return m
}

func (m *stateMachineMocker) DuringEvolveState(fn func(mock.Arguments)) *stateMachineMocker {
	m.runInEvolveState = fn
	return m
}

func (m *stateMachineMocker) ServiceEventsMatch(fn func(arg interface{}) bool) *stateMachineMocker {
	m.expectedServiceEvents = mock.MatchedBy(fn)
	return m
}

func (m *stateMachineMocker) Mock() protocol_statemock.OrthogonalStoreStateMachine[protocol.KVStoreReader] {
	stateMachine := protocol_statemock.NewOrthogonalStoreStateMachine[protocol.KVStoreReader](m.t)
	if m.runInEvolveState == nil {
		m.runInEvolveState = func(mock.Arguments) {}
	}
	if m.expectedServiceEvents == nil {
		stateMachine.On("EvolveState", mock.Anything).Return(nil).Run(m.runInEvolveState).Once()
	} else {
		stateMachine.On("EvolveState", m.expectedServiceEvents).Return(nil).Run(m.runInEvolveState).Once()
	}
	deferredUpdate := storagemock.NewDeferredDBUpdate(m.t)
	deferredUpdate.On("Execute", mock.Anything).Return(nil).Once()
	deferredDBUpdates := transaction.NewDeferredBlockPersist().AddDbOp(deferredUpdate.Execute)
	stateMachine.On("Build").Return(deferredDBUpdates, nil).Once()
	return *stateMachine
}
