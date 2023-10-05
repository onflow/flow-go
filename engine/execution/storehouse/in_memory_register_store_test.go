package storehouse

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

// 1. SaveRegisters should fail if height is below or equal to pruned height
//
// 2. SaveRegisters should fail if the block is already saved
//
//  3. SaveRegisters should succeed if height is above pruned height and block is not saved,
//     the updates can be retrieved by GetUpdatedRegisters
//
//  4. SaveRegisters should succeed if a different block at the same height was saved before,
//     updates for different blocks can be retrieved by their blockID
//
// 5. SaveRegisters should fail if its parent block doesn't exist and it is not the pruned block
//
// 6. Given A(X: 1, Y: 2), GetRegister(A, X) should return 1, GetRegister(A, X) should return 2
//
//  7. Given A(X: 1, Y: 2) <- B(Y: 3),
//     GetRegister(B, X) should return 1, because X is not updated in B
//     GetRegister(B, Y) should return 3, because Y is updated in B
//     GetRegister(A, Y) should return 2, because the query queries the value at A, not B
//     GetRegister(B, Z) should return NotFound, because register is unknown
//     GetRegister(C, X) should return BlockNotExecuted, because block is not executed (unexecuted)
//
// 8. GetRegister should return out of range error if the queried height is below pruned height
//
//  9. Given the following tree:
//     Pruned <- A(X:1) <- B(Y:2)
//     .......^- C(X:3) <- D(Y:4)
//     GetRegister(D, X) should return 3
//
// 10. Prune should fail if the block is unknown
//
// 11. Prune should succeed if the block is known, and GetUpdatedRegisters should return out of range error
//
//  12. Prune should prune up to the pruned height.
//     Given Pruned <- A(X:1) <- B(X:2) <- C(X:3),
//     after Prune(B), GetRegister(C, X) should return 3, GetRegister(B, X) should return out of range error
//
//  13. Prune should prune conflicting forks
//     Given Pruned <- A(X:1) <- B(X:2)
//     ............ ^- C(X:3) <- D(X:4)
//     Prune(A) should prune C and D, and GetUpdatedRegisters(C) should return out of range error,
//     GetUpdatedRegisters(D) should return NotFound
//
// 14. Concurrency: SaveRegisters can happen concurrently with GetUpdatedRegisters, and GetRegister
// 15. Concurrency: Prune can happen concurrently with GetUpdatedRegisters, and GetRegister
func TestInMemoryRegisterStoreFailBelowOrEqualPrunedHeight(t *testing.T) {
	// 1.
	height := uint64(10)
	lastID := unittest.IdentifierFixture()
	store := NewInMemoryRegisterStore(height, lastID)
	err := store.SaveRegisters(
		height-1, // below pruned height, will fail
		unittest.IdentifierFixture(),
		unittest.IdentifierFixture(),
		[]flow.RegisterEntry{},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "<= pruned height")

	err = store.SaveRegisters(
		height, // equal to pruned height, will fail
		lastID,
		unittest.IdentifierFixture(),
		[]flow.RegisterEntry{},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "<= pruned height")
}
