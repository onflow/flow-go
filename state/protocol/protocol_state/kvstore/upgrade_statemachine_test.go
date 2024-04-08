package kvstore_test

import (
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
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
	parentState *mockprotocol.KVStoreReader
	mutator     *mock.KVStoreMutator
	params      *mockprotocol.GlobalParams

	stateMachine *kvstore.PSVersionUpgradeStateMachine
}

func (s *StateMachineSuite) SetupTest() {
	s.parentState = mockprotocol.NewKVStoreReader(s.T())
	s.mutator = mock.NewKVStoreMutator(s.T())
	s.params = mockprotocol.NewGlobalParams(s.T())
	s.view = 1000

	s.params.On("EpochCommitSafetyThreshold").Return(uint64(100)).Maybe()

	s.stateMachine = kvstore.NewPSVersionUpgradeStateMachine(s.view, s.params, s.parentState, s.mutator)
	require.NotNil(s.T(), s.stateMachine)
}

// TestInitialInvariants ensures that initial state machine invariants are met.
// It checks that state machine has correct candidateView and parent state.
func (s *StateMachineSuite) TestInitialInvariants() {
	require.Equal(s.T(), s.view, s.stateMachine.View())
	require.Equal(s.T(), s.parentState, s.stateMachine.ParentState())
}

// TestEvolveState_ProtocolStateVersionUpgrade ensures that state machine can process protocol state version upgrade event.
// It checks several cases including
// * happy path - valid upgrade version and activation view
// * invalid upgrade version - has to return sentinel error since version is invalid
// * invalid activation view - has to return sentinel error since activation view doesn't meet threshold.
func (s *StateMachineSuite) TestEvolveState_ProtocolStateVersionUpgrade() {
	s.Run("happy-path", func() {
		oldVersion := uint64(0)
		s.parentState.On("GetProtocolStateVersion").Return(oldVersion)

		upgrade := unittest.ProtocolStateVersionUpgradeFixture()
		upgrade.ActiveView = s.view + s.params.EpochCommitSafetyThreshold() + 1
		upgrade.NewProtocolStateVersion = oldVersion + 1

		s.mutator.On("GetVersionUpgrade").Return(nil)
		s.mutator.On("SetVersionUpgrade", &protocol.ViewBasedActivator[uint64]{
			Data:           upgrade.NewProtocolStateVersion,
			ActivationView: upgrade.ActiveView,
		}).Return()

		err := s.stateMachine.EvolveState([]flow.ServiceEvent{upgrade.ServiceEvent()})
		require.NoError(s.T(), err)
	})
	s.Run("invalid-protocol-state-version", func() {
		s.mutator = mock.NewKVStoreMutator(s.T())
		oldVersion := uint64(0)
		s.parentState.On("GetProtocolStateVersion").Return(oldVersion)

		upgrade := unittest.ProtocolStateVersionUpgradeFixture()
		upgrade.ActiveView = s.view + s.params.EpochCommitSafetyThreshold() + 1
		upgrade.NewProtocolStateVersion = oldVersion

		_ = s.stateMachine.EvolveState([]flow.ServiceEvent{upgrade.ServiceEvent()})

		// TODO: this needs to be fixed to consume error for consumer, since sentinels are handled internally
		//require.ErrorIs(s.T(), err, ErrInvalidUpgradeVersion, "has to be expected sentinel")
		//require.True(s.T(), protocol.IsInvalidServiceEventError(err), "has to be expected sentinel")
		s.mutator.AssertNumberOfCalls(s.T(), "SetVersionUpgrade", 0)
	})
	s.Run("skipping-protocol-state-version", func() {
		s.mutator = mock.NewKVStoreMutator(s.T())
		oldVersion := uint64(0)
		s.parentState.On("GetProtocolStateVersion").Return(oldVersion)

		upgrade := unittest.ProtocolStateVersionUpgradeFixture()
		upgrade.ActiveView = s.view + s.params.EpochCommitSafetyThreshold() + 1
		upgrade.NewProtocolStateVersion = oldVersion + 2 // has to be exactly +1

		_ = s.stateMachine.EvolveState([]flow.ServiceEvent{upgrade.ServiceEvent()})

		// TODO: this needs to be fixed to consume error for consumer, since sentinels are handled internally
		//require.ErrorIs(s.T(), err, ErrInvalidUpgradeVersion, "has to be expected sentinel")
		//require.True(s.T(), protocol.IsInvalidServiceEventError(err), "has to be expected sentinel")
		s.mutator.AssertNumberOfCalls(s.T(), "SetVersionUpgrade", 0)
	})
	s.Run("invalid-activation-view", func() {
		s.mutator = mock.NewKVStoreMutator(s.T())
		upgrade := unittest.ProtocolStateVersionUpgradeFixture()
		upgrade.ActiveView = s.view + s.params.EpochCommitSafetyThreshold()

		_ = s.stateMachine.EvolveState([]flow.ServiceEvent{upgrade.ServiceEvent()})

		// TODO: this needs to be fixed to consume error for consumer, since sentinels are handled internally
		//require.ErrorIs(s.T(), err, ErrInvalidActivationView, "has to be expected sentinel")
		//require.True(s.T(), protocol.IsInvalidServiceEventError(err), "has to be expected sentinel")
		s.mutator.AssertNumberOfCalls(s.T(), "SetVersionUpgrade", 0)
	})
}

// TestBuild ensures that state machine returns empty list of deferred operations.
func (s *StateMachineSuite) TestBuild() {
	require.Empty(s.T(), s.stateMachine.Build())
}
