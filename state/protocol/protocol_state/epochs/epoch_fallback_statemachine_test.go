package epochs

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestEpochFallbackStateMachine(t *testing.T) {
	suite.Run(t, new(EpochFallbackStateMachineSuite))
}

// ProtocolStateMachineSuite is a dedicated test suite for testing happy path state machine.
type EpochFallbackStateMachineSuite struct {
	BaseProtocolStateMachineSuite
	stateMachine *EpochFallbackStateMachine
}

func (s *EpochFallbackStateMachineSuite) SetupTest() {
	s.BaseProtocolStateMachineSuite.SetupTest()
	s.parentProtocolState.InvalidEpochTransitionAttempted = true
	s.stateMachine = NewEpochFallbackStateMachine(s.candidate.View, s.parentProtocolState.Copy())
}

// ProcessEpochSetupIsNoop ensures that processing epoch setup event is noop.
func (s *EpochFallbackStateMachineSuite) TestProcessEpochSetupIsNoop() {
	setup := unittest.EpochSetupFixture()
	applied, err := s.stateMachine.ProcessEpochSetup(setup)
	require.NoError(s.T(), err)
	require.False(s.T(), applied)
	updatedState, stateID, hasChanges := s.stateMachine.Build()
	require.False(s.T(), hasChanges)
	require.Equal(s.T(), s.parentProtocolState.ID(), updatedState.ID())
	require.Equal(s.T(), updatedState.ID(), stateID)
	require.Equal(s.T(), s.parentProtocolState.ID(), s.stateMachine.ParentState().ID())
}

// ProcessEpochCommitIsNoop ensures that processing epoch commit event is noop.
func (s *EpochFallbackStateMachineSuite) TestProcessEpochCommitIsNoop() {
	commit := unittest.EpochCommitFixture()
	applied, err := s.stateMachine.ProcessEpochCommit(commit)
	require.NoError(s.T(), err)
	require.False(s.T(), applied)
	updatedState, stateID, hasChanges := s.stateMachine.Build()
	require.False(s.T(), hasChanges)
	require.Equal(s.T(), s.parentProtocolState.ID(), updatedState.ID())
	require.Equal(s.T(), updatedState.ID(), stateID)
	require.Equal(s.T(), s.parentProtocolState.ID(), s.stateMachine.ParentState().ID())
}

// TestTransitionToNextEpoch ensures that transition to next epoch is not possible.
func (s *EpochFallbackStateMachineSuite) TestTransitionToNextEpoch() {
	err := s.stateMachine.TransitionToNextEpoch()
	require.NoError(s.T(), err)
	updatedState, updateStateID, hasChanges := s.stateMachine.Build()
	require.False(s.T(), hasChanges)
	require.Equal(s.T(), updatedState.ID(), updateStateID)
	require.Equal(s.T(), s.parentProtocolState.ID(), updateStateID)
}

// TestNewEpochFallbackStateMachine tests that creating epoch fallback state machine sets
// `InvalidEpochTransitionAttempted` to true to record that we have entered epoch fallback mode[EFM].
func (s *EpochFallbackStateMachineSuite) TestNewEpochFallbackStateMachine() {
	s.parentProtocolState.InvalidEpochTransitionAttempted = false
	s.stateMachine = NewEpochFallbackStateMachine(s.candidate.View, s.parentProtocolState.Copy())
	require.Equal(s.T(), s.parentProtocolState.ID(), s.stateMachine.ParentState().ID())
	require.Equal(s.T(), s.candidate.View, s.stateMachine.View())

	updatedState, stateID, hasChanges := s.stateMachine.Build()
	require.True(s.T(), hasChanges, "InvalidEpochTransitionAttempted has to be updated")
	require.True(s.T(), updatedState.InvalidEpochTransitionAttempted, "InvalidEpochTransitionAttempted has to be set")
	require.Equal(s.T(), updatedState.ID(), stateID)
	require.NotEqual(s.T(), s.parentProtocolState.ID(), stateID)
}
