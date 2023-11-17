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

func (ctx *testingContext) stateCommitmentExist(blockID flow.Identifier, commit flow.StateCommitment) {
	ctx.executionState.On("StateCommitmentByBlockID", mock.Anything, blockID).Return(commit, nil)
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
	t.Run("happy path", func(t *testing.T) {
		runWithEngine(t, func(ctx testingContext) {
			// Meaningless script
			script := []byte{1, 1, 2, 3, 5, 8, 11}
			scriptResult := []byte{1}

			// Ensure block we're about to query against is executable
			blockA := unittest.ExecutableBlockFixture(nil, unittest.StateCommitmentPointerFixture())

			snapshot := new(protocol.Snapshot)
			snapshot.On("Head").Return(blockA.Block.Header, nil)

			commits := make(map[flow.Identifier]flow.StateCommitment)
			commits[blockA.ID()] = *blockA.StartState

			ctx.stateCommitmentExist(blockA.ID(), *blockA.StartState)

			ctx.state.On("AtBlockID", blockA.Block.ID()).Return(snapshot)
			ctx.executionState.On("CreateStorageSnapshot", blockA.Block.ID()).Return(nil, nil, nil)

			ctx.executionState.On("HasState", *blockA.StartState).Return(true)

			// Successful call to computation manager
			ctx.queryExecutor.
				On("ExecuteScript", mock.Anything, script, [][]byte(nil), blockA.Block.Header, nil).
				Return(scriptResult, nil)

			// Execute our script and expect no error
			res, err := ctx.engine.ExecuteScriptAtBlockID(context.Background(), script, nil, blockA.Block.ID())
			assert.NoError(t, err)
			assert.Equal(t, scriptResult, res)

			// Assert other components were called as expected
			ctx.queryExecutor.AssertExpectations(t)
			ctx.executionState.AssertExpectations(t)
			ctx.state.AssertExpectations(t)
		})
	})

	t.Run("return early when state commitment not exist", func(t *testing.T) {
		runWithEngine(t, func(ctx testingContext) {
			// Meaningless script
			script := []byte{1, 1, 2, 3, 5, 8, 11}

			// Ensure block we're about to query against is executable
			blockA := unittest.ExecutableBlockFixture(nil, unittest.StateCommitmentPointerFixture())

			// make sure blockID to state commitment mapping exist
			ctx.executionState.On("StateCommitmentByBlockID", mock.Anything, blockA.ID()).Return(*blockA.StartState, nil)

			// but the state commitment does not exist (e.g. purged)
			ctx.executionState.On("HasState", *blockA.StartState).Return(false)

			// Execute our script and expect no error
			_, err := ctx.engine.ExecuteScriptAtBlockID(context.Background(), script, nil, blockA.Block.ID())
			assert.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), "state commitment not found"))

			// Assert other components were called as expected
			ctx.executionState.AssertExpectations(t)
			ctx.state.AssertExpectations(t)
		})
	})

}
