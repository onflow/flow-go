package storehouse

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// 1. SaveRegisters should fail if height is below or equal to pruned height
func TestInMemoryRegisterStore(t *testing.T) {
	t.Run("FailBelowOrEqualPrunedHeight", func(t *testing.T) {
		t.Parallel()
		// 1.
		pruned := uint64(10)
		lastID := unittest.IdentifierFixture()
		store := NewInMemoryRegisterStore(pruned, lastID)
		err := store.SaveRegisters(
			pruned-1, // below pruned pruned, will fail
			unittest.IdentifierFixture(),
			unittest.IdentifierFixture(),
			flow.RegisterEntries{},
		)
		require.Error(t, err)
		require.Contains(t, err.Error(), "<= pruned height")

		err = store.SaveRegisters(
			pruned, // equal to pruned height, will fail
			lastID,
			unittest.IdentifierFixture(),
			flow.RegisterEntries{},
		)
		require.Error(t, err)
		require.Contains(t, err.Error(), "<= pruned height")
	})

	//  2. SaveRegisters should fail if its parent block doesn't exist and it is not the pruned block
	//     SaveRegisters should succeed if height is above pruned height and block is not saved,
	//     the updates can be retrieved by GetUpdatedRegisters
	//     GetRegister should return PrunedError if the queried key is not updated since pruned height
	//     GetRegister should return PrunedError if the queried height is below pruned height
	//     GetRegister should return ErrNotExecuted if the block is unknown
	t.Run("FailParentNotExist", func(t *testing.T) {
		t.Parallel()
		pruned := uint64(10)
		lastID := unittest.IdentifierFixture()
		store := NewInMemoryRegisterStore(pruned, lastID)

		height := pruned + 1 // above the pruned pruned
		blockID := unittest.IdentifierFixture()
		notExistParent := unittest.IdentifierFixture()
		reg := unittest.RegisterEntryFixture()
		err := store.SaveRegisters(
			height,
			blockID,
			notExistParent, // should fail because parent doesn't exist
			flow.RegisterEntries{reg},
		)
		require.Error(t, err)
		require.Contains(t, err.Error(), "but its parent")
	})

	t.Run("StoreOK", func(t *testing.T) {
		t.Parallel()
		// 3.
		pruned := uint64(10)
		lastID := unittest.IdentifierFixture()
		store := NewInMemoryRegisterStore(pruned, lastID)

		height := pruned + 1 // above the pruned pruned
		blockID := unittest.IdentifierFixture()
		reg := unittest.RegisterEntryFixture()
		err := store.SaveRegisters(
			height,
			blockID,
			lastID,
			flow.RegisterEntries{reg},
		)
		require.NoError(t, err)

		val, err := store.GetRegister(height, blockID, reg.Key)
		require.NoError(t, err)
		require.Equal(t, reg.Value, val)

		// unknown key
		_, err = store.GetRegister(height, blockID, unknownKey)
		require.Error(t, err)
		pe, ok := IsPrunedError(err)
		require.True(t, ok)
		require.Equal(t, pe.PrunedHeight, pruned)
		require.Equal(t, pe.Height, height)

		// unknown block with unknown height
		_, err = store.GetRegister(height+1, unknownBlock, reg.Key)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrNotExecuted)

		// unknown block with known height
		_, err = store.GetRegister(height, unknownBlock, reg.Key)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrNotExecuted)

		// too low height
		_, err = store.GetRegister(height-1, unknownBlock, reg.Key)
		require.Error(t, err)
		pe, ok = IsPrunedError(err)
		require.True(t, ok)
		require.Equal(t, pe.PrunedHeight, pruned)
		require.Equal(t, pe.Height, height-1)
	})

	// 3. SaveRegisters should fail if the block is already saved
	t.Run("StoreFailAlreadyExist", func(t *testing.T) {
		t.Parallel()
		pruned := uint64(10)
		lastID := unittest.IdentifierFixture()
		store := NewInMemoryRegisterStore(pruned, lastID)

		height := pruned + 1 // above the pruned pruned
		blockID := unittest.IdentifierFixture()
		reg := unittest.RegisterEntryFixture()
		err := store.SaveRegisters(
			height,
			blockID,
			lastID,
			flow.RegisterEntries{reg},
		)
		require.NoError(t, err)

		// saving again should fail
		err = store.SaveRegisters(
			height,
			blockID,
			lastID,
			flow.RegisterEntries{reg},
		)
		require.Error(t, err)
		require.Contains(t, err.Error(), "already exists")
	})

	//  4. SaveRegisters should succeed if a different block at the same height was saved before,
	//     updates for different blocks can be retrieved by their blockID
	t.Run("StoreOKDifferentBlockSameParent", func(t *testing.T) {
		t.Parallel()
		pruned := uint64(10)
		lastID := unittest.IdentifierFixture()
		store := NewInMemoryRegisterStore(pruned, lastID)

		// 10 <- A
		//    ^- B
		height := pruned + 1 // above the pruned pruned
		blockA := unittest.IdentifierFixture()
		regA := unittest.RegisterEntryFixture()
		err := store.SaveRegisters(
			height,
			blockA,
			lastID,
			flow.RegisterEntries{regA},
		)
		require.NoError(t, err)

		blockB := unittest.IdentifierFixture()
		regB := unittest.RegisterEntryFixture()
		err = store.SaveRegisters(
			height,
			blockB, // different block
			lastID, // same parent
			flow.RegisterEntries{regB},
		)
		require.NoError(t, err)

		valA, err := store.GetRegister(height, blockA, regA.Key)
		require.NoError(t, err)
		require.Equal(t, regA.Value, valA)

		valB, err := store.GetRegister(height, blockB, regB.Key)
		require.NoError(t, err)
		require.Equal(t, regB.Value, valB)
	})

	// 5. Given A(X: 1, Y: 2), GetRegister(A, X) should return 1, GetRegister(A, X) should return 2
	t.Run("GetRegistersOK", func(t *testing.T) {
		t.Parallel()
		pruned := uint64(10)
		lastID := unittest.IdentifierFixture()
		store := NewInMemoryRegisterStore(pruned, lastID)

		// 10 <- A (X: 1, Y: 2)
		height := pruned + 1 // above the pruned pruned
		blockA := unittest.IdentifierFixture()
		regX := makeReg("X", "1")
		regY := makeReg("Y", "2")
		err := store.SaveRegisters(
			height,
			blockA,
			lastID,
			flow.RegisterEntries{regX, regY},
		)
		require.NoError(t, err)

		valX, err := store.GetRegister(height, blockA, regX.Key)
		require.NoError(t, err)
		require.Equal(t, regX.Value, valX)

		valY, err := store.GetRegister(height, blockA, regY.Key)
		require.NoError(t, err)
		require.Equal(t, regY.Value, valY)
	})

	//  6. Given A(X: 1, Y: 2) <- B(Y: 3),
	//     GetRegister(B, X) should return 1, because X is not updated in B
	//     GetRegister(B, Y) should return 3, because Y is updated in B
	//     GetRegister(A, Y) should return 2, because the query queries the value at A, not B
	//     GetRegister(B, Z) should return PrunedError, because register is unknown
	//     GetRegister(C, X) should return BlockNotExecuted, because block is not executed (unexecuted)
	t.Run("GetLatestValueOK", func(t *testing.T) {
		t.Parallel()
		pruned := uint64(10)
		lastID := unittest.IdentifierFixture()
		store := NewInMemoryRegisterStore(pruned, lastID)

		// 10 <- A (X: 1, Y: 2) <- B (Y: 3)
		blockA := unittest.IdentifierFixture()
		regX := makeReg("X", "1")
		regY := makeReg("Y", "2")
		err := store.SaveRegisters(
			pruned+1,
			blockA,
			lastID,
			flow.RegisterEntries{regX, regY},
		)
		require.NoError(t, err)

		blockB := unittest.IdentifierFixture()
		regY3 := makeReg("Y", "3")
		err = store.SaveRegisters(
			pruned+2,
			blockB,
			blockA,
			flow.RegisterEntries{regY3},
		)
		require.NoError(t, err)

		val, err := store.GetRegister(pruned+2, blockB, regX.Key)
		require.NoError(t, err)
		require.Equal(t, regX.Value, val) // X is not updated in B

		val, err = store.GetRegister(pruned+2, blockB, regY.Key)
		require.NoError(t, err)
		require.Equal(t, regY3.Value, val) // Y is updated in B

		val, err = store.GetRegister(pruned+1, blockA, regY.Key)
		require.NoError(t, err)
		require.Equal(t, regY.Value, val) // Y's old value at A

		_, err = store.GetRegister(pruned+2, blockB, unknownKey)
		require.Error(t, err)
		pe, ok := IsPrunedError(err)
		require.True(t, ok)
		require.Equal(t, pe.PrunedHeight, pruned)
		require.Equal(t, pe.Height, pruned+2)

		_, err = store.GetRegister(pruned+3, unittest.IdentifierFixture(), regX.Key)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrNotExecuted) // unknown block
	})

	//  7. Given the following tree:
	//     Pruned <- A(X:1) <- B(Y:2)
	//     .......^- C(X:3) <- D(Y:4)
	//     GetRegister(D, X) should return 3
	t.Run("StoreMultiForkOK", func(t *testing.T) {
		t.Parallel()
		pruned := uint64(10)
		lastID := unittest.IdentifierFixture()
		store := NewInMemoryRegisterStore(pruned, lastID)

		// 10 <- A (X: 1) <- B (Y: 2)
		//		^- C (X: 3) <- D (Y: 4)
		blockA := unittest.IdentifierFixture()
		blockB := unittest.IdentifierFixture()
		blockC := unittest.IdentifierFixture()
		blockD := unittest.IdentifierFixture()

		require.NoError(t, store.SaveRegisters(
			pruned+1,
			blockA,
			lastID,
			flow.RegisterEntries{makeReg("X", "1")},
		))

		require.NoError(t, store.SaveRegisters(
			pruned+2,
			blockB,
			blockA,
			flow.RegisterEntries{makeReg("Y", "2")},
		))

		require.NoError(t, store.SaveRegisters(
			pruned+1,
			blockC,
			lastID,
			flow.RegisterEntries{makeReg("X", "3")},
		))

		require.NoError(t, store.SaveRegisters(
			pruned+2,
			blockD,
			blockC,
			flow.RegisterEntries{makeReg("Y", "4")},
		))

		reg := makeReg("X", "3")
		val, err := store.GetRegister(pruned+2, blockD, reg.Key)
		require.NoError(t, err)
		require.Equal(t, reg.Value, val)
	})

	//  8. Given the following tree:
	//     Pruned <- A(X:1) <- B(Y:2), B is not executed
	//     GetUpdatedRegisters(B) should return ErrNotExecuted
	t.Run("GetUpdatedRegisters", func(t *testing.T) {
		t.Parallel()
		pruned := uint64(10)
		lastID := unittest.IdentifierFixture()
		store := NewInMemoryRegisterStore(pruned, lastID)

		// 10 <- A (X: 1) <- B (Y: 2)
		blockA := unittest.IdentifierFixture()
		blockB := unittest.IdentifierFixture()

		require.NoError(t, store.SaveRegisters(
			pruned+1,
			blockA,
			lastID,
			flow.RegisterEntries{makeReg("X", "1")},
		))

		reg, err := store.GetUpdatedRegisters(pruned+1, blockA)
		require.NoError(t, err)
		require.Equal(t, flow.RegisterEntries{makeReg("X", "1")}, reg)

		_, err = store.GetUpdatedRegisters(pruned+2, blockB)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrNotExecuted)
	})

	//  9. Prune should fail if the block is unknown
	//     Prune should succeed if the block is known, and GetUpdatedRegisters should return err
	//     Prune should prune up to the pruned height.
	//     Given Pruned <- A(X:1) <- B(X:2) <- C(X:3) <- D(X:4)
	//     after Prune(B), GetRegister(C, X) should return 3, GetRegister(B, X) should return err
	t.Run("StorePrune", func(t *testing.T) {
		t.Parallel()
		pruned := uint64(10)
		lastID := unittest.IdentifierFixture()
		store := NewInMemoryRegisterStore(pruned, lastID)

		blockA := unittest.IdentifierFixture()
		blockB := unittest.IdentifierFixture()
		blockC := unittest.IdentifierFixture()
		blockD := unittest.IdentifierFixture()

		require.NoError(t, store.SaveRegisters(
			pruned+1,
			blockA,
			lastID,
			flow.RegisterEntries{makeReg("X", "1")},
		))

		require.NoError(t, store.SaveRegisters(
			pruned+2,
			blockB,
			blockA,
			flow.RegisterEntries{makeReg("X", "2")},
		))

		require.NoError(t, store.SaveRegisters(
			pruned+3,
			blockC,
			blockB,
			flow.RegisterEntries{makeReg("X", "3")},
		))

		require.NoError(t, store.SaveRegisters(
			pruned+4,
			blockD,
			blockC,
			flow.RegisterEntries{makeReg("X", "4")},
		))

		err := store.Prune(pruned+1, unknownBlock) // block is unknown
		require.Error(t, err)

		err = store.Prune(pruned+1, blockB) // block is known, but height is wrong
		require.Error(t, err)

		err = store.Prune(pruned+4, unknownBlock) // height is unknown
		require.Error(t, err)

		err = store.Prune(pruned+1, blockA) // prune next block
		require.NoError(t, err)

		require.Equal(t, pruned+1, store.PrunedHeight())

		reg := makeReg("X", "3")
		val, err := store.GetRegister(pruned+3, blockC, reg.Key)
		require.NoError(t, err)
		require.Equal(t, reg.Value, val)

		_, err = store.GetRegister(pruned+1, blockA, reg.Key) // A is pruned
		require.Error(t, err)
		pe, ok := IsPrunedError(err)
		require.True(t, ok)
		require.Equal(t, pe.PrunedHeight, pruned+1)
		require.Equal(t, pe.Height, pruned+1)

		err = store.Prune(pruned+3, blockC) // prune both B and C
		require.NoError(t, err)

		require.Equal(t, pruned+3, store.PrunedHeight())

		reg = makeReg("X", "4")
		val, err = store.GetRegister(pruned+4, blockD, reg.Key) // can still get X at block D
		require.NoError(t, err)
		require.Equal(t, reg.Value, val)
	})

	//  10. Prune should prune conflicting forks
	//     Given Pruned <- A(X:1) <- B(X:2)
	//     .................. ^----- E(X:5)
	//     ............ ^- C(X:3) <- D(X:4)
	//     Prune(A) should prune C and D, and GetUpdatedRegisters(C) should return out of range error,
	//     GetUpdatedRegisters(D) should return NotFound
	t.Run("PruneConflictingForks", func(t *testing.T) {
		t.Parallel()
		pruned := uint64(10)
		lastID := unittest.IdentifierFixture()
		store := NewInMemoryRegisterStore(pruned, lastID)

		blockA := unittest.IdentifierFixture()
		blockB := unittest.IdentifierFixture()
		blockC := unittest.IdentifierFixture()
		blockD := unittest.IdentifierFixture()
		blockE := unittest.IdentifierFixture()

		require.NoError(t, store.SaveRegisters(
			pruned+1,
			blockA,
			lastID,
			flow.RegisterEntries{makeReg("X", "1")},
		))

		require.NoError(t, store.SaveRegisters(
			pruned+2,
			blockB,
			blockA,
			flow.RegisterEntries{makeReg("X", "2")},
		))

		require.NoError(t, store.SaveRegisters(
			pruned+1,
			blockC,
			lastID,
			flow.RegisterEntries{makeReg("X", "3")},
		))

		require.NoError(t, store.SaveRegisters(
			pruned+2,
			blockD,
			blockC,
			flow.RegisterEntries{makeReg("X", "4")},
		))

		require.NoError(t, store.SaveRegisters(
			pruned+2,
			blockE,
			blockA,
			flow.RegisterEntries{makeReg("X", "5")},
		))

		err := store.Prune(pruned+1, blockA) // prune A should prune C and D
		require.NoError(t, err)

		_, err = store.GetUpdatedRegisters(pruned+2, blockD)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")

		_, err = store.GetUpdatedRegisters(pruned+2, blockE)
		require.NoError(t, err)
	})

	// 11. Concurrency: SaveRegisters can happen concurrently with GetUpdatedRegisters, and GetRegister
	t.Run("ConcurrentSaveAndGet", func(t *testing.T) {
		t.Parallel()
		pruned := uint64(10)
		lastID := unittest.IdentifierFixture()
		store := NewInMemoryRegisterStore(pruned, lastID)

		// prepare a chain of 101 blocks with the first as lastID
		count := 100
		blocks := make(map[uint64]flow.Identifier, count)
		blocks[pruned] = lastID
		for i := 1; i < count; i++ {
			block := unittest.IdentifierFixture()
			blocks[pruned+uint64(i)] = block
		}

		reg := makeReg("X", "0")

		var wg sync.WaitGroup
		for i := 1; i < count; i++ {
			height := pruned + uint64(i)
			require.NoError(t, store.SaveRegisters(
				height,
				blocks[height],
				blocks[height-1],
				flow.RegisterEntries{makeReg("X", fmt.Sprintf("%v", height))},
			))

			// concurrently query get registers for past registers
			wg.Add(1)
			go func(i int) {
				defer wg.Done()

				rdHeight := randBetween(pruned+1, pruned+uint64(i)+1)
				val, err := store.GetRegister(rdHeight, blocks[rdHeight], reg.Key)
				require.NoError(t, err)
				r := makeReg("X", fmt.Sprintf("%v", rdHeight))
				require.Equal(t, r.Value, val)
			}(i)

			// concurrently query updated registers
			wg.Add(1)
			go func(i int) {
				defer wg.Done()

				rdHeight := randBetween(pruned+1, pruned+uint64(i)+1)
				vals, err := store.GetUpdatedRegisters(rdHeight, blocks[rdHeight])
				require.NoError(t, err)
				r := makeReg("X", fmt.Sprintf("%v", rdHeight))
				require.Equal(t, flow.RegisterEntries{r}, vals)
			}(i)
		}

		wg.Wait()
	})

	// 12. Concurrency: Prune can happen concurrently with GetUpdatedRegisters, and GetRegister
	t.Run("ConcurrentSaveAndPrune", func(t *testing.T) {
		t.Parallel()
		pruned := uint64(10)
		lastID := unittest.IdentifierFixture()
		store := NewInMemoryRegisterStore(pruned, lastID)

		// prepare a chain of 101 blocks with the first as lastID
		count := 100
		blocks := make(map[uint64]flow.Identifier, count)
		blocks[pruned] = lastID
		for i := 1; i < count; i++ {
			block := unittest.IdentifierFixture()
			blocks[pruned+uint64(i)] = block
		}

		var wg sync.WaitGroup
		savedHeights := make(chan uint64, 100)

		wg.Add(1)
		go func() {
			defer wg.Done()

			lastPrunedHeight := pruned
			for savedHeight := range savedHeights {
				if savedHeight%10 != 0 {
					continue
				}
				rdHeight := randBetween(lastPrunedHeight+1, savedHeight+1)
				err := store.Prune(rdHeight, blocks[rdHeight])
				require.NoError(t, err)
				lastPrunedHeight = rdHeight
			}
		}()

		// save 100 blocks
		for i := 1; i < count; i++ {
			height := pruned + uint64(i)
			require.NoError(t, store.SaveRegisters(
				height,
				blocks[height],
				blocks[height-1],
				flow.RegisterEntries{makeReg("X", fmt.Sprintf("%v", i))},
			))
			savedHeights <- height
		}

		close(savedHeights)

		wg.Wait()
	})

	t.Run("PrunedError", func(t *testing.T) {
		e := NewPrunedError(1, 2, unittest.IdentifierFixture())
		pe, ok := IsPrunedError(e)
		require.True(t, ok)
		require.Equal(t, uint64(1), pe.Height)
		require.Equal(t, uint64(2), pe.PrunedHeight)
	})
}

// Benchmark GetRegister, and see how long in average it takes to return a register
// if it has to iterate through N blocks
// for instance, if N is 10, then given a chain of 10 blocks, if register A is only
// updated at the block 1, then getting register A at block 10 will have to iterate
// through block 10, block 9, block 8, ... block 1, and finally return the register,
// in other words, it will have to iterate through 10 blocks.
// Given the performance of GetRegister is largely affected by the number of blocks to
// iterate through, this test is to benchmark and compare the performance with different
// number of blocks to iterate.
func BenchmarkInMemoryRegisterStoreGetRegister(b *testing.B) {
	b.Run("1 block", func(b *testing.B) {
		benchmarkGetRegisterWhenIterateNBlocks(b, 1)
	})
	b.Run("7 blocks", func(b *testing.B) {
		// 7 blocks is the number of sealed blocks in average
		benchmarkGetRegisterWhenIterateNBlocks(b, 7)
	})
	b.Run("10 blocks", func(b *testing.B) {
		// 10 blocks if finalization has some hiccup
		benchmarkGetRegisterWhenIterateNBlocks(b, 10)
	})
}

func benchmarkGetRegisterWhenIterateNBlocks(b *testing.B, numBlock int) {
	_, blocks, store := createChainAndStore(b, numBlock)
	benchmarkGetRegister(b, store, blocks)
}

func benchmarkGetRegister(b *testing.B, store *InMemoryRegisterStore, blocks []*flow.Block) {
	// Benchmark GetRegister
	b.ResetTimer()
	log.Info().Msgf("Benchmark GetRegister with b.N %v", b.N)
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// since each register is updated at every height, if we would like to benchmark
		// getting a register which needs to traverse n blocks, we just need to get the register
		// at the last-nth block
		block := blocks[len(blocks)-1]
		blockID := block.ID()
		reg := makeReg(fmt.Sprintf("%v", 0), "")
		b.StartTimer()
		_, err := store.GetRegister(block.Header.Height, blockID, reg.Key)
		require.NoError(b, err)
	}

	b.StopTimer()
}

func BenchmarkInMemoryRegisterStoreGetRegisterSaveRegister(b *testing.B) {
	numBlock := 10
	root, blocks, store := createChainAndStore(b, numBlock)
	stop, start := saveRegistersUntilStop(b, root, store)
	defer start()
	benchmarkGetRegister(b, store, blocks)
	close(stop)
}

func createChainAndStore(b testing.TB, numBlock int) (*flow.Header, []*flow.Block, *InMemoryRegisterStore) {
	chain, _, _ := unittest.ChainFixture(numBlock + 1)
	genesis, blocks := chain[0], chain[1:]
	store := NewInMemoryRegisterStore(genesis.Header.Height, genesis.ID())
	// Create a register store with 1 register saved at each block
	for i, block := range blocks {
		registers := make(flow.RegisterEntries, 0, 1)
		registers = append(registers,
			makeReg(fmt.Sprintf("%v", i), // key
				fmt.Sprintf("%v", block.Header.Height), // value
			))
		require.NoError(b, store.SaveRegisters(
			block.Header.Height, block.ID(), block.Header.ParentID, registers))
	}
	return genesis.Header, blocks, store
}

func saveRegistersUntilStop(t testing.TB, root *flow.Header, store *InMemoryRegisterStore) (chan struct{}, func()) {
	stop := make(chan struct{})
	parent := root
	asyncLoop := func() {
		for {
			select {
			case _ = <-stop:
				break
			case <-time.After(time.Second):
				registers := make(flow.RegisterEntries, 0, 1)
				registers = append(registers, makeReg("hello", "world"))
				block := unittest.BlockWithParentFixture(parent)
				require.NoError(t, store.SaveRegisters(
					block.Header.Height, block.ID(), block.Header.ParentID, registers))
				parent = block.Header
			}
		}
	}
	return stop, asyncLoop
}

func randBetween(min, max uint64) uint64 {
	return uint64(rand.Intn(int(max)-int(min))) + min
}

func makeReg(key string, value string) flow.RegisterEntry {
	return unittest.MakeOwnerReg(key, value)
}

var unknownBlock = unittest.IdentifierFixture()
var unknownKey = flow.RegisterID{
	Owner: "unknown",
	Key:   "unknown",
}
