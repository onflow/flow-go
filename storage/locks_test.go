package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeldOneLock(t *testing.T) {
	lockManager := NewTestingLockManager()

	t.Run("holds only lockA", func(t *testing.T) {
		lctx := lockManager.NewContext()
		defer lctx.Release()
		err := lctx.AcquireLock(LockInsertBlock)
		require.NoError(t, err)

		held, msg := HeldOneLock(lctx, LockInsertBlock, LockFinalizeBlock)
		assert.True(t, held)
		assert.Empty(t, msg)
	})

	t.Run("holds only lockB", func(t *testing.T) {
		lctx := lockManager.NewContext()
		defer lctx.Release()
		err := lctx.AcquireLock(LockFinalizeBlock)
		require.NoError(t, err)

		held, msg := HeldOneLock(lctx, LockInsertBlock, LockFinalizeBlock)
		assert.True(t, held)
		assert.Empty(t, msg)
	})

	t.Run("holds both locks", func(t *testing.T) {
		lctx := lockManager.NewContext()
		defer lctx.Release()
		err := lctx.AcquireLock(LockInsertBlock)
		require.NoError(t, err)
		err = lctx.AcquireLock(LockFinalizeBlock)
		require.NoError(t, err)

		held, msg := HeldOneLock(lctx, LockInsertBlock, LockFinalizeBlock)
		assert.False(t, held)
		assert.Contains(t, msg, "epxect to hold only one lock, but actually held both locks")
		assert.Contains(t, msg, LockInsertBlock)
		assert.Contains(t, msg, LockFinalizeBlock)
	})

	t.Run("holds neither lock", func(t *testing.T) {
		// Create a context that doesn't hold any locks
		lctx := lockManager.NewContext()
		defer lctx.Release()

		held, msg := HeldOneLock(lctx, LockInsertBlock, LockFinalizeBlock)
		assert.False(t, held)
		assert.Contains(t, msg, "expect to hold one of the locks")
		assert.Contains(t, msg, LockInsertBlock)
		assert.Contains(t, msg, LockFinalizeBlock)
	})

	t.Run("holds different lock", func(t *testing.T) {
		lctx := lockManager.NewContext()
		defer lctx.Release()
		err := lctx.AcquireLock(LockInsertInstanceParams)
		require.NoError(t, err)

		held, msg := HeldOneLock(lctx, LockInsertBlock, LockFinalizeBlock)
		assert.False(t, held)
		assert.Contains(t, msg, "expect to hold one of the locks")
		assert.Contains(t, msg, LockInsertBlock)
		assert.Contains(t, msg, LockFinalizeBlock)
	})

	t.Run("with different lock combinations", func(t *testing.T) {
		lctx := lockManager.NewContext()
		defer lctx.Release()
		err := lctx.AcquireLock(LockInsertOwnReceipt)
		require.NoError(t, err)

		held, msg := HeldOneLock(lctx, LockInsertOwnReceipt, LockInsertCollection)
		assert.True(t, held)
		assert.Empty(t, msg)
	})

	t.Run("with cluster block locks", func(t *testing.T) {
		lctx := lockManager.NewContext()
		defer lctx.Release()
		err := lctx.AcquireLock(LockInsertOrFinalizeClusterBlock)
		require.NoError(t, err)

		held, msg := HeldOneLock(lctx, LockInsertOrFinalizeClusterBlock, LockIndexResultApproval)
		assert.True(t, held)
		assert.Empty(t, msg)
	})

	t.Run("error message format for both locks", func(t *testing.T) {
		lctx := lockManager.NewContext()
		defer lctx.Release()
		err := lctx.AcquireLock(LockInsertBlock)
		require.NoError(t, err)
		err = lctx.AcquireLock(LockFinalizeBlock)
		require.NoError(t, err)

		// Check that both locks are actually held
		assert.True(t, lctx.HoldsLock(LockInsertBlock))
		assert.True(t, lctx.HoldsLock(LockFinalizeBlock))

		held, msg := HeldOneLock(lctx, LockInsertBlock, LockFinalizeBlock)
		assert.False(t, held)
		require.NotEmpty(t, msg)
		assert.Contains(t, msg, "epxect to hold only one lock, but actually held both locks")
		assert.Contains(t, msg, LockInsertBlock)
		assert.Contains(t, msg, LockFinalizeBlock)
	})

	t.Run("error message format for no locks", func(t *testing.T) {
		lctx := lockManager.NewContext()
		defer lctx.Release()

		held, msg := HeldOneLock(lctx, "lockA", "lockB")
		assert.False(t, held)
		require.NotEmpty(t, msg)
		assert.Contains(t, msg, "expect to hold one of the locks")
		assert.Contains(t, msg, "lockA")
		assert.Contains(t, msg, "lockB")
	})
}
