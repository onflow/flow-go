package locks

import (
	"sync"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/storage"
)

// makeLockPolicy constructs the policy used by the storage layer to prevent deadlocks.
// We use a policy defined by a directed acyclic graph, where vertices are named locks.
// A directed edge between two vertices A, B means: I can acquire B next after acquiring A.
// When no edges are added, each lock context may acquire at most one lock.
//
// For example, the bootstrapping logic both inserts and finalizes blocks. So it needs to
// acquire both LockNewBlock and LockFinalizeBlock. To allow this, we add the directed
// edge LockNewBlock -> LockFinalizeBlock with `Add(LockNewBlock, LockFinalizeBlock)`.
// This means:
//   - a context can acquire either LockNewBlock or LockFinalizeBlock first (always true)
//   - a context holding LockNewBlock can acquire LockFinalizeBlock next (allowed by the edge)
//   - a context holding LockFinalizeBlock cannot acquire LockNewBlock next (disallowed, because the edge is directed)
//
// This function will panic if a policy is created which does not prevent deadlocks,
// i.e. if the constructed graph has cycles.
func makeLockPolicy() lockctx.Policy {
	return lockctx.NewDAGPolicyBuilder().
		Add(storage.LockInsertBlock, storage.LockFinalizeBlock).
		Build()
}

var makeLockManagerOnce sync.Once

// SingletonLockManager returns the lock manager used by the storage layer.
// This function must be used for production builds and must be called exactly once process-wide.
//
// The Lock Manager is a core component enforcing atomicity of various storage operations across different
// components. Therefore, the lock manager is a singleton instance, because its correctness depends on the
// same set of locks being used everywhere.
// By convention, the lock mananger singleton is injected into the node's components during their
// initialization, following the same dependency-injection pattern as other components that are conceptually
// singletons (e.g. the storage layer abstractions). Thereby, we explicitly codify in the constructor that a
// component uses the lock mananger. We think it is helpful to emphasize that the component at times
// will acquire _exclusive access_ to all key-value pairs in the database whose keys start with some specific
// prefixes (see `storage/badger/operation/prefix.go` for an exhaustive list of prefixes).
// In comparison, the alternative pattern (which we do not use) of retrieving a singleton instance via a
// global variable would hide which components required exclusive storage access, and in addition, it would
// break with our broadly established dependency-injection pattern. To enforce best practices, this function
// will panic if it is called more than once.
//
// CAUTION:
//   - The lock manager only guarantees atomicity of reads and writes for the thread holding the lock.
//     Other threads can continue to read possibly stale values, while the lock is held by a different thread.
//     Though, this is no different than working with any transaction-based database, where reads might
//     operate on a prior snapshot and return stale values, while a concurrent write is ongoing.
//   - Furthermore, the writer must bundle all their writes into a _single_ Write Batch for atomicity. Even
//     when holding the lock, reading threads can still observe the writes of one batch while not observing
//     the writes of a second batch, despite the thread writing both batches while holding the lock. It was
//     a deliberate choice for the sake of performance to allow reads without any locking - so instead of
//     waiting for the newest value in case a write is currently ongoing, the reader will just retrieve the
//     previous value. This aligns with our architecture of the node operating as an eventually-consistent
//     system, which favors loose coupling and high throughput for different components within a node.
func SingletonLockManager() lockctx.Manager {
	var manager lockctx.Manager
	makeLockManagerOnce.Do(func() {
		manager = lockctx.NewManager(storage.Locks(), makeLockPolicy())
	})
	if manager == nil {
		panic("critical sanity check failed: SingletonLockManager invoked more than once")
	}
	return manager
}

// NewTestingLockManager returns a new lock manager instance for testing purposes.
// CAUTION: for production code, there should only ever be one lock manager per database!
// This is only used for integration testing, where we want to emulate within the same process
// multiple different nodes, each with their dedicated database and lock manager.
func NewTestingLockManager() lockctx.Manager {
	return lockctx.NewManager(storage.Locks(), makeLockPolicy())
}
