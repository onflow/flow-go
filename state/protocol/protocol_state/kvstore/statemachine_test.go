package kvstore

import (
	"github.com/onflow/flow-go/state/protocol/protocol_state/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestStateMachine(t *testing.T) {
	suite.Run(t, new(StateMachineSuite))
}

// BaseProtocolStateMachineSuite is a base test suite that holds common functionality for testing protocol state machines.
// It reflects the portion of data which is present in baseProtocolStateMachine.
type StateMachineSuite struct {
	suite.Suite

	view        uint64
	parentState *mock.Reader
	mutator     *mock.API

	stateMachine *ProcessingStateMachine
}

func (s *StateMachineSuite) SetupTest() {
	s.parentState = mock.NewReader(s.T())
	s.mutator = mock.NewAPI(s.T())
	s.view = 1000

	s.stateMachine = NewProcessingStateMachine(s.view, s.parentState, s.mutator)
	require.NotNil(s.T(), s.stateMachine)
}

func (s *StateMachineSuite) TestInitialInvariants() {
	stateID := unittest.IdentifierFixture()
	s.parentState.On("ID").Return(stateID)
	s.mutator.On("ID").Return(stateID)

	require.Equal(s.T(), s.view, s.stateMachine.View())
	require.Equal(s.T(), s.parentState, s.stateMachine.ParentState())
	_, id, hasChanges := s.stateMachine.Build()
	require.False(s.T(), hasChanges, "initial state should not have changes")
	require.Equal(s.T(), s.parentState.ID(), id, "initial state should not have changes")
}

func (s *StateMachineSuite) TestProcessUpdate() {

}
