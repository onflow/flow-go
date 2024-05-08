package epochs

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	mockstate "github.com/onflow/flow-go/state/protocol/mock"
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

	var err error
	s.stateMachine, err = NewFallbackStateMachine(s.params, s.candidate.View, s.parentProtocolState.Copy())
	require.NoError(s.T(), err)
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
// It tests scenarios where the EFM is entered in different phases of the epoch,
// and verifies protocol-compliant addition of epoch extensions, depending on the candidate view and epoch phase.
func (s *EpochFallbackStateMachineSuite) TestNewEpochFallbackStateMachine() {
	parentProtocolState := s.parentProtocolState.Copy()
	parentProtocolState.InvalidEpochTransitionAttempted = false

	candidateView := parentProtocolState.CurrentEpochSetup.FinalView - s.params.EpochCommitSafetyThreshold() + 1
	// The view we enter EFM is in the staking phase. The resulting epoch state should be unchanged to the
	// parent state _except_ that `InvalidEpochTransitionAttempted` is set to true.
	s.Run("staking-phase", func() {
		stateMachine, err := NewFallbackStateMachine(s.params, candidateView, parentProtocolState.Copy())
		require.NoError(s.T(), err)
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

	// The view we enter EFM is in the epoch setup phase. This means that a SetupEvent for the next epoch is in the parent block's
	// protocol state. We expect an epoch extension to be added and the outdated information for the next epoch to be removed.
	s.Run("setup-phase", func() {
		parentProtocolState := parentProtocolState.Copy()
		// setup next epoch but without commit event
		unittest.WithNextEpochProtocolState()(parentProtocolState)
		parentProtocolState.NextEpoch.CommitID = flow.ZeroID
		parentProtocolState.NextEpochCommit = nil

		stateMachine, err := NewFallbackStateMachine(s.params, candidateView, parentProtocolState)
		require.NoError(s.T(), err)
		require.Equal(s.T(), parentProtocolState.ID(), stateMachine.ParentState().ID())
		require.Equal(s.T(), candidateView, stateMachine.View())

		updatedState, stateID, hasChanges := stateMachine.Build()
		require.True(s.T(), hasChanges, "InvalidEpochTransitionAttempted has to be updated")
		require.True(s.T(), updatedState.InvalidEpochTransitionAttempted, "InvalidEpochTransitionAttempted has to be set")
		require.Nil(s.T(), updatedState.NextEpoch, "outdated information for the next epoch should have been removed")
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

	// If the next epoch has been committed, the extension shouldn't be added to the current epoch (verified below). Instead, the
	// extension should be added to the next epoch when **next** epoch reaches its safety threshold, which is covered in separate test.
	s.Run("commit-phase", func() {
		parentProtocolState := parentProtocolState.Copy()
		// setup next committed epoch
		unittest.WithNextEpochProtocolState()(parentProtocolState)

		// if the next epoch has been committed, the extension shouldn't be added to the current epoch
		// instead it will be added to the next epoch when **next** epoch reaches its safety threshold.

		stateMachine, err := NewFallbackStateMachine(s.params, candidateView, parentProtocolState)
		require.NoError(s.T(), err)
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

// TestEpochFallbackStateMachineInjectsMultipleExtensions tests that the state machine injects multiple extensions
// as it reaches the safety threshold of the current epoch and the extensions themselves.
// In this test, we are simulating the scenario where the current epoch enters EFM at a view when the next epoch has not been committed yet.
// When the next epoch has been committed the extension should be added to the next epoch, this is covered in separate test.
func (s *EpochFallbackStateMachineSuite) TestEpochFallbackStateMachineInjectsMultipleExtensions() {
	parentProtocolState := s.parentProtocolState.Copy()
	parentProtocolState.InvalidEpochTransitionAttempted = false
	unittest.WithNextEpochProtocolState()(parentProtocolState)
	parentProtocolState.NextEpoch.CommitID = flow.ZeroID
	parentProtocolState.NextEpochCommit = nil

	// finalBlockView is the cumulative number of views that will be produced in the current epoch and its extensions
	finalBlockView := DefaultEpochExtensionLength +
		(parentProtocolState.CurrentEpochSetup.FinalView - parentProtocolState.CurrentEpochSetup.FirstView) + 1
	candidateView := parentProtocolState.CurrentEpochSetup.FirstView + 1
	for i := uint64(0); i < finalBlockView; i++ {
		stateMachine, err := NewFallbackStateMachine(s.params, candidateView, parentProtocolState.Copy())
		require.NoError(s.T(), err)
		updatedState, _, _ := stateMachine.Build()

		parentProtocolState, err = flow.NewRichProtocolStateEntry(updatedState,
			parentProtocolState.PreviousEpochSetup,
			parentProtocolState.PreviousEpochCommit,
			parentProtocolState.CurrentEpochSetup,
			parentProtocolState.CurrentEpochCommit,
			parentProtocolState.NextEpochSetup,
			parentProtocolState.NextEpochCommit)

		require.NoError(s.T(), err)
		candidateView++
	}

	// assert the validity of extensions after producing multiple extensions,
	// we expect 2 extensions to be added to the current epoch
	// 1 after we reach the commit threshold of the epoch and another one after reaching the threshold of the extension themselves
	require.Len(s.T(), parentProtocolState.CurrentEpoch.EpochExtensions, 2)
	require.Equal(s.T(), flow.EpochExtension{
		FirstView:     parentProtocolState.CurrentEpochSetup.FinalView + 1,
		FinalView:     parentProtocolState.CurrentEpochSetup.FinalView + 1 + DefaultEpochExtensionLength,
		TargetEndTime: 0,
	}, parentProtocolState.CurrentEpoch.EpochExtensions[0])

	require.Equal(s.T(), flow.EpochExtension{
		FirstView:     parentProtocolState.CurrentEpoch.EpochExtensions[0].FinalView + 1,
		FinalView:     parentProtocolState.CurrentEpoch.EpochExtensions[0].FinalView + 1 + DefaultEpochExtensionLength,
		TargetEndTime: 0,
	}, parentProtocolState.CurrentEpoch.EpochExtensions[1])

	require.Greater(s.T(), parentProtocolState.CurrentEpochFinalView(), candidateView,
		"final view should be greater than final view of test")
}

// TestEpochFallbackStateMachineInjectsMultipleExtensions_NextEpochCommitted tests that the state machine injects multiple extensions
// as it reaches the safety threshold of the current epoch and the extensions themselves.
// In this test we are simulating the scenario where the current epoch enters fallback mode when the next epoch has been committed.
// It is expected that it will transition into the next epoch (since it was committed)
// then reach the safety threshold and add the extension to the next epoch, which at that point will be considered 'current'.
func (s *EpochFallbackStateMachineSuite) TestEpochFallbackStateMachineInjectsMultipleExtensions_NextEpochCommitted() {
	unittest.SkipUnless(s.T(), unittest.TEST_TODO,
		"This test doesn't work since it misses logic to transition to the next epoch when we are in EFM.")
	parentProtocolState := s.parentProtocolState.Copy()
	parentProtocolState.InvalidEpochTransitionAttempted = false
	unittest.WithNextEpochProtocolState()(parentProtocolState)

	// finalBlockView is the cumulative number of views that will be produced in the current epoch and its extensions
	finalBlockView := DefaultEpochExtensionLength +
		(parentProtocolState.CurrentEpochSetup.FinalView - parentProtocolState.CurrentEpochSetup.FirstView) +
		(parentProtocolState.NextEpochSetup.FinalView - parentProtocolState.NextEpochSetup.FirstView) +
		1
	candidateView := parentProtocolState.CurrentEpochSetup.FirstView + 1
	for i := uint64(0); i < finalBlockView; i++ {
		stateMachine, err := NewFallbackStateMachine(s.params, candidateView, parentProtocolState.Copy())
		require.NoError(s.T(), err)

		if candidateView > parentProtocolState.CurrentEpochFinalView() {
			require.NoError(s.T(), stateMachine.TransitionToNextEpoch())
		}

		updatedState, _, _ := stateMachine.Build()

		parentProtocolState, err = flow.NewRichProtocolStateEntry(updatedState,
			parentProtocolState.PreviousEpochSetup,
			parentProtocolState.PreviousEpochCommit,
			parentProtocolState.CurrentEpochSetup,
			parentProtocolState.CurrentEpochCommit,
			parentProtocolState.NextEpochSetup,
			parentProtocolState.NextEpochCommit)

		require.NoError(s.T(), err)
		candidateView++
	}

	// assert the validity of extensions after producing multiple extensions
	// we expect 3 extensions to be added to the current epoch
	// 1 after we reach the commit threshold of the epoch and two more after reaching the threshold of the extensions themselves
	require.Len(s.T(), parentProtocolState.CurrentEpoch.EpochExtensions, 3)
	require.Equal(s.T(), flow.EpochExtension{
		FirstView:     parentProtocolState.CurrentEpochSetup.FinalView + 1,
		FinalView:     parentProtocolState.CurrentEpochSetup.FinalView + 1 + DefaultEpochExtensionLength,
		TargetEndTime: 0,
	}, parentProtocolState.CurrentEpoch.EpochExtensions[0])

	require.Equal(s.T(), flow.EpochExtension{
		FirstView:     parentProtocolState.CurrentEpoch.EpochExtensions[0].FinalView + 1,
		FinalView:     parentProtocolState.CurrentEpoch.EpochExtensions[0].FinalView + 1 + DefaultEpochExtensionLength,
		TargetEndTime: 0,
	}, parentProtocolState.CurrentEpoch.EpochExtensions[1])

	require.Equal(s.T(), flow.EpochExtension{
		FirstView:     parentProtocolState.CurrentEpoch.EpochExtensions[1].FinalView + 1,
		FinalView:     parentProtocolState.CurrentEpoch.EpochExtensions[1].FinalView + 1 + DefaultEpochExtensionLength,
		TargetEndTime: 0,
	}, parentProtocolState.CurrentEpoch.EpochExtensions[2])

	require.Greater(s.T(), parentProtocolState.CurrentEpochFinalView(), candidateView,
		"final view should be greater than final view of test")
}
