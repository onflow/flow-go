package storehouse

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

// 1. SaveRegisters should fail if height is below or equal to pruned height
//
// 2. SaveRegisters should fail if its parent block doesn't exist and it is not the pruned block

//  3. SaveRegisters should succeed if height is above pruned height and block is not saved,
//     the updates can be retrieved by GetUpdatedRegisters
//     GetRegister should return ErrPruned if the queried key is not updated since pruned height
//     GetRegister should return ErrPruned if the queried height is below pruned height
//     GetRegister should return ErrNotExecuted if the block is unknown
//
// 4. SaveRegisters should fail if the block is already saved
//
//  5. SaveRegisters should succeed if a different block at the same height was saved before,
//     updates for different blocks can be retrieved by their blockID
//
// 6. Given A(X: 1, Y: 2), GetRegister(A, X) should return 1, GetRegister(A, X) should return 2
//
//  7. Given A(X: 1, Y: 2) <- B(Y: 3),
//     GetRegister(B, X) should return 1, because X is not updated in B
//     GetRegister(B, Y) should return 3, because Y is updated in B
//     GetRegister(A, Y) should return 2, because the query queries the value at A, not B
//     GetRegister(B, Z) should return ErrPruned, because register is unknown
//     GetRegister(C, X) should return BlockNotExecuted, because block is not executed (unexecuted)
//
//  8. Given the following tree:
//     Pruned <- A(X:1) <- B(Y:2)
//     .......^- C(X:3) <- D(Y:4)
//     GetRegister(D, X) should return 3
//
//  9. Prune should fail if the block is unknown
//     Prune should succeed if the block is known, and GetUpdatedRegisters should return out of range error
//     Prune should prune up to the pruned height.
//     Given Pruned <- A(X:1) <- B(X:2) <- C(X:3),
//     after Prune(B), GetRegister(C, X) should return 3, GetRegister(B, X) should return out of range error
//
//  10. Prune should prune conflicting forks
//     Given Pruned <- A(X:1) <- B(X:2)
//     ............ ^- C(X:3) <- D(X:4)
//     Prune(A) should prune C and D, and GetUpdatedRegisters(C) should return out of range error,
//     GetUpdatedRegisters(D) should return NotFound
//
// 11. Concurrency: SaveRegisters can happen concurrently with GetUpdatedRegisters, and GetRegister
// 12. Concurrency: Prune can happen concurrently with GetUpdatedRegisters, and GetRegister
func TestInMemoryRegisterStoreFailBelowOrEqualPrunedHeight(t *testing.T) {
	// 1.
	pruned := uint64(10)
	lastID := unittest.IdentifierFixture()
	store := NewInMemoryRegisterStore(pruned, lastID)
	err := store.SaveRegisters(
		pruned-1, // below pruned pruned, will fail
		unittest.IdentifierFixture(),
		unittest.IdentifierFixture(),
		[]flow.RegisterEntry{},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "<= pruned height")

	err = store.SaveRegisters(
		pruned, // equal to pruned height, will fail
		lastID,
		unittest.IdentifierFixture(),
		[]flow.RegisterEntry{},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "<= pruned height")
}

func TestInMemoryRegisterStoreFailParentNotExist(t *testing.T) {
	// 2.
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
		[]flow.RegisterEntry{reg},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "but its parent")
}

func TestInMemoryRegisterStoreOK(t *testing.T) {
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
		[]flow.RegisterEntry{reg},
	)
	require.NoError(t, err)

	val, err := store.GetRegister(height, blockID, reg.Key)
	require.NoError(t, err)
	require.Equal(t, reg.Value, val)

	// unknown key
	_, err = store.GetRegister(height, blockID, unknownKey)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPruned)

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
	require.ErrorIs(t, err, ErrPruned)
}

func TestInMemoryRegisterStoreFailAlreadyExist(t *testing.T) {
	// 4.
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
		[]flow.RegisterEntry{reg},
	)
	require.NoError(t, err)

	// saving again should fail
	err = store.SaveRegisters(
		height,
		blockID,
		lastID,
		[]flow.RegisterEntry{reg},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}

func TestInMemoryRegisterStoreOKDifferentBlockSameParent(t *testing.T) {
	// 5.
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
		[]flow.RegisterEntry{regA},
	)
	require.NoError(t, err)

	blockB := unittest.IdentifierFixture()
	regB := unittest.RegisterEntryFixture()
	err = store.SaveRegisters(
		height,
		blockB, // different block
		lastID, // same parent
		[]flow.RegisterEntry{regB},
	)
	require.NoError(t, err)

	valA, err := store.GetRegister(height, blockA, regA.Key)
	require.NoError(t, err)
	require.Equal(t, regA.Value, valA)

	valB, err := store.GetRegister(height, blockB, regB.Key)
	require.NoError(t, err)
	require.Equal(t, regB.Value, valB)
}

func TestInMemoryRegisterGetRegistersOK(t *testing.T) {
	// 6.
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
		[]flow.RegisterEntry{regX, regY},
	)
	require.NoError(t, err)

	valX, err := store.GetRegister(height, blockA, regX.Key)
	require.NoError(t, err)
	require.Equal(t, regX.Value, valX)

	valY, err := store.GetRegister(height, blockA, regY.Key)
	require.NoError(t, err)
	require.Equal(t, regY.Value, valY)
}

func TestInMemoryRegisterStoreGetLatestValueOK(t *testing.T) {
	// 7
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
		[]flow.RegisterEntry{regX, regY},
	)
	require.NoError(t, err)

	blockB := unittest.IdentifierFixture()
	regY3 := makeReg("Y", "3")
	err = store.SaveRegisters(
		pruned+2,
		blockB,
		blockA,
		[]flow.RegisterEntry{regY3},
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

	val, err = store.GetRegister(pruned+2, blockB, unknownKey)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPruned) // unknown key

	val, err = store.GetRegister(pruned+3, unittest.IdentifierFixture(), regX.Key)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotExecuted) // unknown block
}

func TestInMemoryRegisterStoreMultiForkOK(t *testing.T) {
	// 9
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
		[]flow.RegisterEntry{makeReg("X", "1")},
	))

	require.NoError(t, store.SaveRegisters(
		pruned+2,
		blockB,
		blockA,
		[]flow.RegisterEntry{makeReg("Y", "2")},
	))

	require.NoError(t, store.SaveRegisters(
		pruned+1,
		blockC,
		lastID,
		[]flow.RegisterEntry{makeReg("X", "3")},
	))

	require.NoError(t, store.SaveRegisters(
		pruned+2,
		blockD,
		blockC,
		[]flow.RegisterEntry{makeReg("Y", "4")},
	))

	reg := makeReg("X", "3")
	val, err := store.GetRegister(pruned+2, blockD, reg.Key)
	require.NoError(t, err)
	require.Equal(t, reg.Value, val)
}

func TestInMemoryRegisterStorePrune(t *testing.T) {
	// 9
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
		[]flow.RegisterEntry{makeReg("X", "1")},
	))

	require.NoError(t, store.SaveRegisters(
		pruned+2,
		blockB,
		blockA,
		[]flow.RegisterEntry{makeReg("X", "2")},
	))

	require.NoError(t, store.SaveRegisters(
		pruned+3,
		blockC,
		blockB,
		[]flow.RegisterEntry{makeReg("X", "3")},
	))

	require.NoError(t, store.SaveRegisters(
		pruned+4,
		blockD,
		blockC,
		[]flow.RegisterEntry{makeReg("X", "4")},
	))

	err := store.Prune(pruned+1, unknownBlock) // block is unknown
	require.Error(t, err)

	err = store.Prune(pruned+1, blockB) // block is known, but height is wrong
	require.Error(t, err)

	err = store.Prune(pruned+2, blockA) // height is known, but block is wrong
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

	val, err = store.GetRegister(pruned+1, blockA, reg.Key) // A is pruned
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPruned)

	err = store.Prune(pruned+3, blockC) // prune both B and C
	require.NoError(t, err)

	require.Equal(t, pruned+3, store.PrunedHeight())

	reg = makeReg("X", "4")
	val, err = store.GetRegister(pruned+4, blockD, reg.Key) // can still get X at block D
	require.NoError(t, err)
	require.Equal(t, reg.Value, val)
}

func TestPruneConflictingForks(t *testing.T) {
	// 9
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
		[]flow.RegisterEntry{makeReg("X", "1")},
	))

	require.NoError(t, store.SaveRegisters(
		pruned+2,
		blockB,
		blockA,
		[]flow.RegisterEntry{makeReg("X", "2")},
	))

	require.NoError(t, store.SaveRegisters(
		pruned+1,
		blockC,
		lastID,
		[]flow.RegisterEntry{makeReg("X", "3")},
	))

	require.NoError(t, store.SaveRegisters(
		pruned+2,
		blockD,
		blockC,
		[]flow.RegisterEntry{makeReg("X", "4")},
	))

	err := store.Prune(pruned+1, blockA) // prune A should prune C and D
	require.NoError(t, err)

	_, err = store.GetUpdatedRegisters(pruned+2, blockD)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func makeReg(key string, value string) flow.RegisterEntry {
	return flow.RegisterEntry{
		Key: flow.RegisterID{
			Owner: "owner",
			Key:   key,
		},
		Value: []byte(value),
	}
}

var unknownBlock = unittest.IdentifierFixture()
var unknownKey = flow.RegisterID{
	Owner: "unknown",
	Key:   "unknown",
}
