package kvstore_test

import (
	"errors"
	"testing"

	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"
	"github.com/onflow/flow-go/state/protocol/protocol_state/mock"
)

func TestSetKeyValueStoreValueStateMachine(t *testing.T) {
	suite.Run(t, new(SetKeyValueStoreValueStateMachineSuite))
}

// SetKeyValueStoreValueStateMachineSuite is a dedicated test suite for testing KV store state machine.
type SetKeyValueStoreValueStateMachineSuite struct {
	suite.Suite

	view        uint64
	parentState *mockprotocol.KVStoreReader
	mutator     *mock.KVStoreMutator
	telemetry   *mock.StateMachineTelemetryConsumer

	stateMachine *kvstore.SetValueStateMachine
}

func (s *SetKeyValueStoreValueStateMachineSuite) SetupTest() {
	s.telemetry = mock.NewStateMachineTelemetryConsumer(s.T())
	s.parentState = mockprotocol.NewKVStoreReader(s.T())
	s.mutator = mock.NewKVStoreMutator(s.T())
	s.view = 1000

	s.parentState.On("GetEpochCommitSafetyThreshold").Return(uint64(100)).Maybe()

	s.stateMachine = kvstore.NewSetValueStateMachine(s.telemetry, s.view, s.parentState, s.mutator)
	require.NotNil(s.T(), s.stateMachine)
}

// TestInitialInvariants ensures that initial state machine invariants are met.
// It checks that state machine has the correct candidateView and parent state.
func (s *SetKeyValueStoreValueStateMachineSuite) TestInitialInvariants() {
	require.Equal(s.T(), s.view, s.stateMachine.View())
	require.Equal(s.T(), s.parentState, s.stateMachine.ParentState())
}

// TestEvolveState_ SetEpochExtensionViewCount ensures that state machine can process protocol state version upgrade event.
// It checks several cases including
//   - happy path - valid extension length value
//   - invalid extension length value
func (s *SetKeyValueStoreValueStateMachineSuite) TestEvolveState_ SetEpochExtensionViewCount() {
	s.Run("happy-path", func() {
		ev := &flow.SetEpochExtensionViewCount{
			Value: 1000,
		}

		s.telemetry.On("OnServiceEventReceived", ev.ServiceEvent()).Return().Once()
		s.telemetry.On("OnServiceEventProcessed", ev.ServiceEvent()).Return().Once()
		s.mutator.On("SetEpochExtensionViewCount", ev.Value).Return(nil)
		err := s.stateMachine.EvolveState([]flow.ServiceEvent{ev.ServiceEvent()})
		require.NoError(s.T(), err)
	})
	// process two events, one is valid and one is invalid, ensure:
	// 1. valid event is processed
	// 2. invalid event is ignored and reported
	s.Run("invalid-value", func() {
		s.mutator = mock.NewKVStoreMutator(s.T())
		s.stateMachine = kvstore.NewSetValueStateMachine(s.telemetry, s.view, s.parentState, s.mutator)

		valid := &flow.SetEpochExtensionViewCount{
			Value: 1000,
		}
		invalid := &flow.SetEpochExtensionViewCount{
			Value: 50,
		}

		s.mutator.On("SetEpochExtensionViewCount", valid.Value).Return(nil).Once()
		s.mutator.On("SetEpochExtensionViewCount", invalid.Value).Return(kvstore.ErrInvalidValue).Once()
		s.telemetry.On("OnServiceEventReceived", valid.ServiceEvent()).Return().Once()
		s.telemetry.On("OnServiceEventProcessed", valid.ServiceEvent()).Return().Once()
		s.telemetry.On("OnServiceEventReceived", invalid.ServiceEvent()).Return().Once()
		s.telemetry.On("OnInvalidServiceEvent", invalid.ServiceEvent(),
			mocks.MatchedBy(protocol.IsInvalidServiceEventError)).Return().Once()

		err := s.stateMachine.EvolveState([]flow.ServiceEvent{invalid.ServiceEvent(), valid.ServiceEvent()})
		require.NoError(s.T(), err, "sentinel error has to be handled internally")
	})
	s.Run("exception", func() {
		s.mutator = mock.NewKVStoreMutator(s.T())
		s.stateMachine = kvstore.NewSetValueStateMachine(s.telemetry, s.view, s.parentState, s.mutator)
		invalid := &flow.SetEpochExtensionViewCount{
			Value: 50,
		}

		exception := errors.New("kvstore-exception")
		s.mutator.On("SetEpochExtensionViewCount", invalid.Value).Return(exception).Once()
		s.telemetry.On("OnServiceEventReceived", invalid.ServiceEvent()).Return().Once()
		err := s.stateMachine.EvolveState([]flow.ServiceEvent{invalid.ServiceEvent(), invalid.ServiceEvent()})
		require.ErrorIs(s.T(), err, exception, "exception has to be propagated")
	})
}

// TestBuild ensures that state machine returns empty list of deferred operations.
func (s *SetKeyValueStoreValueStateMachineSuite) TestBuild() {
	dbOps, err := s.stateMachine.Build()
	require.NoError(s.T(), err)
	require.True(s.T(), dbOps.IsEmpty())
}
