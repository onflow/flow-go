package storehouse_test

import (
	"fmt"
	"sync"
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
	t.Parallel()
	withRegisterStore(t, func(
		t *testing.T,
		rs *storehouse.RegisterStore,
		diskStore execution.OnDiskRegisterStore,
		finalized *mockFinalizedReader,
		rootHeight uint64,
		endHeight uint64,
		headerByHeight map[uint64]*flow.Header,
	) {
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

func newMockFinalizedReader(initHeight uint64, count int) (*mockFinalizedReader, map[uint64]*flow.Header, uint64) {
	root := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(initHeight))
	blocks := unittest.ChainFixtureFrom(count, root)
	headerByHeight := make(map[uint64]*flow.Header, len(blocks)+1)
	headerByHeight[root.Height] = root

	for _, b := range blocks {
		headerByHeight[b.Header.Height] = b.Header
	}

	highest := blocks[len(blocks)-1].Header.Height
	return &mockFinalizedReader{
		headerByHeight:  headerByHeight,
		lowest:          initHeight,
		highest:         highest,
		finalizedHeight: atomic.NewUint64(initHeight),
	}, headerByHeight, highest
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
	finalized *mockFinalizedReader,
	rootHeight uint64,
	endHeight uint64,
	headers map[uint64]*flow.Header,
)) {
	pebble.RunWithRegistersStorageAtInitialHeights(t, 10, 10, func(diskStore *pebble.Registers) {
		log := unittest.Logger()
		var wal execution.ExecutedFinalizedWAL
		finalized, headerByHeight, highest := newMockFinalizedReader(10, 100)
		rs, err := storehouse.NewRegisterStore(diskStore, wal, finalized, log)
		require.NoError(t, err)
		fn(t, rs, diskStore, finalized, 10, highest, headerByHeight)
	})
}

// SaveRegisters should fail for
// 1. mismatching parent
// 2. saved block
func TestRegisterStoreSaveRegistersShouldFail(t *testing.T) {
	t.Parallel()
	withRegisterStore(t, func(
		t *testing.T,
		rs *storehouse.RegisterStore,
		diskStore execution.OnDiskRegisterStore,
		finalized *mockFinalizedReader,
		rootHeight uint64,
		endHeight uint64,
		headerByHeight map[uint64]*flow.Header,
	) {
		wrongParent := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(rootHeight + 1))
		err := rs.SaveRegisters(wrongParent, []flow.RegisterEntry{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "parent")

		err = rs.SaveRegisters(headerByHeight[rootHeight], []flow.RegisterEntry{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "pruned")
	})
}

// SaveRegisters should ok, and
// 1. GetRegister can get saved registers,
// 2. IsBlockExecuted should return true
//
// if SaveRegisters with empty register, then
// 1. FinalizedAndExecutedHeight should be updated
// 2. IsBlockExecuted should return true
func TestRegisterStoreSaveRegistersShouldOK(t *testing.T) {
	t.Parallel()
	withRegisterStore(t, func(
		t *testing.T,
		rs *storehouse.RegisterStore,
		diskStore execution.OnDiskRegisterStore,
		finalized *mockFinalizedReader,
		rootHeight uint64,
		endHeight uint64,
		headerByHeight map[uint64]*flow.Header,
	) {
		// not executed
		executed, err := rs.IsBlockExecuted(rootHeight+1, headerByHeight[rootHeight+1].ID())
		require.NoError(t, err)
		require.False(t, executed)

		// save block 11
		reg := makeReg("X", "1")
		err = rs.SaveRegisters(headerByHeight[rootHeight+1], []flow.RegisterEntry{reg})
		require.NoError(t, err)

		// should get value
		val, err := rs.GetRegister(rootHeight+1, headerByHeight[rootHeight+1].ID(), reg.Key)
		require.NoError(t, err)
		require.Equal(t, reg.Value, val)

		// should become executed
		executed, err = rs.IsBlockExecuted(rootHeight+1, headerByHeight[rootHeight+1].ID())
		require.NoError(t, err)
		require.True(t, executed)

		// block 12 is empty
		err = rs.SaveRegisters(headerByHeight[rootHeight+2], []flow.RegisterEntry{})
		require.NoError(t, err)

		// should get same value
		val, err = rs.GetRegister(rootHeight+1, headerByHeight[rootHeight+2].ID(), reg.Key)
		require.NoError(t, err)
		require.Equal(t, reg.Value, val)

		// should become executed
		executed, err = rs.IsBlockExecuted(rootHeight+1, headerByHeight[rootHeight+2].ID())
		require.NoError(t, err)
		require.True(t, executed)
	})
}

// if 11 is latest finalized, then
// 1. IsBlockExecuted should return true for finalized block 10
// 2. IsBlockExecuted should return false for conflicting block 10
// 4. IsBlockExecuted should return true for executed and unfinalized block 12
// 3. IsBlockExecuted should return false for unexecuted block 13
func TestRegisterStoreIsBlockExecuted(t *testing.T) {
	t.Parallel()
	withRegisterStore(t, func(
		t *testing.T,
		rs *storehouse.RegisterStore,
		diskStore execution.OnDiskRegisterStore,
		finalized *mockFinalizedReader,
		rootHeight uint64,
		endHeight uint64,
		headerByHeight map[uint64]*flow.Header,
	) {
		// save block 11
		reg := makeReg("X", "1")
		err := rs.SaveRegisters(headerByHeight[rootHeight+1], []flow.RegisterEntry{reg})
		require.NoError(t, err)

		// save block 12
		err = rs.SaveRegisters(headerByHeight[rootHeight+2], []flow.RegisterEntry{makeReg("X", "2")})
		require.NoError(t, err)

		require.NoError(t, finalized.MockFinal(rootHeight+1))

		rs.OnBlockFinalized() // notify 11 is finalized

		require.Equal(t, rootHeight+1, rs.FinalizedAndExecutedHeight())

		executed, err := rs.IsBlockExecuted(rootHeight, headerByHeight[rootHeight].ID())
		require.NoError(t, err)
		require.True(t, executed)

		executed, err = rs.IsBlockExecuted(rootHeight+1, headerByHeight[rootHeight+1].ID())
		require.NoError(t, err)
		require.True(t, executed)

		executed, err = rs.IsBlockExecuted(rootHeight+2, headerByHeight[rootHeight+2].ID())
		require.NoError(t, err)
		require.True(t, executed)

		executed, err = rs.IsBlockExecuted(rootHeight+3, headerByHeight[rootHeight+3].ID())
		require.NoError(t, err)
		require.False(t, executed)
	})

}

// Execute first then finalize later
// SaveRegisters(1), SaveRegisters(2), SaveRegisters(3), then
// OnBlockFinalized(1), OnBlockFinalized(2), OnBlockFinalized(3) should
// 1. update FinalizedAndExecutedHeight
// 2. InMemoryRegisterStore should have correct pruned height
// 3. NewRegisterStore with the same OnDiskRegisterStore again should return correct FinalizedAndExecutedHeight
func TestRegisterStoreExecuteFirstFinalizeLater(t *testing.T) {
	t.Parallel()
	withRegisterStore(t, func(
		t *testing.T,
		rs *storehouse.RegisterStore,
		diskStore execution.OnDiskRegisterStore,
		finalized *mockFinalizedReader,
		rootHeight uint64,
		endHeight uint64,
		headerByHeight map[uint64]*flow.Header,
	) {
		// save block 11
		err := rs.SaveRegisters(headerByHeight[rootHeight+1], []flow.RegisterEntry{makeReg("X", "1")})
		require.NoError(t, err)
		require.Equal(t, rootHeight, rs.FinalizedAndExecutedHeight())

		// save block 12
		err = rs.SaveRegisters(headerByHeight[rootHeight+2], []flow.RegisterEntry{makeReg("X", "2")})
		require.NoError(t, err)
		require.Equal(t, rootHeight, rs.FinalizedAndExecutedHeight())

		// save block 13
		err = rs.SaveRegisters(headerByHeight[rootHeight+3], []flow.RegisterEntry{makeReg("X", "3")})
		require.NoError(t, err)
		require.Equal(t, rootHeight, rs.FinalizedAndExecutedHeight())

		require.NoError(t, finalized.MockFinal(rootHeight+1))
		rs.OnBlockFinalized() // notify 11 is finalized
		require.Equal(t, rootHeight+1, rs.FinalizedAndExecutedHeight())

		require.NoError(t, finalized.MockFinal(rootHeight+2))
		rs.OnBlockFinalized() // notify 12 is finalized
		require.Equal(t, rootHeight+2, rs.FinalizedAndExecutedHeight())

		require.NoError(t, finalized.MockFinal(rootHeight+3))
		rs.OnBlockFinalized() // notify 13 is finalized
		require.Equal(t, rootHeight+3, rs.FinalizedAndExecutedHeight())
	})
}

// Finalize first then execute later
// OnBlockFinalized(1), OnBlockFinalized(2), OnBlockFinalized(3), then
// SaveRegisters(1), SaveRegisters(2), SaveRegisters(3) should
// 1. update FinalizedAndExecutedHeight
// 2. InMemoryRegisterStore should have correct pruned height
// 3. NewRegisterStore with the same OnDiskRegisterStore again should return correct FinalizedAndExecutedHeight
func TestRegisterStoreFinalizeFirstExecuteLater(t *testing.T) {
	t.Parallel()
	withRegisterStore(t, func(
		t *testing.T,
		rs *storehouse.RegisterStore,
		diskStore execution.OnDiskRegisterStore,
		finalized *mockFinalizedReader,
		rootHeight uint64,
		endHeight uint64,
		headerByHeight map[uint64]*flow.Header,
	) {
		require.NoError(t, finalized.MockFinal(rootHeight+1))
		rs.OnBlockFinalized() // notify 11 is finalized
		require.Equal(t, rootHeight, rs.FinalizedAndExecutedHeight(), fmt.Sprintf("FinalizedAndExecutedHeight: %d", rs.FinalizedAndExecutedHeight()))

		require.NoError(t, finalized.MockFinal(rootHeight+2))
		rs.OnBlockFinalized() // notify 12 is finalized
		require.Equal(t, rootHeight, rs.FinalizedAndExecutedHeight(), fmt.Sprintf("FinalizedAndExecutedHeight: %d", rs.FinalizedAndExecutedHeight()))

		require.NoError(t, finalized.MockFinal(rootHeight+3))
		rs.OnBlockFinalized() // notify 13 is finalized
		require.Equal(t, rootHeight, rs.FinalizedAndExecutedHeight())

		// save block 11
		err := rs.SaveRegisters(headerByHeight[rootHeight+1], []flow.RegisterEntry{makeReg("X", "1")})
		require.NoError(t, err)
		require.Equal(t, rootHeight+1, rs.FinalizedAndExecutedHeight())

		// save block 12
		err = rs.SaveRegisters(headerByHeight[rootHeight+2], []flow.RegisterEntry{makeReg("X", "2")})
		require.NoError(t, err)
		require.Equal(t, rootHeight+2, rs.FinalizedAndExecutedHeight())

		// save block 13
		err = rs.SaveRegisters(headerByHeight[rootHeight+3], []flow.RegisterEntry{makeReg("X", "3")})
		require.NoError(t, err)
		require.Equal(t, rootHeight+3, rs.FinalizedAndExecutedHeight())
	})
}

// Finalize and Execute concurrently
// SaveRegisters(1), SaveRegisters(2), ... SaveRegisters(100), happen concurrently with
// OnBlockFinalized(1), OnBlockFinalized(2), ... OnBlockFinalized(100), should update FinalizedAndExecutedHeight
func TestRegisterStoreConcurrentFinalizeAndExecute(t *testing.T) {
	t.Parallel()
	withRegisterStore(t, func(
		t *testing.T,
		rs *storehouse.RegisterStore,
		diskStore execution.OnDiskRegisterStore,
		finalized *mockFinalizedReader,
		rootHeight uint64,
		endHeight uint64,
		headerByHeight map[uint64]*flow.Header,
	) {

		var wg sync.WaitGroup
		savedHeights := make(chan uint64, len(headerByHeight)) // enough buffer so that producer won't be blocked

		go func() {
			wg.Add(1)
			defer wg.Done()

			for savedHeight := range savedHeights {
				err := finalized.MockFinal(savedHeight)
				require.NoError(t, err)
				require.NoError(t, rs.OnBlockFinalized(), fmt.Sprintf("saved height %v", savedHeight))
			}
		}()

		for height := rootHeight + 1; height <= endHeight; height++ {
			if height >= 10 {
				savedHeights <- height
			}

			err := rs.SaveRegisters(headerByHeight[height], []flow.RegisterEntry{makeReg("X", fmt.Sprintf("%d", height))})
			require.NoError(t, err)
		}
		close(savedHeights)

		wg.Wait() // wait until all heights are finalized

		// after all heights are executed and finalized, the FinalizedAndExecutedHeight should be the last height
		require.Equal(t, endHeight, rs.FinalizedAndExecutedHeight())
	})
}
