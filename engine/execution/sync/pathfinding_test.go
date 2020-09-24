package sync

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	state "github.com/onflow/flow-go/engine/execution/state/mock"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestPathfinding(t *testing.T) {

	/* Input queue:

	0 1 2 3 (height)

	    b-g
	   /
	a-c-d-f
	     \
	      e
	*/

	a := unittest.StateDeltaFixture()
	c := unittest.StateDeltaWithParentFixture(a.Block.Header)
	b := unittest.StateDeltaWithParentFixture(c.Block.Header)
	d := unittest.StateDeltaWithParentFixture(c.Block.Header)
	e := unittest.StateDeltaWithParentFixture(d.Block.Header)
	f := unittest.StateDeltaWithParentFixture(d.Block.Header)
	g := unittest.StateDeltaWithParentFixture(b.Block.Header)

	executionStateMock := new(state.ReadOnlyExecutionState)

	allStateDeltas := []*messages.ExecutionStateDelta{a, b, c, d, e, f, g}

	for _, delta := range allStateDeltas {
		executionStateMock.
			On("RetrieveStateDelta", mock.Anything, delta.Block.ID()).
			Return(delta, nil).Maybe()
	}

	sync := &stateSync{
		execState: executionStateMock,
	}

	testPath := func(start *messages.ExecutionStateDelta, end *messages.ExecutionStateDelta, expectedPath []*messages.ExecutionStateDelta) func(t *testing.T) {
		return func(t *testing.T) {
			deltas, err := sync.findDeltasToSend(context.Background(), start.Block.ID(), end.Block.ID())
			require.NoError(t, err)
			assert.Equal(t, expectedPath, deltas)
		}
	}

	testError := func(start *messages.ExecutionStateDelta, end *messages.ExecutionStateDelta) func(t *testing.T) {
		return func(t *testing.T) {
			_, err := sync.findDeltasToSend(context.Background(), start.Block.ID(), end.Block.ID())
			require.Error(t, err)
		}
	}

	t.Run("simple path", testPath(a, f, []*messages.ExecutionStateDelta{c, d, f}))

	t.Run("simple path reversed", testError(f, a))

	t.Run("empty path", testPath(b, b, nil))

	// path e-d-c is omitted as it must exist on target node for e to be its starting state
	t.Run("leaves", testPath(d, g, []*messages.ExecutionStateDelta{b, g}))
	t.Run("leaves reversed", testPath(g, d, []*messages.ExecutionStateDelta{d}))

	t.Run("one stop", testPath(c, d, []*messages.ExecutionStateDelta{d}))
	t.Run("one stop reverse", testError(d, c))

	t.Run("same height forks", testPath(d, b, []*messages.ExecutionStateDelta{b}))
	t.Run("same height forks reversed ", testPath(b, d, []*messages.ExecutionStateDelta{d}))

	t.Run("same height far forks", testPath(g, e, []*messages.ExecutionStateDelta{d, e}))
	t.Run("same height far forks reversed ", testPath(e, g, []*messages.ExecutionStateDelta{b, g}))
}
