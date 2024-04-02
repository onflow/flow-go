package epochs_test

import (
	"errors"
	"github.com/onflow/flow-go/state/protocol/protocol_state/epochs"
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

	s.epochStateDB.On("ByBlockID", s.candidate.ParentID).Return(s.parentEpochState, nil)
	s.parentState.On("GetEpochStateID").Return(s.parentEpochState.ID())

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
