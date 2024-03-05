package kvstore

import (
	"github.com/onflow/flow-go/state/protocol"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
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
	params      *mockprotocol.GlobalParams

	stateMachine *ProcessingStateMachine
}

func (s *StateMachineSuite) SetupTest() {
	s.parentState = mock.NewReader(s.T())
	s.mutator = mock.NewAPI(s.T())
	s.params = mockprotocol.NewGlobalParams(s.T())
	s.view = 1000

	s.mutator.On("Clone").Return(s.mutator).Once()
	s.params.On("EpochCommitSafetyThreshold").Return(uint64(100)).Maybe()

	s.stateMachine = NewProcessingStateMachine(s.view, s.params, s.parentState, s.mutator)
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

func (s *StateMachineSuite) TestProcessUpdate_ProtocolStateVersionUpgrade() {
	s.Run("happy-path", func() {
		oldVersion := uint64(0)
		s.parentState.On("GetProtocolStateVersion").Return(oldVersion)

		upgrade := unittest.ProtocolStateVersionUpgradeFixture()
		upgrade.ActiveView = s.view + s.params.EpochCommitSafetyThreshold() + 1
		upgrade.NewProtocolStateVersion = oldVersion + 1

		s.mutator.On("SetVersionUpgrade", &protocol_state.ViewBasedActivator[uint64]{
			Data:           upgrade.NewProtocolStateVersion,
			ActivationView: upgrade.ActiveView,
		}).Return()

		se := upgrade.ServiceEvent()
		err := s.stateMachine.ProcessUpdate(&se)
		require.NoError(s.T(), err)
	})
	s.Run("invalid-protocol-state-version", func() {
		oldVersion := uint64(0)
		s.parentState.On("GetProtocolStateVersion").Return(oldVersion)

		upgrade := unittest.ProtocolStateVersionUpgradeFixture()
		upgrade.ActiveView = s.view + s.params.EpochCommitSafetyThreshold() + 1
		upgrade.NewProtocolStateVersion = oldVersion

		se := upgrade.ServiceEvent()
		err := s.stateMachine.ProcessUpdate(&se)
		require.ErrorIs(s.T(), err, ErrInvalidUpgradeVersion, "has to be expected sentinel")
		require.True(s.T(), protocol.IsInvalidServiceEventError(err), "has to be expected sentinel")
	})
	s.Run("invalid-activation-view", func() {
		upgrade := unittest.ProtocolStateVersionUpgradeFixture()
		upgrade.ActiveView = s.view + s.params.EpochCommitSafetyThreshold()

		se := upgrade.ServiceEvent()
		err := s.stateMachine.ProcessUpdate(&se)
		require.ErrorIs(s.T(), err, ErrInvalidActivationView, "has to be expected sentinel")
		require.True(s.T(), protocol.IsInvalidServiceEventError(err), "has to be expected sentinel")
	})
}
