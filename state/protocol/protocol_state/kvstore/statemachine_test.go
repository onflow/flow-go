package kvstore

import (
	"github.com/onflow/flow-go/model/flow"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/state/protocol"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/state/protocol/protocol_state/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestStateMachine(t *testing.T) {
	suite.Run(t, new(StateMachineSuite))
}

// StateMachineSuite is a dedicated test suite for testing KV store state machine.
type StateMachineSuite struct {
	suite.Suite

	view        uint64
	parentState *mock.KVStoreReader
	mutator     *mock.KVStoreMutator
	params      *mockprotocol.GlobalParams

	stateMachine *PSVersionUpgradeStateMachine
}

func (s *StateMachineSuite) SetupTest() {
	s.parentState = mock.NewKVStoreReader(s.T())
	s.mutator = mock.NewKVStoreMutator(s.T())
	s.params = mockprotocol.NewGlobalParams(s.T())
	s.view = 1000

	s.params.On("EpochCommitSafetyThreshold").Return(uint64(100)).Maybe()

	s.stateMachine = NewPSVersionUpgradeStateMachine(s.view, s.params, s.parentState, s.mutator)
	require.NotNil(s.T(), s.stateMachine)
}

// TestInitialInvariants ensures that initial state machine invariants are met.
// It checks that state machine has correct view and parent state.
func (s *StateMachineSuite) TestInitialInvariants() {
	require.Equal(s.T(), s.view, s.stateMachine.View())
	require.Equal(s.T(), s.parentState, s.stateMachine.ParentState())
}

// TestProcessUpdate_ProtocolStateVersionUpgrade ensures that state machine can process protocol state version upgrade event.
// It checks several cases including
// * happy path - valid upgrade version and activation view
// * invalid upgrade version - has to return sentinel error since version is invalid
// * invalid activation view - has to return sentinel error since activation view doesn't meet threshold.
func (s *StateMachineSuite) TestProcessUpdate_ProtocolStateVersionUpgrade() {
	s.Run("happy-path", func() {
		oldVersion := uint64(0)
		s.parentState.On("GetProtocolStateVersion").Return(oldVersion)

		upgrade := unittest.ProtocolStateVersionUpgradeFixture()
		upgrade.ActiveView = s.view + s.params.EpochCommitSafetyThreshold() + 1
		upgrade.NewProtocolStateVersion = oldVersion + 1

		s.parentState.On("GetVersionUpgrade").Return(nil)
		s.mutator.On("GetVersionUpgrade").Return(nil)
		s.mutator.On("SetVersionUpgrade", &protocol_state.ViewBasedActivator[uint64]{
			Data:           upgrade.NewProtocolStateVersion,
			ActivationView: upgrade.ActiveView,
		}).Return()

		se := upgrade.ServiceEvent()
		err := s.stateMachine.ProcessUpdate([]*flow.ServiceEvent{&se})
		require.NoError(s.T(), err)
	})
	s.Run("invalid-protocol-state-version", func() {
		oldVersion := uint64(0)
		s.parentState.On("GetProtocolStateVersion").Return(oldVersion)

		upgrade := unittest.ProtocolStateVersionUpgradeFixture()
		upgrade.ActiveView = s.view + s.params.EpochCommitSafetyThreshold() + 1
		upgrade.NewProtocolStateVersion = oldVersion

		se := upgrade.ServiceEvent()
		err := s.stateMachine.ProcessUpdate([]*flow.ServiceEvent{&se})
		require.ErrorIs(s.T(), err, ErrInvalidUpgradeVersion, "has to be expected sentinel")
		require.True(s.T(), protocol.IsInvalidServiceEventError(err), "has to be expected sentinel")
	})
	s.Run("invalid-activation-view", func() {
		upgrade := unittest.ProtocolStateVersionUpgradeFixture()
		upgrade.ActiveView = s.view + s.params.EpochCommitSafetyThreshold()

		se := upgrade.ServiceEvent()
		err := s.stateMachine.ProcessUpdate([]*flow.ServiceEvent{&se})
		require.ErrorIs(s.T(), err, ErrInvalidActivationView, "has to be expected sentinel")
		require.True(s.T(), protocol.IsInvalidServiceEventError(err), "has to be expected sentinel")
	})
}

// TestBuild ensures that state machine returns empty list of deferred operations.
func (s *StateMachineSuite) TestBuild() {
	require.Empty(s.T(), s.stateMachine.Build())
}
