package state

import (
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

	headersDB         *storagemock.Headers
	resultsDB         *storagemock.ExecutionResults
	protocolKVStoreDB *storagemock.ProtocolKVStore
	parentState       *protocol_statemock.KVStoreAPI
	candidate         *flow.Header

	mutator *stateMutator
}

func (s *StateMutatorSuite) SetupTest() {
	s.headersDB = storagemock.NewHeaders(s.T())
	s.resultsDB = storagemock.NewExecutionResults(s.T())
	s.parentState = protocol_statemock.NewKVStoreAPI(s.T())
	s.candidate = unittest.BlockHeaderFixture(unittest.HeaderWithView(1000))

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
	stateMachines := make([]*protocol_statemock.KeyValueStoreStateMachine, 2)
	//expectedDbUpdateCalls := make([])
	for i := range stateMachines {
		dbUpdatedExecuted := &mock.Mock{}
		stateMachines[i] = protocol_statemock.NewKeyValueStoreStateMachine(s.T())
		stateMachines[i].On("Build").Return([]transaction.DeferredDBUpdate{
			func(tx *transaction.Tx) error {
				dbUpdatedExecuted.Called()
				return nil
			},
		})
	}

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

	_, dbUpdates, err := s.mutator.Build()
	require.NoError(s.T(), err)
	require.Empty(s.T(), dbUpdates)
}

func (s *StateMutatorSuite) TestBuild_EncodeFailed() {

}

// TestStateMutator_Constructor tests the behaviour of the StateMutator constructor.
// We expect the constructor to select the appropriate state machine constructor, and
// to handle (pass-through) exceptions from the state machine constructor.
func (s *StateMutatorSuite) TestStateMutator_Constructor() {
	s.Run("no-upgrade", func() {

	})
	s.Run("upgrade-available", func() {

	})
	s.Run("outdated-upgrade", func() {

	})
	s.Run("replicate-failure", func() {

	})
	s.Run("multiple-state-machines", func() {

	})
	s.Run("create-state-machine-exception", func() {

	})
}

//// TestApplyServiceEvents_InvalidEpochSetup tests that handleServiceEvents rejects invalid epoch setup event and sets
//// InvalidEpochTransitionAttempted flag in protocol.StateMachine.
//func (s *StateMutatorSuite) TestApplyServiceEvents_InvalidEpochSetup() {
//	s.Run("invalid-epoch-setup", func() {
//		mutator, err := newStateMutator(
//			s.headersDB,
//			s.resultsDB,
//			s.setupsDB,
//			s.commitsDB,
//			s.protocolStateDB,
//			s.protocolKVStoreDB,
//			s.globalParams,
//			s.candidate,
//			s.parentState,
//		)
//		require.NoError(s.T(), err)
//		parentState := unittest.ProtocolStateFixture()
//		s.stateMachine.On("ParentState").Return(parentState)
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
//		s.stateMachine.On("ProcessEpochSetup", epochSetup).Return(false, protocol.NewInvalidServiceEventErrorf("")).Once()
//
//		err = mutator.ApplyServiceEventsFromValidatedSeals([]*flow.Seal{seal})
//		require.NoError(s.T(), err)
//	})
//	s.Run("process-epoch-setup-exception", func() {
//		parentState := unittest.ProtocolStateFixture()
//		s.stateMachine.On("ParentState").Return(parentState)
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
//		exception := errors.New("exception")
//		s.stateMachine.On("ProcessEpochSetup", epochSetup).Return(false, exception).Once()
//
//		err := s.mutator.ApplyServiceEventsFromValidatedSeals([]*flow.Seal{seal})
//		require.Error(s.T(), err)
//		require.False(s.T(), protocol.IsInvalidServiceEventError(err))
//	})
//}

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
