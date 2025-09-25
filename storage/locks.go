package storage

import (
	"sync"

	"github.com/jordanschalm/lockctx"
)

// This file enumerates all named locks used by the storage layer.

const (
	// LockInsertBlock protects the entire block insertion process (`ParticipantState.Extend` or `FollowerState.ExtendCertified`)
	LockInsertBlock = "lock_insert_block"
	// LockFinalizeBlock protects the entire block finalization process (`FollowerState.Finalize`)
	LockFinalizeBlock = "lock_finalize_block"
	// LockIndexResultApproval protects indexing result approvals by approval and chunk.
	LockIndexResultApproval = "lock_index_result_approval"
	// LockInsertOrFinalizeClusterBlock protects the entire cluster block insertion or finalization process.
	// The reason they are combined is because insertion process reads some data updated by finalization process,
	// in order to prevent dirty reads, we need to acquire the lock for both operations.
	LockInsertOrFinalizeClusterBlock = "lock_insert_or_finalize_cluster_block"
	// LockInsertOwnReceipt is intended for Execution Nodes to ensure that they never publish different receipts for the same block.
	// Specifically, with this lock we prevent accidental overwrites of the index `executed block ID` ➜ `Receipt ID`.
	LockInsertOwnReceipt = "lock_insert_own_receipt"
	// LockInsertCollection protects the insertion of collections.
	LockInsertCollection = "lock_insert_collection"
	// LockBootstrapping protects data that is *exclusively* written during bootstrapping.
	LockBootstrapping = "lock_bootstrapping"
)

// Locks returns a list of all named locks used by the storage layer.
func Locks() []string {
	return []string{
		LockInsertBlock,
		LockFinalizeBlock,
		LockIndexResultApproval,
		LockInsertOrFinalizeClusterBlock,
		LockInsertOwnReceipt,
		LockInsertCollection,
		LockBootstrapping,
	}
}

type LockManager = lockctx.Manager

// makeLockPolicy constructs the policy used by the storage layer to prevent deadlocks.
// We use a policy defined by a directed acyclic graph, where vertices represent named locks.
// A directed edge between two vertices A, B means: I can acquire B next after acquiring A.
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
		Add(LockFinalizeBlock, LockBootstrapping).
		Build()
}

var makeLockManagerOnce sync.Once

// MakeSingletonLockManager returns the lock manager used by the storage layer.
// This function must be used for production builds and must be called exactly once process-wide.
//
// The Lock Manager is a core component enforcing atomicity of various storage operations across different
// components. Therefore, the lock manager is a singleton instance, as the storage layer's atomicity and
// consistency depends on the same set of locks being used everywhere.
// By convention, the lock manager singleton is injected into the node's components during their
// initialization, following the same dependency-injection pattern as other components that are conceptually
// singletons (e.g. the storage layer abstractions). Thereby, we explicitly codify in the constructor that a
// component uses the lock manager. We think it is helpful to emphasize that the component at times
// will acquire _exclusive access_ to all key-value pairs in the database whose keys start with some specific
// prefixes (see `storage/badger/operation/prefix.go` for an exhaustive list of prefixes).
// In comparison, the alternative pattern (which we do not use) of retrieving a singleton instance via a
// global variable would hide which components required exclusive storage access, and in addition, it would
// break with our broadly established dependency-injection pattern. To enforce best practices, this function
// will panic if it is called more than once.
//
// CAUTION:
//   - The lock manager only guarantees atomicity of reads and writes for the thread holding the lock.
//     Other threads can continue to read (possibly stale) values, while the lock is held by a different thread.
//   - Furthermore, the writer must bundle all their writes into a _single_ Write Batch for atomicity. Even
//     when holding the lock, reading threads can still observe the writes of one batch while not observing
//     the writes of a second batch, despite the thread writing both batches while holding the lock. It was
//     a deliberate choice for the sake of performance to allow reads without any locking - so instead of
//     waiting for the newest value in case a write is currently ongoing, the reader will just retrieve the
//     previous value. This aligns with our architecture of the node operating as an eventually-consistent
//     system, which favors loose coupling and high throughput for different components within a node.
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
// This function must be used for testing only but NOT for PRODUCTION builds.
// Unlike MakeSingletonLockManager, this function may be called multiple times.
func NewTestingLockManager() lockctx.Manager {
	return lockctx.NewManager(Locks(), makeLockPolicy())
}
