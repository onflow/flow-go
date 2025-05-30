package locks

import (
	"sync"

	"github.com/jordanschalm/lockctx"
	"github.com/onflow/flow-go/storage"
)

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
		Add(storage.LockInsertBlock, storage.LockFinalizeBlock).
		Build()
}

var makeLockManagerOnce sync.Once

func MakeSingletonLockManager() lockctx.Manager {
	var manager lockctx.Manager
	makeLockManagerOnce.Do(func() {
		manager = lockctx.NewManager(storage.Locks(), makeLockPolicy())
	})
	if manager == nil {
		panic("critical sanity check failed: MakeSingletonLockManager invoked more than once")
	}
	return manager
}

// NewTestingLockManager returns a new lock manager instance for testing purposes.
// This function should only be used in tests.
func NewTestingLockManager() lockctx.Manager {
	return lockctx.NewManager(storage.Locks(), makeLockPolicy())
}
