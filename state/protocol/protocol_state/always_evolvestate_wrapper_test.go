package protocol_state_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	ps_mock "github.com/onflow/flow-go/state/protocol/protocol_state/mock"
	"github.com/onflow/flow-go/storage/badger/transaction"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestView verifies that the AlwaysEvolveStateWrapper correctly report the view from the wrapped state machine
func TestView(t *testing.T) {
	view := uint64(127)
	stateMachine := ps_mock.NewOrthogonalStoreStateMachine[interface{}](t)
	stateMachine.On("View").Return(view).Once()
	wrapper := protocol_state.NewAlwaysEvolveStateWrapper[interface{}](stateMachine)
	require.Equal(t, view, wrapper.View())
}

// TestParentState verifies that the AlwaysEvolveStateWrapper correctly reports the wrapped state machine's ParentState
func TestParentState(t *testing.T) {
	type customType string
	var parentState customType = "some state"
	stateMachine := ps_mock.NewOrthogonalStoreStateMachine[customType](t)
	stateMachine.On("ParentState").Return(parentState).Once()
	wrapper := protocol_state.NewAlwaysEvolveStateWrapper[customType](stateMachine)
	require.Equal(t, parentState, wrapper.ParentState())
}

// TestEvolveState verifies that the `AlwaysEvolveStateWrapper.EvolveState(serviceEvents)` correctly passes the input
// Service Events to the wrapped state machine and passes back its respective outputs.
func TestEvolveState(t *testing.T) {
	serviceEvents := []flow.ServiceEvent{
		unittest.ProtocolStateVersionUpgradeFixture().ServiceEvent(),
		unittest.EpochCommitFixture().ServiceEvent(),
	}

	t.Run("Input passed to wrapped state machine", func(t *testing.T) {
		stateMachine := ps_mock.NewOrthogonalStoreStateMachine[interface{}](t)
		stateMachine.On("EvolveState", serviceEvents).Return(nil).Once()
		wrapper := protocol_state.NewAlwaysEvolveStateWrapper[interface{}](stateMachine)
		require.NoError(t, wrapper.EvolveState(serviceEvents))
	})

	t.Run("wrapped state machine errors", func(t *testing.T) {
		customError := fmt.Errorf("custom error")
		stateMachine := ps_mock.NewOrthogonalStoreStateMachine[interface{}](t)
		stateMachine.On("EvolveState", serviceEvents).Return(customError).Once()
		wrapper := protocol_state.NewAlwaysEvolveStateWrapper[interface{}](stateMachine)
		require.ErrorIs(t, wrapper.EvolveState(serviceEvents), customError, "error should be bubbled up")
	})

}

// TestBuild verifies that the wrapped state machine's `EvolveState(..)` is called by the wrapper
// if and only if it wasn't called before. Furthermore, we test that the wrapper's `Build` method bubbles up
// errors returned by either the state machine's `Build()` or `EvolveState` method.
func TestBuild(t *testing.T) {
	//epochSetup := unittest.EpochSetupFixture().ServiceEvent()
	//epochCommit := unittest.EpochCommitFixture().ServiceEvent()
	//versionBeacon := unittest.VersionBeaconFixture().ServiceEvent()
	//protocolVersionUpgrade := unittest.ProtocolStateVersionUpgradeFixture().ServiceEvent()

	serviceEvents := []flow.ServiceEvent{
		unittest.ProtocolStateVersionUpgradeFixture().ServiceEvent(),
		unittest.EpochCommitFixture().ServiceEvent(),
	}

	someIndexingOperation := func(blockID flow.Identifier, tx *transaction.Tx) error { return nil }
	dbPersist := protocol.NewDeferredBlockPersist().AddIndexingOp(someIndexingOperation)

	t.Run("Happy Path: external logic calls EvolveState before Build", func(t *testing.T) {
		stateMachine := ps_mock.NewOrthogonalStoreStateMachine[interface{}](t)
		wrapper := protocol_state.NewAlwaysEvolveStateWrapper[interface{}](stateMachine)

		// external logic calls `EvolveState` first, which returns with no error
		stateMachine.On("EvolveState", serviceEvents).Return(nil).Once()
		require.NoError(t, wrapper.EvolveState(serviceEvents))

		// external logic subsequently calls `Build`, the wrapper should *not* call EvolveState again:
		stateMachine.On("Build").Return(dbPersist, nil).Once()
		deferredDb, err := wrapper.Build()
		require.NoError(t, err)
		require.Equal(t, dbPersist, deferredDb)
	})

	t.Run("Happy Path: external logic calls only Build", func(t *testing.T) {
		stateMachine := ps_mock.NewOrthogonalStoreStateMachine[interface{}](t)
		wrapper := protocol_state.NewAlwaysEvolveStateWrapper[interface{}](stateMachine)

		// We haven't called `EvolveState` but call `Build` right away. The wrapper should
		//  (1) execute `EvolveState` on the wrapped state machine with an _empty_ list of service events _first_
		//  (2) call `Build` on the wrapped state machine
		evolveStateCalled := atomic.NewBool(false)
		stateMachine.On("EvolveState", mock.Anything).Run(func(args mock.Arguments) {
			require.Empty(t, args.Get(0).([]flow.ServiceEvent), "expecting empty list of service events")
			evolveStateCalled.Store(true)
		}).Return(nil).Once()
		stateMachine.On("Build").Run(func(args mock.Arguments) {
			require.True(t, evolveStateCalled.Load(), "Method EvolveState should have been called first")
		}).Return(dbPersist, nil).Once()

		deferredDb, err := wrapper.Build()
		require.NoError(t, err)
		require.Equal(t, dbPersist, deferredDb)
	})

	t.Run("Unhappy Path: wrapped state machine errors on EvolveState", func(t *testing.T) {
		customError := fmt.Errorf("custom error")
		stateMachine := ps_mock.NewOrthogonalStoreStateMachine[interface{}](t)
		wrapper := protocol_state.NewAlwaysEvolveStateWrapper[interface{}](stateMachine)

		// We haven't called `EvolveState` but call `Build` right away. The wrapper should execute `EvolveState`
		// on the wrapped state machine. If this errors, the wrapper should return the error, _without_ calling `Build`
		stateMachine.On("EvolveState", mock.Anything).Return(customError).Once()
		_, err := wrapper.Build()
		require.ErrorIs(t, err, customError, "error should be bubbled up")
	})

	t.Run("Unhappy Path: wrapped state machine errors on Build", func(t *testing.T) {
		customError := fmt.Errorf("custom error")
		stateMachine := ps_mock.NewOrthogonalStoreStateMachine[interface{}](t)
		wrapper := protocol_state.NewAlwaysEvolveStateWrapper[interface{}](stateMachine)

		// We haven't called `EvolveState` but call `Build` right away. The wrapper should execute `EvolveState` on
		// the wrapped state machine first and only then `Build`. Errors return by the second step sould be bubbled up.
		stateMachine.On("EvolveState", mock.Anything).Return(nil).Once()
		stateMachine.On("Build").Return(dbPersist, customError).Once()

		_, err := wrapper.Build()
		require.ErrorIs(t, err, customError, "error should be bubbled up")
	})
}
