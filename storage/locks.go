package storage

import (
	"fmt"
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
	// LockInsertChunkDataPack protects the insertion of chunk data packs (not yet used anywhere)
	LockInsertChunkDataPack = "lock_insert_chunk_data_pack"
	// LockInsertTransactionResultErrMessage protects the insertion of transaction result error messages
	LockInsertTransactionResultErrMessage = "lock_insert_transaction_result_message"
	// LockInsertLightTransactionResult protects the insertion of light transaction results
	LockInsertLightTransactionResult = "lock_insert_light_transaction_result"
	// LockInsertExecutionForkEvidence protects the insertion of execution fork evidence
	LockInsertExecutionForkEvidence = "lock_insert_execution_fork_evidence"
	LockInsertSafetyData            = "lock_insert_safety_data"
	LockInsertLivenessData          = "lock_insert_liveness_data"
	// LockIndexScheduledTransaction protects the indexing of scheduled transactions.
	LockIndexScheduledTransaction = "lock_index_scheduled_transaction"
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
		LockInsertChunkDataPack,
		LockInsertTransactionResultErrMessage,
		LockInsertLightTransactionResult,
		LockInsertExecutionForkEvidence,
		LockInsertSafetyData,
		LockInsertLivenessData,
		LockIndexScheduledTransaction,
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
		Add(LockBootstrapping, LockInsertSafetyData).
		Add(LockInsertSafetyData, LockInsertLivenessData).
		Add(LockInsertOrFinalizeClusterBlock, LockInsertSafetyData).
		Add(LockInsertOwnReceipt, LockInsertChunkDataPack).

		// module/executiondatasync/optimistic_sync/persisters/block.go#Persist
		Add(LockInsertCollection, LockInsertLightTransactionResult).
		Add(LockInsertLightTransactionResult, LockInsertTransactionResultErrMessage).
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

// HeldOneLock checks that exactly one of the two specified locks is held in the provided lock context.
func HeldOneLock(lctx lockctx.Proof, lockA string, lockB string) (bool, string) {
	heldLockA := lctx.HoldsLock(lockA)
	heldLockB := lctx.HoldsLock(lockB)
	if heldLockA {
		if heldLockB {
			return false, fmt.Sprintf("epxect to hold only one lock, but actually held both locks: %s and %s", lockA, lockB)
		} else {
			return true, ""
		}
	} else {
		if heldLockB {
			return true, ""
		} else {
			return false, fmt.Sprintf("expect to hold one of the locks: %s or %s, but actually held none", lockA, lockB)
		}
	}
}

// WithLock is a helper function that creates a new lock context, acquires the specified lock,
// and executes the provided function within that context.
// This function passes through any errors returned by fn.
func WithLock(manager lockctx.Manager, lockID string, fn func(lctx lockctx.Context) error) error {
	return WithLocks(manager, []string{lockID}, fn)
}

// WithLocks is a helper function that creates a new lock context, acquires the specified locks,
// and executes the provided function within that context.
// This function passes through any errors returned by fn.
func WithLocks(manager lockctx.Manager, lockIDs []string, fn func(lctx lockctx.Context) error) error {
	lctx := manager.NewContext()
	defer lctx.Release()
	for _, lockID := range lockIDs {
		if err := lctx.AcquireLock(lockID); err != nil {
			return err
		}
	}
	return fn(lctx)
}
