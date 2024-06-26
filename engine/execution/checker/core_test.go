package checker_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/checker"
	stateMock "github.com/onflow/flow-go/engine/execution/state/mock"
	"github.com/onflow/flow-go/model/flow"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func makeCore(t *testing.T) (*checker.Core, *protocol.State, *stateMock.ExecutionState) {
	logger := unittest.Logger()
	state := protocol.NewState(t)
	execState := stateMock.NewExecutionState(t)
	core := checker.NewCore(logger, state, execState)
	return core, state, execState
}

func mockFinalizedBlock(t *testing.T, state *protocol.State, finalized *flow.Header) *protocol.Snapshot {
	finalizedSnapshot := protocol.NewSnapshot(t)
	finalizedSnapshot.On("Head").Return(finalized, nil)
	state.On("Final").Return(finalizedSnapshot)
	return finalizedSnapshot
}

func mockAtBlockID(t *testing.T, state *protocol.State, header *flow.Header) *protocol.Snapshot {
	snapshot := protocol.NewSnapshot(t)
	snapshot.On("Head").Return(header, nil)
	state.On("AtBlockID", header.ID()).Return(snapshot)
	return snapshot
}

func mockSealedBlock(t *testing.T, state *protocol.State, finalized *protocol.Snapshot, sealed *flow.Header) (*flow.ExecutionResult, *flow.Seal) {
	lastSealResult := unittest.ExecutionResultFixture(func(r *flow.ExecutionResult) {
		r.BlockID = sealed.ID()
	})
	lastSeal := unittest.Seal.Fixture(unittest.Seal.WithResult(lastSealResult))
	finalized.On("SealedResult").Return(lastSealResult, lastSeal, nil)
	return lastSealResult, lastSeal
}

func mockFinalizedSealedBlock(t *testing.T, state *protocol.State, finalized *flow.Header, sealed *flow.Header) (*flow.ExecutionResult, *flow.Seal) {
	finalizedSnapshot := mockFinalizedBlock(t, state, finalized)
	return mockSealedBlock(t, state, finalizedSnapshot, sealed)
}

func mockSealedBlockAtHeight(t *testing.T, state *protocol.State, height uint64, lastSealed *flow.Header) (*flow.ExecutionResult, *flow.Seal) {
	snapshotAtHeight := protocol.NewSnapshot(t)
	lastSealedResultAtHeight := unittest.ExecutionResultFixture(func(r *flow.ExecutionResult) {
		r.BlockID = lastSealed.ID()
	})
	lastSealAtHeight := unittest.Seal.Fixture(unittest.Seal.WithResult(lastSealedResultAtHeight))
	snapshotAtHeight.On("SealedResult").Return(lastSealedResultAtHeight, lastSealAtHeight, nil)
	state.On("AtHeight", height).Return(snapshotAtHeight, nil)
	return lastSealedResultAtHeight, lastSealAtHeight
}

func mockExecutedBlock(t *testing.T, es *stateMock.ExecutionState, executed *flow.Header, result *flow.ExecutionResult) {
	commit, err := result.FinalStateCommitment()
	require.NoError(t, err)
	es.On("StateCommitmentByBlockID", executed.ID()).Return(commit, nil)
}

func mockUnexecutedBlock(t *testing.T, es *stateMock.ExecutionState, unexecuted *flow.Header) {
	es.On("StateCommitmentByBlockID", unexecuted.ID()).Return(nil, storage.ErrNotFound)
}

func TestCheckPassIfLastSealedIsExecutedAndMatch(t *testing.T) {
	// ..<- LastSealed(executed) <- .. <- LastFinalized <- .. <- LastExecuted <- ...
	chain, _, _ := unittest.ChainFixture(10)
	lastFinal := chain[7].Header
	lastSealed := chain[5].Header

	core, state, es := makeCore(t)
	lastSealedResult, _ := mockFinalizedSealedBlock(t, state, lastFinal, lastSealed)
	mockAtBlockID(t, state, lastSealed)
	mockExecutedBlock(t, es, lastSealed, lastSealedResult)

	require.NoError(t, core.RunCheck())
}

func TestCheckFailIfLastSealedIsExecutedButMismatch(t *testing.T) {
	// ..<- LastSealed(executed) <- .. <- LastFinalized <- .. <- LastExecuted <- ...
	chain, _, _ := unittest.ChainFixture(10)
	lastFinal := chain[7].Header
	lastSealed := chain[5].Header

	core, state, es := makeCore(t)
	_, _ = mockFinalizedSealedBlock(t, state, lastFinal, lastSealed)
	mockAtBlockID(t, state, lastSealed)

	mismatchingResult := unittest.ExecutionResultFixture()

	mockExecutedBlock(t, es, lastSealed, mismatchingResult)

	require.Error(t, core.RunCheck())
	require.Contains(t, core.RunCheck().Error(), "execution result is different from the sealed result")
}

func TestCheckPassIfLastSealedIsNotExecutedAndLastExecutedMatch(t *testing.T) {
	// LastSealedExecuted (sealed) <..<- LastExecuted(finalized) <..<- LastSealed(not executed) <..<- LastFinalized
	chain, _, _ := unittest.ChainFixture(10)
	lastFinal := chain[7].Header
	lastSealed := chain[5].Header
	lastExecuted := chain[3].Header
	lastSealedExecuted := chain[1].Header

	core, state, es := makeCore(t)
	// mock that last sealed is not executed
	mockFinalizedSealedBlock(t, state, lastFinal, lastSealed)
	mockAtBlockID(t, state, lastSealed)
	mockUnexecutedBlock(t, es, lastSealed)

	// mock the last sealed and is also executed
	es.On("GetHighestExecutedBlockID", mock.Anything).Return(lastExecuted.Height, lastExecuted.ID(), nil)
	lastSealedResultAtExecutedHeight, _ := mockSealedBlockAtHeight(t, state, lastExecuted.Height, lastSealedExecuted)
	mockAtBlockID(t, state, lastSealedExecuted)

	// mock with matching result
	mockExecutedBlock(t, es, lastSealedExecuted, lastSealedResultAtExecutedHeight)

	require.NoError(t, core.RunCheck())
}

func TestCheckFailIfLastSealedIsNotExecutedAndLastExecutedMismatch(t *testing.T) {
	// LastSealedExecuted (sealed) <..<- LastExecuted(finalized) <..<- LastSealed(not executed) <..<- LastFinalized
	chain, _, _ := unittest.ChainFixture(10)
	lastFinal := chain[7].Header
	lastSealed := chain[5].Header
	lastExecuted := chain[3].Header
	lastSealedExecuted := chain[1].Header

	core, state, es := makeCore(t)
	// mock that last sealed is not executed
	mockFinalizedSealedBlock(t, state, lastFinal, lastSealed)
	mockAtBlockID(t, state, lastSealed)
	mockUnexecutedBlock(t, es, lastSealed)

	// mock the last sealed and is also executed
	es.On("GetHighestExecutedBlockID", mock.Anything).Return(lastExecuted.Height, lastExecuted.ID(), nil)
	mockSealedBlockAtHeight(t, state, lastExecuted.Height, lastSealedExecuted)
	mockAtBlockID(t, state, lastSealedExecuted)

	// mock with mismatching result
	mismatchingResult := unittest.ExecutionResultFixture()
	mockExecutedBlock(t, es, lastSealedExecuted, mismatchingResult)

	require.Error(t, core.RunCheck())
	require.Contains(t, core.RunCheck().Error(), "execution result is different from the sealed result")
}
