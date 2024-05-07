package environment_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	storageErr "github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBlockInfo(t *testing.T) {
	tracer := tracing.NewMockTracerSpan()
	meter := &util.NopMeter{}
	blocks := &mockBlocks{
		blocks: make(map[uint64]*flow.Header),
	}
	height := uint64(flow.DefaultTransactionExpiry)
	header := unittest.BlockHeaderWithHeight(height)

	bi := environment.NewBlockInfo(tracer, meter, header, blocks)

	// verify the current block exists
	blocks.Add(header)
	b, exists, err := bi.GetBlockAtHeight(height)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, header.Height, b.Height)

	// verify blocks that do not exist
	b, exists, err = bi.GetBlockAtHeight(height + 1)
	require.NoError(t, err)
	require.False(t, exists)

	// verify that the block at the height before the lowest accepted height exists
	lowestAcceptedHeight := height - flow.DefaultTransactionExpiry
	lowestHeader := unittest.BlockHeaderWithHeight(lowestAcceptedHeight)
	blocks.Add(lowestHeader)
	b, exists, err = bi.GetBlockAtHeight(lowestAcceptedHeight)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, lowestHeader.Height, b.Height)

	// verify that the block at the height before the lowest accepted height does not exist
	_, exists, err = bi.GetBlockAtHeight(lowestAcceptedHeight - 1)
	require.NoError(t, err)
	require.False(t, exists)
}

type mockBlocks struct {
	blocks map[uint64]*flow.Header
}

func (m *mockBlocks) ByHeightFrom(height uint64, header *flow.Header) (*flow.Header, error) {
	h, ok := m.blocks[height]
	if !ok {
		return nil, fmt.Errorf("block does not exist: %w", storageErr.ErrNotFound)
	}
	return h, nil
}

func (m *mockBlocks) Add(h *flow.Header) {
	m.blocks[h.Height] = h
}
