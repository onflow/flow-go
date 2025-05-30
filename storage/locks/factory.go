package locks

import (
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

func NewLockManager() lockctx.Manager {
	// Create a new lockctx manager with the storage locks and the lock policy.
	return lockctx.NewManager(storage.Locks(), makeLockPolicy())
}

// LockManagerFactory is for creating a singleton lockctx manager
// The factory can only create a single manager, by ensuring the
// NewLockManagerFactory function is only called once, only one
// manager is created.
// This is useful for the integration tests to create multiple followers, each
// has their own singleton lockctx manager.
type LockManagerFactory struct {
	// the default value is false, so that if the type is used by constructor
	// from other package, the default canCreate is false preventing it from
	// creating lock manager
	canCreate bool
}

func NewLockManagerFactory() *LockManagerFactory {
	return &LockManagerFactory{
		canCreate: true,
	}
}

// Create creates a lockctx manager, this method can only be called once to ensure
// only a single manager can be created.
func (f *LockManagerFactory) Create() lockctx.Manager {
	if !f.canCreate {
		panic("critical sanity check fail: factory can only be used once to create lock manager")
	}

	f.canCreate = false

	return lockctx.NewManager(storage.Locks(), makeLockPolicy())
}

// NewTestingLockManager returns the lock manager used by the storage layer.
// This function must be used for testing only and must not be used in production builds.
func NewTestingLockManager() lockctx.Manager {
	return lockctx.NewManager(storage.Locks(), makeLockPolicy())
}
