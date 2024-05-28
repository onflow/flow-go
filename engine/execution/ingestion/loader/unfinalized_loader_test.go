package loader_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/ingestion"
	"github.com/onflow/flow-go/engine/execution/ingestion/loader"
	stateMock "github.com/onflow/flow-go/engine/execution/state/mock"
	"github.com/onflow/flow-go/model/flow"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

var _ ingestion.BlockLoader = (*loader.UnfinalizedLoader)(nil)

func TestLoadingUnfinalizedBlocks(t *testing.T) {
	ps := mocks.NewProtocolState()

	// Genesis <- A <- B <- C (finalized) <- D
	chain, result, seal := unittest.ChainFixture(5)
	genesis, blockA, blockB, blockC, blockD :=
		chain[0], chain[1], chain[2], chain[3], chain[4]

	logChain(chain)

	require.NoError(t, ps.Bootstrap(genesis, result, seal))
	require.NoError(t, ps.Extend(blockA))
	require.NoError(t, ps.Extend(blockB))
	require.NoError(t, ps.Extend(blockC))
	require.NoError(t, ps.Extend(blockD))
	require.NoError(t, ps.Finalize(blockC.ID()))

	es := new(stateMock.FinalizedExecutionState)
	es.On("GetHighestFinalizedExecuted").Return(genesis.Header.Height, nil)
	headers := new(storage.Headers)
	headers.On("BlockIDByHeight", blockA.Header.Height).Return(blockA.Header.ID(), nil)
	headers.On("BlockIDByHeight", blockB.Header.Height).Return(blockB.Header.ID(), nil)
	headers.On("BlockIDByHeight", blockC.Header.Height).Return(blockC.Header.ID(), nil)

	loader := loader.NewUnfinalizedLoader(unittest.Logger(), ps, headers, es)

	unexecuted, err := loader.LoadUnexecuted(context.Background())
	require.NoError(t, err)

	unittest.IDsEqual(t, []flow.Identifier{
		blockA.ID(),
		blockB.ID(),
		blockC.ID(),
		blockD.ID(),
	}, unexecuted)
}
