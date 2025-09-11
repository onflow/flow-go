package unittest

import (
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"
)

// WithLock creates a lock context from the given manager, acquires the given lock, then executes the function `fn`.
// The test will fail if we are unable to acquire the lock or if `fn` returns any non-nil error.
func WithLock(t testing.TB, manager lockctx.Manager, lockID string, fn func(lctx lockctx.Context) error) {
	t.Helper()
	lctx := manager.NewContext()
	require.NoError(t, lctx.AcquireLock(lockID))
	defer lctx.Release()
	require.NoError(t, fn(lctx))
}

// WithLocks creates a lock context from the given manager, acquires the given locks, then executes the function `fn`.
// The test will fail if we are unable to acquire any of the locks or if `fn` returns any non-nil error.
func WithLocks(t testing.TB, manager lockctx.Manager, lockIDs []string, fn func(lctx lockctx.Context) error) {
	t.Helper()
	lctx := manager.NewContext()
	for _, lockID := range lockIDs {
		require.NoError(t, lctx.AcquireLock(lockID))
	}
	defer lctx.Release()
	require.NoError(t, fn(lctx))
}
