package scripts

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	queryMock "github.com/onflow/flow-go/engine/execution/computation/query/mock"
	stateMock "github.com/onflow/flow-go/engine/execution/state/mock"
	"github.com/onflow/flow-go/model/flow"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type testingContext struct {
	t              *testing.T
	engine         *Engine
	state          *protocol.State
	executionState *stateMock.ExecutionState
	queryExecutor  *queryMock.Executor
	mu             *sync.Mutex
}

func (ctx *testingContext) BlockExist(header *flow.Header) {
	snapshot := new(protocol.Snapshot)
	snapshot.On("Head").Return(header, nil)
	ctx.state.On("AtBlockID", header.ID()).Return(snapshot)
	ctx.executionState.On("NewStorageSnapshot", header.ID(), header.Height).Return(nil)
}

func (ctx *testingContext) BlockExecuted(height uint64, blockID flow.Identifier, executed bool) {
	ctx.executionState.On("IsBlockExecuted", height, blockID).Return(executed, nil)
}

func runWithEngine(t *testing.T, fn func(ctx testingContext)) {
	log := unittest.Logger()

	queryExecutor := new(queryMock.Executor)
	protocolState := new(protocol.State)
	execState := new(stateMock.ExecutionState)

	engine := New(log, protocolState, queryExecutor, execState)
	fn(testingContext{
		t:              t,
		engine:         engine,
		queryExecutor:  queryExecutor,
		executionState: execState,
		state:          protocolState,
	})
}

func TestExecuteScriptAtBlockID(t *testing.T) {
	t.Run("execute script on executed block", func(t *testing.T) {
		runWithEngine(t, func(ctx testingContext) {
			// Meaningless script
			script := []byte{1, 1, 2, 3, 5, 8, 11}
			scriptResult := []byte{1}

			// Ensure block we're about to query against is executable
			blockA := unittest.BlockHeaderFixture()

			// Ensure block exists
			ctx.BlockExist(blockA)

			// Ensure block is executed
			ctx.BlockExecuted(blockA.Height, blockA.ID(), true)

			// Successful call to computation manager
			// TODO(leo): assert query executor received a call to ExecuteScript with
			// the currect block ID, height, and storage snapshot.
			ctx.queryExecutor.
				On("ExecuteScript", mock.Anything, script, [][]byte(nil), blockA, nil).
				Return(scriptResult, nil)

			// Execute our script and expect no error
			res, err := ctx.engine.ExecuteScriptAtBlockID(context.Background(), script, nil, blockA.ID())
			assert.NoError(t, err)
			assert.Equal(t, scriptResult, res)

			// Assert other components were called as expected
			ctx.queryExecutor.AssertExpectations(t)
			ctx.executionState.AssertExpectations(t)
			ctx.state.AssertExpectations(t)
		})
	})

	t.Run("execute script on existing, but un-executed block", func(t *testing.T) {
		runWithEngine(t, func(ctx testingContext) {
			// Meaningless script
			script := []byte{1, 1, 2, 3, 5, 8, 11}

			// Ensure block we're about to query against is executable
			blockA := unittest.BlockHeaderFixture()

			// Ensure block exists
			ctx.BlockExist(blockA)

			// Ensure block we're about to query against is not executed
			ctx.BlockExecuted(blockA.Height, blockA.ID(), false)

			// Execute our script and expect no error
			_, err := ctx.engine.ExecuteScriptAtBlockID(context.Background(), script, nil, blockA.ID())
			assert.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), "state commitment not found"))

			// Assert other components were called as expected
			ctx.executionState.AssertExpectations(t)
			ctx.state.AssertExpectations(t)
		})
	})

	t.Run("execute script on non-existing block", func(t *testing.T) {
		runWithEngine(t, func(ctx testingContext) {
			// Meaningless script
			script := []byte{1, 1, 2, 3, 5, 8, 11}

			// Ensure block we're about to query against is executable
			blockA := unittest.BlockHeaderFixture()

			// Execute our script and expect no error
			_, err := ctx.engine.ExecuteScriptAtBlockID(context.Background(), script, nil, blockA.ID())
			assert.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), "state commitment not found"))

			// Assert other components were called as expected
			ctx.executionState.AssertExpectations(t)
			ctx.state.AssertExpectations(t)
		})
	})

}
