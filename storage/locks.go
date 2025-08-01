package storage

// This file enumerates all named locks used by the storage layer.

const (
	// LockNewBlock protects the entire block insertion process (Extend or ExtendCertified)
	LockNewBlock = "lock_new_block"
	// LockFinalizeBlock protects the entire block finalization process (Finalize)
	LockFinalizeBlock = "lock_finalize_block"
	// LockMyResultApproval protects indexing result approvals by approval and chunk.
	LockMyResultApproval = "lock_index_result_approval"
)

// Locks returns a list of all named locks used by the storage layer.
func Locks() []string {
	return []string{
		LockNewBlock,
		LockFinalizeBlock,
		LockMyResultApproval,
	}
}
