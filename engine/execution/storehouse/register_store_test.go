package storehouse_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
)

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
		// TODO: enable it once implemented
		// require.ErrorIs(t, err, storehouse.ErrPruned)

		// known block, unknown register
		rootBlock := headerByHeight[rootHeight]
		val, err := rs.GetRegister(rootHeight, rootBlock.ID(), unknownReg.Key)
		require.NoError(t, err)
		require.Nil(t, val)
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
		err := rs.SaveRegisters(wrongParent, flow.RegisterEntries{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "parent")

		err = rs.SaveRegisters(headerByHeight[rootHeight], flow.RegisterEntries{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "pruned")
	})
}

// SaveRegisters should ok, and
// 1. GetRegister can get saved registers,
// 2. IsBlockExecuted should return true
//
// if SaveRegisters with empty register, then
// 1. LastFinalizedAndExecutedHeight should be updated
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
		err = rs.SaveRegisters(headerByHeight[rootHeight+1], flow.RegisterEntries{reg})
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
		err = rs.SaveRegisters(headerByHeight[rootHeight+2], flow.RegisterEntries{})
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
		err := rs.SaveRegisters(headerByHeight[rootHeight+1], flow.RegisterEntries{reg})
		require.NoError(t, err)

		// save block 12
		err = rs.SaveRegisters(headerByHeight[rootHeight+2], flow.RegisterEntries{makeReg("X", "2")})
		require.NoError(t, err)

		require.NoError(t, finalized.MockFinal(rootHeight+1))

		require.NoError(t, rs.OnBlockFinalized()) // notify 11 is finalized

		require.Equal(t, rootHeight+1, rs.LastFinalizedAndExecutedHeight())

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

// Test reading registers from finalized block
func TestRegisterStoreReadingFromDisk(t *testing.T) {
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

		// R <- 11 (X: 1, Y: 2) <- 12 (Y: 3) <- 13 (X: 4)
		// save block 11
		err := rs.SaveRegisters(headerByHeight[rootHeight+1], flow.RegisterEntries{makeReg("X", "1"), makeReg("Y", "2")})
		require.NoError(t, err)

		// save block 12
		err = rs.SaveRegisters(headerByHeight[rootHeight+2], flow.RegisterEntries{makeReg("Y", "3")})
		require.NoError(t, err)

		// save block 13
		err = rs.SaveRegisters(headerByHeight[rootHeight+3], flow.RegisterEntries{makeReg("X", "4")})
		require.NoError(t, err)

		require.NoError(t, finalized.MockFinal(rootHeight+2))
		require.NoError(t, rs.OnBlockFinalized()) // notify 12 is finalized

		val, err := rs.GetRegister(rootHeight+1, headerByHeight[rootHeight+1].ID(), makeReg("Y", "2").Key)
		require.NoError(t, err)
		// value at block 11 is now stored in OnDiskRegisterStore, which is 2
		require.Equal(t, makeReg("Y", "2").Value, val)

		val, err = rs.GetRegister(rootHeight+2, headerByHeight[rootHeight+2].ID(), makeReg("X", "1").Key)
		require.NoError(t, err)
		// value at block 12 is now stored in OnDiskRegisterStore, which is 1
		require.Equal(t, makeReg("X", "1").Value, val)

		val, err = rs.GetRegister(rootHeight+3, headerByHeight[rootHeight+3].ID(), makeReg("Y", "3").Key)
		require.NoError(t, err)
		// value at block 13 was stored in OnDiskRegisterStore at block 12, which is 3
		require.Equal(t, makeReg("Y", "3").Value, val)

		_, err = rs.GetRegister(rootHeight+4, headerByHeight[rootHeight+4].ID(), makeReg("Y", "3").Key)
		require.Error(t, err)
	})
}

func TestRegisterStoreReadingFromInMemStore(t *testing.T) {
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

		// R <- 11 (X: 1, Y: 2) <- 12 (Y: 3)
		//   ^- 11 (X: 4)

		// save block 11
		err := rs.SaveRegisters(headerByHeight[rootHeight+1], flow.RegisterEntries{makeReg("X", "1"), makeReg("Y", "2")})
		require.NoError(t, err)

		// save block 12
		err = rs.SaveRegisters(headerByHeight[rootHeight+2], flow.RegisterEntries{makeReg("Y", "3")})
		require.NoError(t, err)

		// save block 11 fork
		block11Fork := unittest.BlockWithParentFixture(headerByHeight[rootHeight]).Header
		err = rs.SaveRegisters(block11Fork, flow.RegisterEntries{makeReg("X", "4")})
		require.NoError(t, err)

		val, err := rs.GetRegister(rootHeight+1, headerByHeight[rootHeight+1].ID(), makeReg("X", "1").Key)
		require.NoError(t, err)
		require.Equal(t, makeReg("X", "1").Value, val)

		val, err = rs.GetRegister(rootHeight+1, headerByHeight[rootHeight+1].ID(), makeReg("Y", "2").Key)
		require.NoError(t, err)
		require.Equal(t, makeReg("Y", "2").Value, val)

		val, err = rs.GetRegister(rootHeight+2, headerByHeight[rootHeight+2].ID(), makeReg("X", "1").Key)
		require.NoError(t, err)
		require.Equal(t, makeReg("X", "1").Value, val)

		val, err = rs.GetRegister(rootHeight+2, headerByHeight[rootHeight+2].ID(), makeReg("Y", "3").Key)
		require.NoError(t, err)
		require.Equal(t, makeReg("Y", "3").Value, val)

		val, err = rs.GetRegister(rootHeight+1, block11Fork.ID(), makeReg("X", "4").Key)
		require.NoError(t, err)
		require.Equal(t, makeReg("X", "4").Value, val)

		// finalizing 11 should prune block 11 fork, and won't be able to read register from block 11 fork
		require.NoError(t, finalized.MockFinal(rootHeight+1))
		require.NoError(t, rs.OnBlockFinalized()) // notify 11 is finalized

		val, err = rs.GetRegister(rootHeight+1, block11Fork.ID(), makeReg("X", "4").Key)
		require.Error(t, err, fmt.Sprintf("%v", val))
		// pruned conflicting forks are considered not executed
		require.ErrorIs(t, err, storehouse.ErrNotExecuted)
	})
}

// Execute first then finalize later
// SaveRegisters(1), SaveRegisters(2), SaveRegisters(3), then
// OnBlockFinalized(1), OnBlockFinalized(2), OnBlockFinalized(3) should
// 1. update LastFinalizedAndExecutedHeight
// 2. InMemoryRegisterStore should have correct pruned height
// 3. NewRegisterStore with the same OnDiskRegisterStore again should return correct LastFinalizedAndExecutedHeight
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
		err := rs.SaveRegisters(headerByHeight[rootHeight+1], flow.RegisterEntries{makeReg("X", "1")})
		require.NoError(t, err)
		require.Equal(t, rootHeight, rs.LastFinalizedAndExecutedHeight())

		// save block 12
		err = rs.SaveRegisters(headerByHeight[rootHeight+2], flow.RegisterEntries{makeReg("X", "2")})
		require.NoError(t, err)
		require.Equal(t, rootHeight, rs.LastFinalizedAndExecutedHeight())

		// save block 13
		err = rs.SaveRegisters(headerByHeight[rootHeight+3], flow.RegisterEntries{makeReg("X", "3")})
		require.NoError(t, err)
		require.Equal(t, rootHeight, rs.LastFinalizedAndExecutedHeight())

		require.NoError(t, finalized.MockFinal(rootHeight+1))
		require.NoError(t, rs.OnBlockFinalized()) // notify 11 is finalized
		require.Equal(t, rootHeight+1, rs.LastFinalizedAndExecutedHeight())

		require.NoError(t, finalized.MockFinal(rootHeight+2))
		require.NoError(t, rs.OnBlockFinalized()) // notify 12 is finalized
		require.Equal(t, rootHeight+2, rs.LastFinalizedAndExecutedHeight())

		require.NoError(t, finalized.MockFinal(rootHeight+3))
		require.NoError(t, rs.OnBlockFinalized()) // notify 13 is finalized
		require.Equal(t, rootHeight+3, rs.LastFinalizedAndExecutedHeight())
	})
}

// Finalize first then execute later
// OnBlockFinalized(1), OnBlockFinalized(2), OnBlockFinalized(3), then
// SaveRegisters(1), SaveRegisters(2), SaveRegisters(3) should
// 1. update LastFinalizedAndExecutedHeight
// 2. InMemoryRegisterStore should have correct pruned height
// 3. NewRegisterStore with the same OnDiskRegisterStore again should return correct LastFinalizedAndExecutedHeight
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
		require.NoError(t, rs.OnBlockFinalized()) // notify 11 is finalized
		require.Equal(t, rootHeight, rs.LastFinalizedAndExecutedHeight(), fmt.Sprintf("LastFinalizedAndExecutedHeight: %d", rs.LastFinalizedAndExecutedHeight()))

		require.NoError(t, finalized.MockFinal(rootHeight+2))
		require.NoError(t, rs.OnBlockFinalized()) // notify 12 is finalized
		require.Equal(t, rootHeight, rs.LastFinalizedAndExecutedHeight(), fmt.Sprintf("LastFinalizedAndExecutedHeight: %d", rs.LastFinalizedAndExecutedHeight()))

		require.NoError(t, finalized.MockFinal(rootHeight+3))
		require.NoError(t, rs.OnBlockFinalized()) // notify 13 is finalized
		require.Equal(t, rootHeight, rs.LastFinalizedAndExecutedHeight())

		// save block 11
		err := rs.SaveRegisters(headerByHeight[rootHeight+1], flow.RegisterEntries{makeReg("X", "1")})
		require.NoError(t, err)
		require.Equal(t, rootHeight+1, rs.LastFinalizedAndExecutedHeight())

		// save block 12
		err = rs.SaveRegisters(headerByHeight[rootHeight+2], flow.RegisterEntries{makeReg("X", "2")})
		require.NoError(t, err)
		require.Equal(t, rootHeight+2, rs.LastFinalizedAndExecutedHeight())

		// save block 13
		err = rs.SaveRegisters(headerByHeight[rootHeight+3], flow.RegisterEntries{makeReg("X", "3")})
		require.NoError(t, err)
		require.Equal(t, rootHeight+3, rs.LastFinalizedAndExecutedHeight())
	})
}

// Finalize and Execute concurrently
// SaveRegisters(1), SaveRegisters(2), ... SaveRegisters(100), happen concurrently with
// OnBlockFinalized(1), OnBlockFinalized(2), ... OnBlockFinalized(100), should update LastFinalizedAndExecutedHeight
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

		wg.Add(1)
		go func() {
			defer wg.Done()

			for savedHeight := range savedHeights {
				err := finalized.MockFinal(savedHeight)
				require.NoError(t, err)
				require.NoError(t, rs.OnBlockFinalized(), fmt.Sprintf("saved height %v", savedHeight))
			}
		}()

		for height := rootHeight + 1; height <= endHeight; height++ {
			if height >= 50 {
				savedHeights <- height
			}

			err := rs.SaveRegisters(headerByHeight[height], flow.RegisterEntries{makeReg("X", fmt.Sprintf("%d", height))})
			require.NoError(t, err)
		}
		close(savedHeights)

		wg.Wait() // wait until all heights are finalized

		// after all heights are executed and finalized, the LastFinalizedAndExecutedHeight should be the last height
		require.Equal(t, endHeight, rs.LastFinalizedAndExecutedHeight())
	})
}
