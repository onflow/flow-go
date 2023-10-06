package storehouse_test

import (
	"fmt"
	"testing"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

// GetRegister should fail for
// 1. unknown blockID
// 2. height lower than OnDiskRegisterStore's root height
// 3. height too high
// 4. known block, but unknown register
func TestRegisterStoreGetRegisterFail(t *testing.T) {
	withRegisterStore(t, func(
		t *testing.T,
		rs *storehouse.RegisterStore,
		diskStore execution.OnDiskRegisterStore,
		finalized execution.FinalizedReader,
		headerByHeight map[uint64]*flow.Header,
	) {
		rootHeight := uint64(10)
		// unknown block
		_, err := rs.GetRegister(rootHeight+1, unknownBlock, unknownReg.Key)
		require.Error(t, err)
		require.ErrorIs(t, err, storehouse.ErrNotExecuted)

		// too high
		block11 := headerByHeight[rootHeight+1]
		_, err = rs.GetRegister(rootHeight+1, block11.ID(), unknownReg.Key)
		require.Error(t, err)
		require.ErrorIs(t, err, storehouse.ErrNotExecuted)

		// lower than root height
		_, err = rs.GetRegister(rootHeight-1, unknownBlock, unknownReg.Key)
		require.Error(t, err)
		// TODO: enable it
		// require.ErrorIs(t, err, storehouse.ErrPruned)

		// known block, unknown register
		rootBlock := headerByHeight[rootHeight]
		_, err = rs.GetRegister(rootHeight, rootBlock.ID(), unknownReg.Key)
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

var unknownBlock = unittest.IdentifierFixture()
var unknownReg = makeReg("unknown", "unknown")

func makeReg(key string, value string) flow.RegisterEntry {
	return flow.RegisterEntry{
		Key: flow.RegisterID{
			Owner: "owner",
			Key:   key,
		},
		Value: []byte(value),
	}
}

type mockFinalizedReader struct {
	headerByHeight  map[uint64]*flow.Header
	lowest          uint64
	highest         uint64
	finalizedHeight *atomic.Uint64
}

func newMockFinalizedReader(initHeight uint64, count int) (*mockFinalizedReader, map[uint64]*flow.Header) {
	root := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(initHeight))
	blocks := unittest.ChainFixtureFrom(count, root)
	headerByHeight := make(map[uint64]*flow.Header, len(blocks)+1)
	headerByHeight[root.Height] = root

	for _, b := range blocks {
		headerByHeight[b.Header.Height] = b.Header
	}

	return &mockFinalizedReader{
		headerByHeight:  headerByHeight,
		lowest:          initHeight,
		highest:         blocks[len(blocks)-1].Header.Height,
		finalizedHeight: atomic.NewUint64(initHeight),
	}, headerByHeight
}

func (r *mockFinalizedReader) GetFinalizedBlockIDAtHeight(height uint64) (flow.Identifier, error) {
	finalized := r.finalizedHeight.Load()
	if height > finalized {
		return flow.Identifier{}, storage.ErrNotFound
	}

	return r.headerByHeight[height].ID(), nil
}

func (r *mockFinalizedReader) MockFinal(height uint64) error {
	if height < r.lowest || height > r.highest {
		return fmt.Errorf("height %d is out of range [%d, %d]", height, r.lowest, r.highest)
	}

	r.finalizedHeight.Store(height)
	return nil
}

func withRegisterStore(t *testing.T, fn func(
	t *testing.T,
	rs *storehouse.RegisterStore,
	diskStore execution.OnDiskRegisterStore,
	finalized execution.FinalizedReader,
	headers map[uint64]*flow.Header,
)) {
	pebble.RunWithRegistersStorageAtInitialHeights(t, 10, 10, func(diskStore *pebble.Registers) {
		log := unittest.Logger()
		var wal execution.ExecutedFinalizedWAL
		finalized, headerByHeight := newMockFinalizedReader(10, 100)
		rs, err := storehouse.NewRegisterStore(diskStore, wal, finalized, log)
		require.NoError(t, err)
		fn(t, rs, diskStore, finalized, headerByHeight)
	})
}

// SaveRegisters should fail for
// 1. mismatching parent
// 2. saved block

// SaveRegisters should ok, and
// 1. GetRegister can get saved registers,
// 2. FinalizedAndExecutedHeight should be updated
// 3. IsBlockExecuted should return true

// if SaveRegisters with empty register, then
// 1. FinalizedAndExecutedHeight should be updated
// 2. IsBlockExecuted should return true

// if 10 is latest finalized, then
// 1. IsBlockExecuted should return true for finalized block 9
// 2. IsBlockExecuted should return false for conflicting block 9
// 4. IsBlockExecuted should return true for executed and unfinalized block 11
// 3. IsBlockExecuted should return false for unexecuted block 12

// Execute first then finalize later
// SaveRegisters(1), SaveRegisters(2), SaveRegisters(3), then
// OnBlockFinalized(1), OnBlockFinalized(2), OnBlockFinalized(3) should
// 1. update FinalizedAndExecutedHeight
// 2. InMemoryRegisterStore should have correct pruned height
// 3. NewRegisterStore with the same OnDiskRegisterStore again should return correct FinalizedAndExecutedHeight

// Finalize first then execute later
// OnBlockFinalized(1), OnBlockFinalized(2), OnBlockFinalized(3), then
// SaveRegisters(1), SaveRegisters(2), SaveRegisters(3) should
// 1. update FinalizedAndExecutedHeight
// 2. InMemoryRegisterStore should have correct pruned height
// 3. NewRegisterStore with the same OnDiskRegisterStore again should return correct FinalizedAndExecutedHeight

// Finalize and Execute concurrently
// SaveRegisters(1), SaveRegisters(2), ... SaveRegisters(100), happen concurrently with
// OnBlockFinalized(1), OnBlockFinalized(2), ... OnBlockFinalized(100), should update FinalizedAndExecutedHeight

//
