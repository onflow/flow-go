package storage

// This file enumerates all named locks used by the storage layer.

const (
	// LockInsertBlock protects the entire block insertion process (Extend or ExtendCertified)
	LockInsertBlock = "lock_insert_block"
	// LockFinalizeBlock protects the entire block finalization process (Finalize)
	LockFinalizeBlock = "lock_finalize_block"
	// LockIndexResultApproval protects indexing result approvals by approval and chunk.
	LockIndexResultApproval = "lock_index_result_approval"
)

// Locks returns a list of all named locks used by the storage layer.
func Locks() []string {
	return []string{
		LockInsertBlock,
		LockFinalizeBlock,
		LockIndexResultApproval,
	}
}
