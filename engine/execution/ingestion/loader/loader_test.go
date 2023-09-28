package loader_test

import (
	"context"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/ingestion"
	"github.com/onflow/flow-go/engine/execution/ingestion/loader"
	"github.com/onflow/flow-go/engine/execution/state"
	stateMock "github.com/onflow/flow-go/engine/execution/state/mock"
	"github.com/onflow/flow-go/model/flow"
	storageerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mocks"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

var _ ingestion.BlockLoader = (*loader.Loader)(nil)

// ExecutionState is a mocked version of execution state that
// simulates some of its behavior for testing purpose
type mockExecutionState struct {
	sync.Mutex
	stateMock.ExecutionState
	commits map[flow.Identifier]flow.StateCommitment
}

func newMockExecutionState(seal *flow.Seal, genesis *flow.Header) *mockExecutionState {
	commits := make(map[flow.Identifier]flow.StateCommitment)
	commits[seal.BlockID] = seal.FinalState
	es := &mockExecutionState{
		commits: commits,
	}
	es.On("GetHighestExecutedBlockID", mock.Anything).Return(genesis.Height, genesis.ID(), nil)
	return es
}

func (es *mockExecutionState) StateCommitmentByBlockID(
	ctx context.Context,
	blockID flow.Identifier,
) (
	flow.StateCommitment,
	error,
) {
	es.Lock()
	defer es.Unlock()
	commit, ok := es.commits[blockID]
	if !ok {
		return flow.DummyStateCommitment, storageerr.ErrNotFound
	}

	return commit, nil
}

func (es *mockExecutionState) ExecuteBlock(t *testing.T, block *flow.Block) {
	parentExecuted, err := state.IsBlockExecuted(
		context.Background(),
		es,
		block.Header.ParentID)
	require.NoError(t, err)
	require.True(t, parentExecuted, "parent block not executed")

	es.Lock()
	defer es.Unlock()
	es.commits[block.ID()] = unittest.StateCommitmentFixture()
}

func logChain(chain []*flow.Block) {
	log := unittest.Logger()
	for i, block := range chain {
		log.Info().Msgf("block %v, height: %v, ID: %v", i, block.Header.Height, block.ID())
	}
}

func TestLoadingUnexecutedBlocks(t *testing.T) {
	t.Run("only genesis", func(t *testing.T) {
		ps := mocks.NewProtocolState()

		chain, result, seal := unittest.ChainFixture(0)
		genesis := chain[0]

		logChain(chain)

		require.NoError(t, ps.Bootstrap(genesis, result, seal))

		es := newMockExecutionState(seal, genesis.Header)
		ctrl := gomock.NewController(t)
		headers := storage.NewMockHeaders(ctrl)
		headers.EXPECT().ByBlockID(genesis.ID()).Return(genesis.Header, nil)
		log := unittest.Logger()
		loader := loader.NewLoader(log, ps, headers, es)

		unexecuted, err := loader.LoadUnexecuted(context.Background())
		require.NoError(t, err)

		unittest.IDsEqual(t, []flow.Identifier{}, unexecuted)
	})

	t.Run("no finalized, nor pending unexected", func(t *testing.T) {
		ps := mocks.NewProtocolState()

		chain, result, seal := unittest.ChainFixture(4)
		genesis, blockA, blockB, blockC, blockD :=
			chain[0], chain[1], chain[2], chain[3], chain[4]

		logChain(chain)

		require.NoError(t, ps.Bootstrap(genesis, result, seal))
		require.NoError(t, ps.Extend(blockA))
		require.NoError(t, ps.Extend(blockB))
		require.NoError(t, ps.Extend(blockC))
		require.NoError(t, ps.Extend(blockD))

		es := newMockExecutionState(seal, genesis.Header)
		ctrl := gomock.NewController(t)
		headers := storage.NewMockHeaders(ctrl)
		headers.EXPECT().ByBlockID(genesis.ID()).Return(genesis.Header, nil)
		log := unittest.Logger()
		loader := loader.NewLoader(log, ps, headers, es)

		unexecuted, err := loader.LoadUnexecuted(context.Background())
		require.NoError(t, err)

		unittest.IDsEqual(t, []flow.Identifier{blockA.ID(), blockB.ID(), blockC.ID(), blockD.ID()}, unexecuted)
	})

	t.Run("no finalized, some pending executed", func(t *testing.T) {
		ps := mocks.NewProtocolState()

		chain, result, seal := unittest.ChainFixture(4)
		genesis, blockA, blockB, blockC, blockD :=
			chain[0], chain[1], chain[2], chain[3], chain[4]

		logChain(chain)

		require.NoError(t, ps.Bootstrap(genesis, result, seal))
		require.NoError(t, ps.Extend(blockA))
		require.NoError(t, ps.Extend(blockB))
		require.NoError(t, ps.Extend(blockC))
		require.NoError(t, ps.Extend(blockD))

		es := newMockExecutionState(seal, genesis.Header)
		ctrl := gomock.NewController(t)
		headers := storage.NewMockHeaders(ctrl)
		headers.EXPECT().ByBlockID(genesis.ID()).Return(genesis.Header, nil)
		log := unittest.Logger()
		loader := loader.NewLoader(log, ps, headers, es)

		es.ExecuteBlock(t, blockA)
		es.ExecuteBlock(t, blockB)

		unexecuted, err := loader.LoadUnexecuted(context.Background())
		require.NoError(t, err)

		unittest.IDsEqual(t, []flow.Identifier{blockC.ID(), blockD.ID()}, unexecuted)
	})

	t.Run("all finalized have been executed, and no pending executed", func(t *testing.T) {
		ps := mocks.NewProtocolState()

		chain, result, seal := unittest.ChainFixture(4)
		genesis, blockA, blockB, blockC, blockD :=
			chain[0], chain[1], chain[2], chain[3], chain[4]

		logChain(chain)

		require.NoError(t, ps.Bootstrap(genesis, result, seal))
		require.NoError(t, ps.Extend(blockA))
		require.NoError(t, ps.Extend(blockB))
		require.NoError(t, ps.Extend(blockC))
		require.NoError(t, ps.Extend(blockD))

		require.NoError(t, ps.Finalize(blockC.ID()))

		es := newMockExecutionState(seal, genesis.Header)
		ctrl := gomock.NewController(t)
		headers := storage.NewMockHeaders(ctrl)
		headers.EXPECT().ByBlockID(genesis.ID()).Return(genesis.Header, nil)
		log := unittest.Logger()
		loader := loader.NewLoader(log, ps, headers, es)

		// block C is the only finalized block, index its header by its height
		headers.EXPECT().ByHeight(blockC.Header.Height).Return(blockC.Header, nil)

		es.ExecuteBlock(t, blockA)
		es.ExecuteBlock(t, blockB)
		es.ExecuteBlock(t, blockC)

		unexecuted, err := loader.LoadUnexecuted(context.Background())
		require.NoError(t, err)

		unittest.IDsEqual(t, []flow.Identifier{blockD.ID()}, unexecuted)
	})

	t.Run("some finalized are executed and conflicting are executed", func(t *testing.T) {
		ps := mocks.NewProtocolState()

		chain, result, seal := unittest.ChainFixture(4)
		genesis, blockA, blockB, blockC, blockD :=
			chain[0], chain[1], chain[2], chain[3], chain[4]

		logChain(chain)

		require.NoError(t, ps.Bootstrap(genesis, result, seal))
		require.NoError(t, ps.Extend(blockA))
		require.NoError(t, ps.Extend(blockB))
		require.NoError(t, ps.Extend(blockC))
		require.NoError(t, ps.Extend(blockD))

		require.NoError(t, ps.Finalize(blockC.ID()))

		es := newMockExecutionState(seal, genesis.Header)
		ctrl := gomock.NewController(t)
		headers := storage.NewMockHeaders(ctrl)
		headers.EXPECT().ByBlockID(genesis.ID()).Return(genesis.Header, nil)
		log := unittest.Logger()
		loader := loader.NewLoader(log, ps, headers, es)

		// block C is finalized, index its header by its height
		headers.EXPECT().ByHeight(blockC.Header.Height).Return(blockC.Header, nil)

		es.ExecuteBlock(t, blockA)
		es.ExecuteBlock(t, blockB)
		es.ExecuteBlock(t, blockC)

		unexecuted, err := loader.LoadUnexecuted(context.Background())
		require.NoError(t, err)

		unittest.IDsEqual(t, []flow.Identifier{blockD.ID()}, unexecuted)
	})

	t.Run("all pending executed", func(t *testing.T) {
		ps := mocks.NewProtocolState()

		chain, result, seal := unittest.ChainFixture(4)
		genesis, blockA, blockB, blockC, blockD :=
			chain[0], chain[1], chain[2], chain[3], chain[4]

		logChain(chain)

		require.NoError(t, ps.Bootstrap(genesis, result, seal))
		require.NoError(t, ps.Extend(blockA))
		require.NoError(t, ps.Extend(blockB))
		require.NoError(t, ps.Extend(blockC))
		require.NoError(t, ps.Extend(blockD))
		require.NoError(t, ps.Finalize(blockA.ID()))

		es := newMockExecutionState(seal, genesis.Header)
		ctrl := gomock.NewController(t)
		headers := storage.NewMockHeaders(ctrl)
		headers.EXPECT().ByBlockID(genesis.ID()).Return(genesis.Header, nil)
		log := unittest.Logger()
		loader := loader.NewLoader(log, ps, headers, es)

		// block A is finalized, index its header by its height
		headers.EXPECT().ByHeight(blockA.Header.Height).Return(blockA.Header, nil)

		es.ExecuteBlock(t, blockA)
		es.ExecuteBlock(t, blockB)
		es.ExecuteBlock(t, blockC)
		es.ExecuteBlock(t, blockD)

		unexecuted, err := loader.LoadUnexecuted(context.Background())
		require.NoError(t, err)

		unittest.IDsEqual(t, []flow.Identifier{}, unexecuted)
	})

	t.Run("some fork is executed", func(t *testing.T) {
		ps := mocks.NewProtocolState()

		// Genesis <- A <- B <- C (finalized) <- D <- E <- F
		//                                       ^--- G <- H
		//                      ^-- I
		//						     ^--- J <- K
		chain, result, seal := unittest.ChainFixture(6)
		genesis, blockA, blockB, blockC, blockD, blockE, blockF :=
			chain[0], chain[1], chain[2], chain[3], chain[4], chain[5], chain[6]

		fork1 := unittest.ChainFixtureFrom(2, blockD.Header)
		blockG, blockH := fork1[0], fork1[1]

		fork2 := unittest.ChainFixtureFrom(1, blockC.Header)
		blockI := fork2[0]

		fork3 := unittest.ChainFixtureFrom(2, blockB.Header)
		blockJ, blockK := fork3[0], fork3[1]

		logChain(chain)
		logChain(fork1)
		logChain(fork2)
		logChain(fork3)

		require.NoError(t, ps.Bootstrap(genesis, result, seal))
		require.NoError(t, ps.Extend(blockA))
		require.NoError(t, ps.Extend(blockB))
		require.NoError(t, ps.Extend(blockC))
		require.NoError(t, ps.Extend(blockI))
		require.NoError(t, ps.Extend(blockJ))
		require.NoError(t, ps.Extend(blockK))
		require.NoError(t, ps.Extend(blockD))
		require.NoError(t, ps.Extend(blockE))
		require.NoError(t, ps.Extend(blockF))
		require.NoError(t, ps.Extend(blockG))
		require.NoError(t, ps.Extend(blockH))

		require.NoError(t, ps.Finalize(blockC.ID()))

		es := newMockExecutionState(seal, genesis.Header)
		ctrl := gomock.NewController(t)
		headers := storage.NewMockHeaders(ctrl)
		headers.EXPECT().ByBlockID(genesis.ID()).Return(genesis.Header, nil)
		log := unittest.Logger()
		loader := loader.NewLoader(log, ps, headers, es)

		// block C is finalized, index its header by its height
		headers.EXPECT().ByHeight(blockC.Header.Height).Return(blockC.Header, nil)

		es.ExecuteBlock(t, blockA)
		es.ExecuteBlock(t, blockB)
		es.ExecuteBlock(t, blockC)
		es.ExecuteBlock(t, blockD)
		es.ExecuteBlock(t, blockG)
		es.ExecuteBlock(t, blockJ)

		unexecuted, err := loader.LoadUnexecuted(context.Background())
		require.NoError(t, err)

		unittest.IDsEqual(t, []flow.Identifier{
			blockI.ID(), // I is still pending, and unexecuted
			blockE.ID(),
			blockF.ID(),
			// note K is not a pending block, but a conflicting block, even if it's not executed,
			// it won't included
			blockH.ID()},
			unexecuted)
	})
}
