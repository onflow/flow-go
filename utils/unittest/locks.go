package unittest

import (
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"
)

func WithLock(t *testing.T, manager lockctx.Manager, lockID string, fn func(lctx lockctx.Context) error) {
	t.Helper()
	lctx := manager.NewContext()
	require.NoError(t, lctx.AcquireLock(lockID))
	defer lctx.Release()
	require.NoError(t, fn(lctx))
}

func WithLockBench(b *testing.B, manager lockctx.Manager, lockID string, fn func(lctx lockctx.Context) error) {
	lctx := manager.NewContext()
	require.NoError(b, lctx.AcquireLock(lockID))
	defer lctx.Release()
	require.NoError(b, fn(lctx))
}
