package mocks

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution"
	executionUnittest "github.com/onflow/flow-go/engine/execution/state/unittest"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/utils/unittest"
)

type BlockResult struct {
	Block  *entity.ExecutableBlock
	Result *execution.ComputationResult
}

// MockBlockStore contains mocked block computation result
// it ensures that as blocks computation result are created, the block's start state
// is the same as its parent block's end state.
// it also stores which block is executed, so that the mock execution state or computer
// can determine what result to return
type MockBlockStore struct {
	sync.Mutex
	ResultByBlock map[flow.Identifier]*BlockResult
	Executed      map[flow.Identifier]struct{}
	RootBlock     *flow.Header
}

func NewMockBlockStore(t *testing.T) *MockBlockStore {
	rootBlock := unittest.ExecutableBlockFixtureWithParent([][]flow.Identifier{{}},
		unittest.BlockHeaderFixture(), unittest.StateCommitmentPointerFixture())
	rootResult := executionUnittest.ComputationResultForBlockFixture(t,
		unittest.IdentifierFixture(), rootBlock)

	blockResult := &BlockResult{
		Block:  rootBlock,
		Result: rootResult,
	}

	byBlock := make(map[flow.Identifier]*BlockResult)
	byBlock[rootResult.Block.ID()] = blockResult

	executed := make(map[flow.Identifier]struct{})
	executed[rootResult.Block.ID()] = struct{}{}
	return &MockBlockStore{
		ResultByBlock: byBlock,
		Executed:      executed,
		RootBlock:     rootBlock.Block.Header,
	}
}

func (bs *MockBlockStore) MarkExecuted(computationResult *execution.ComputationResult) error {
	bs.Lock()
	defer bs.Unlock()
	blockID := computationResult.ExecutableBlock.Block.Header.ID()
	_, executed := bs.Executed[blockID]
	if executed {
		return fmt.Errorf("block %s already executed", blockID)
	}

	expected, exist := bs.ResultByBlock[blockID]
	if !exist {
		return fmt.Errorf("block %s not found", blockID)
	}

	if expected.Result != computationResult {
		return fmt.Errorf("block %s expected %v, got %v", blockID, expected, computationResult)
	}
	bs.Executed[blockID] = struct{}{}
	return nil
}

func (bs *MockBlockStore) CreateBlockAndMockResult(t *testing.T, block *entity.ExecutableBlock) *execution.ComputationResult {
	bs.Lock()
	defer bs.Unlock()
	blockID := block.ID()
	_, exist := bs.ResultByBlock[blockID]
	require.False(t, exist, "block %s already exists", blockID)

	parent := block.Block.Header.ParentID
	parentResult, ok := bs.ResultByBlock[parent]
	require.True(t, ok, "parent block %s not found", parent)

	previousExecutionResultID := parentResult.Result.ExecutionReceipt.ExecutionResult.ID()

	previousCommit := parentResult.Result.CurrentEndState()

	block.StartState = &previousCommit

	// mock computation result
	cr := executionUnittest.ComputationResultForBlockFixture(t,
		previousExecutionResultID,
		block,
	)
	result := &BlockResult{
		Block:  block,
		Result: cr,
	}
	bs.ResultByBlock[blockID] = result
	return cr
}

func (bs *MockBlockStore) GetExecuted(blockID flow.Identifier) (*BlockResult, error) {
	bs.Lock()
	defer bs.Unlock()
	_, exist := bs.Executed[blockID]
	if !exist {
		return nil, fmt.Errorf("block %s not executed", blockID)
	}

	result, exist := bs.ResultByBlock[blockID]
	if !exist {
		return nil, fmt.Errorf("block %s not found", blockID)
	}
	return result, nil
}

func (bs *MockBlockStore) AssertExecuted(t *testing.T, alias string, block flow.Identifier) {
	bs.Lock()
	defer bs.Unlock()
	_, exist := bs.Executed[block]
	require.True(t, exist, "block %s not executed", alias)
}

func (bs *MockBlockStore) AssertNotExecuted(t *testing.T, alias string, block flow.Identifier) {
	bs.Lock()
	defer bs.Unlock()
	_, exist := bs.Executed[block]
	require.False(t, exist, "block %s executed", alias)
}
