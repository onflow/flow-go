package storage

import (
	"sync"

	"github.com/jordanschalm/lockctx"
)

// This file enumerates all named locks used by the storage layer.

const (
	// LockInsertBlock protects the entire block insertion process (Extend or ExtendCertified)
	LockInsertBlock = "lock_insert_block"
	// LockFinalizeBlock protects the entire block finalization process (Finalize)
	LockFinalizeBlock = "lock_finalize_block"
	// LockIndexResultApproval protects indexing result approvals by approval and chunk.
	LockIndexResultApproval  = "lock_index_result_approval"
	LockInsertClusterBlock   = "lock_insert_cluster_block"
	LockFinalizeClusterBlock = "lock_finalize_cluster_block"
)

// Locks returns a list of all named locks used by the storage layer.
func Locks() []string {
	return []string{
		LockInsertBlock,
		LockFinalizeBlock,
		LockIndexResultApproval,
		LockInsertClusterBlock,
		LockFinalizeClusterBlock,
	}
}

// makeLockPolicy constructs the policy used by the storage layer to prevent deadlocks.
// We use a policy defined by a directed acyclic graph, where nodes are named locks.
// A directed edge between two nodes A, B means: I can acquire B next after acquiring A.
// When no edges are added, each lock context may acquire at most one lock.
//
// For example, the bootstrapping logic both inserts and finalizes block. So it needs to
// acquire both LockInsertBlock and LockFinalizeBlock. To allow this, we add the directed
// edge LockInsertBlock -> LockFinalizeBlock with `Add(LockInsertBlock, LockFinalizeBlock)`.
// This means:
//   - a context can acquire either LockInsertBlock or LockFinalizeBlock first (always true)
//   - a context holding LockInsertBlock can acquire LockFinalizeBlock next (allowed by the edge)
//   - a context holding LockFinalizeBlock cannot acquire LockInsertBlock next (disallowed, because the edge is directed)
//
// This function will panic if a policy is created which does not prevent deadlocks.
func makeLockPolicy() lockctx.Policy {
	return lockctx.NewDAGPolicyBuilder().
		Add(LockInsertBlock, LockFinalizeBlock).
		Add(LockInsertClusterBlock, LockFinalizeClusterBlock).
		Build()
}

var makeLockManagerOnce sync.Once

// MakeSingletonLockManager returns the lock manager used by the storage layer.
// This function must be used for production builds and must be called exactly once process-wide.
// If this function is called more than once, it will panic. This is strictly enforced because
// correctness of the lock manager depends on the same set of locks being used everywhere.
func MakeSingletonLockManager() lockctx.Manager {
	var manager lockctx.Manager
	makeLockManagerOnce.Do(func() {
		manager = lockctx.NewManager(Locks(), makeLockPolicy())
	})
	if manager == nil {
		panic("critical sanity check failed: MakeSingletonLockManager invoked more than once")
	}
	return manager
}

// NewTestingLockManager returns the lock manager used by the storage layer.
// This function must be used for testing only and must not be used in production builds.
// Unlike MakeSingletonLockManager, this function may be called multiple times.
func NewTestingLockManager() lockctx.Manager {
	return lockctx.NewManager(Locks(), makeLockPolicy())
}
