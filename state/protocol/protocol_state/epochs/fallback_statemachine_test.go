package epochs

import (
	"github.com/onflow/flow-go/model/flow"
	mockstate "github.com/onflow/flow-go/state/protocol/mock"
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
	BaseStateMachineSuite
	params *mockstate.GlobalParams

	stateMachine *FallbackStateMachine
}

func (s *EpochFallbackStateMachineSuite) SetupTest() {
	s.BaseStateMachineSuite.SetupTest()

	s.params = mockstate.NewGlobalParams(s.T())
	s.params.On("EpochCommitSafetyThreshold").Return(uint64(200))
	s.parentProtocolState.InvalidEpochTransitionAttempted = true

	s.stateMachine = NewFallbackStateMachine(s.params, s.candidate.View, s.parentProtocolState.Copy())
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
// It tests scenarios where the EFM is entered in different phases of the epoch.
// It is expected that the state machine will inject a new epoch extension for the current epoch.
func (s *EpochFallbackStateMachineSuite) TestNewEpochFallbackStateMachine() {
	parentProtocolState := s.parentProtocolState.Copy()
	parentProtocolState.InvalidEpochTransitionAttempted = false

	candidateView := parentProtocolState.CurrentEpochSetup.FinalView - s.params.EpochCommitSafetyThreshold() + 1
	s.Run("staking-phase", func() {

		stateMachine := NewFallbackStateMachine(s.params, candidateView, parentProtocolState.Copy())
		require.Equal(s.T(), parentProtocolState.ID(), stateMachine.ParentState().ID())
		require.Equal(s.T(), candidateView, stateMachine.View())

		updatedState, stateID, hasChanges := stateMachine.Build()
		require.True(s.T(), hasChanges, "InvalidEpochTransitionAttempted has to be updated")
		require.True(s.T(), updatedState.InvalidEpochTransitionAttempted, "InvalidEpochTransitionAttempted has to be set")
		require.Equal(s.T(), updatedState.ID(), stateID)
		require.NotEqual(s.T(), parentProtocolState.ID(), stateID)

		require.Len(s.T(), updatedState.CurrentEpoch.EpochExtensions, 1)
		require.Equal(s.T(), flow.EpochExtension{
			FirstView:     parentProtocolState.CurrentEpochFinalView() + 1,
			FinalView:     parentProtocolState.CurrentEpochFinalView() + 1 + DefaultEpochExtensionLength,
			TargetEndTime: 0,
		}, updatedState.CurrentEpoch.EpochExtensions[0])
	})
	s.Run("setup-phase", func() {
		parentProtocolState := parentProtocolState.Copy()
		// setup next epoch but without commit event
		unittest.WithNextEpochProtocolState()(parentProtocolState)
		parentProtocolState.NextEpoch.CommitID = flow.ZeroID
		parentProtocolState.NextEpochCommit = nil

		stateMachine := NewFallbackStateMachine(s.params, candidateView, parentProtocolState)
		require.Equal(s.T(), parentProtocolState.ID(), stateMachine.ParentState().ID())
		require.Equal(s.T(), candidateView, stateMachine.View())

		updatedState, stateID, hasChanges := stateMachine.Build()
		require.True(s.T(), hasChanges, "InvalidEpochTransitionAttempted has to be updated")
		require.True(s.T(), updatedState.InvalidEpochTransitionAttempted, "InvalidEpochTransitionAttempted has to be set")
		require.Equal(s.T(), updatedState.ID(), stateID)
		require.NotEqual(s.T(), parentProtocolState.ID(), stateID)

		require.Len(s.T(), updatedState.CurrentEpoch.EpochExtensions, 1)
		require.Equal(s.T(), flow.EpochExtension{
			FirstView:     parentProtocolState.CurrentEpochFinalView() + 1,
			FinalView:     parentProtocolState.CurrentEpochFinalView() + 1 + DefaultEpochExtensionLength,
			TargetEndTime: 0,
		}, updatedState.CurrentEpoch.EpochExtensions[0])
		require.Nil(s.T(), updatedState.NextEpoch, "Next epoch has to be nil even if it was previously setup")
	})
	s.Run("commit-phase", func() {
		parentProtocolState := parentProtocolState.Copy()
		// setup next committed epoch
		unittest.WithNextEpochProtocolState()(parentProtocolState)

		// if the next epoch has been committed, the extension shouldn't be added to the current epoch
		// instead it will be added to the next epoch when **next** epoch reaches its safety threshold.

		stateMachine := NewFallbackStateMachine(s.params, candidateView, parentProtocolState)
		require.Equal(s.T(), parentProtocolState.ID(), stateMachine.ParentState().ID())
		require.Equal(s.T(), candidateView, stateMachine.View())

		updatedState, stateID, hasChanges := stateMachine.Build()
		require.True(s.T(), hasChanges, "InvalidEpochTransitionAttempted has to be updated")
		require.True(s.T(), updatedState.InvalidEpochTransitionAttempted, "InvalidEpochTransitionAttempted has to be set")
		require.Equal(s.T(), updatedState.ID(), stateID)
		require.NotEqual(s.T(), parentProtocolState.ID(), stateID)

		require.Empty(s.T(), updatedState.CurrentEpoch.EpochExtensions,
			"No extension should be added to the current epoch since next epoch has benn committed")
	})
}
