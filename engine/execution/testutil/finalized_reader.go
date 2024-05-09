package testutil

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

type MockFinalizedReader struct {
	headerByHeight  map[uint64]*flow.Header
	blockByHeight   map[uint64]*flow.Block
	lowest          uint64
	highest         uint64
	finalizedHeight *atomic.Uint64
	finalizedCalled *atomic.Int64
}

func NewMockFinalizedReader(initHeight uint64, count int) (*MockFinalizedReader, map[uint64]*flow.Header, uint64) {
	root := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(initHeight))
	blocks := unittest.ChainFixtureFrom(count, root)
	headerByHeight := make(map[uint64]*flow.Header, len(blocks)+1)
	headerByHeight[root.Height] = root

	blockByHeight := make(map[uint64]*flow.Block, len(blocks)+1)
	for _, b := range blocks {
		headerByHeight[b.Header.Height] = b.Header
		blockByHeight[b.Header.Height] = b
	}

	highest := blocks[len(blocks)-1].Header.Height
	return &MockFinalizedReader{
		headerByHeight:  headerByHeight,
		blockByHeight:   blockByHeight,
		lowest:          initHeight,
		highest:         highest,
		finalizedHeight: atomic.NewUint64(initHeight),
		finalizedCalled: atomic.NewInt64(0),
	}, headerByHeight, highest
}

func (r *MockFinalizedReader) FinalizedBlockIDAtHeight(height uint64) (flow.Identifier, error) {
	r.finalizedCalled.Add(1)
	finalized := r.finalizedHeight.Load()
	if height > finalized {
		return flow.Identifier{}, storage.ErrNotFound
	}

	if height < r.lowest {
		return flow.ZeroID, fmt.Errorf("height %d is out of range [%d, %d]", height, r.lowest, r.highest)
	}
	return r.headerByHeight[height].ID(), nil
}

func (r *MockFinalizedReader) MockFinal(height uint64) error {
	if height < r.lowest || height > r.highest {
		return fmt.Errorf("height %d is out of range [%d, %d]", height, r.lowest, r.highest)
	}

	r.finalizedHeight.Store(height)
	return nil
}

func (r *MockFinalizedReader) BlockAtHeight(height uint64) *flow.Block {
	return r.blockByHeight[height]
}

func (r *MockFinalizedReader) FinalizedCalled() int {
	return int(r.finalizedCalled.Load())
}
