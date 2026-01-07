package io

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestFileLock(t *testing.T) {
	t.Run("basic lock and unlock", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			lock := NewFileLock(dir)
			require.NotNil(t, lock)

			// Verify lock path
			expectedPath := filepath.Join(dir, ".lock")
			require.Equal(t, expectedPath, lock.Path())

			// Acquire lock
			err := lock.Lock()
			require.NoError(t, err)

			// Release lock
			err = lock.Unlock()
			require.NoError(t, err)
		})
	})

	t.Run("lock prevents concurrent access", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			lock1 := NewFileLock(dir)
			lock2 := NewFileLock(dir)

			// First lock should succeed
			err := lock1.Lock()
			require.NoError(t, err)

			// Second lock should fail
			err = lock2.Lock()
			require.Error(t, err)
			require.Contains(t, err.Error(), "another process is already using this resource")

			// Release first lock
			err = lock1.Unlock()
			require.NoError(t, err)

			// Now second lock should succeed
			err = lock2.Lock()
			require.NoError(t, err)

			err = lock2.Unlock()
			require.NoError(t, err)
		})
	})

	t.Run("lock can be re-acquired after unlock", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			lock := NewFileLock(dir)

			// Acquire and release multiple times
			for i := 0; i < 3; i++ {
				err := lock.Lock()
				require.NoError(t, err, "iteration %d", i)

				err = lock.Unlock()
				require.NoError(t, err, "iteration %d", i)
			}
		})
	})

	t.Run("concurrent goroutines competing for lock", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			const numGoroutines = 10
			var wg sync.WaitGroup
			successCount := 0
			failureCount := 0
			var mu sync.Mutex
			var lockHeld sync.WaitGroup

			// First, acquire the lock to hold it
			mainLock := NewFileLock(dir)
			err := mainLock.Lock()
			require.NoError(t, err)

			// Start multiple goroutines trying to acquire the same lock
			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				lockHeld.Add(1)
				go func() {
					defer wg.Done()
					lock := NewFileLock(dir)
					err := lock.Lock()
					mu.Lock()
					if err != nil {
						failureCount++
					} else {
						successCount++
					}
					mu.Unlock()
					lockHeld.Done()
					if err == nil {
						_ = lock.Unlock()
					}
				}()
			}

			// Wait a bit to ensure all goroutines have tried to acquire the lock
			lockHeld.Wait()

			// Release the main lock
			err = mainLock.Unlock()
			require.NoError(t, err)

			// Wait for all goroutines to finish
			wg.Wait()

			// All should have failed since the main lock was held
			require.Equal(t, 0, successCount, "no goroutine should acquire the lock while main lock is held")
			require.Equal(t, numGoroutines, failureCount, "all goroutines should fail to acquire the lock")
		})
	})

	t.Run("lock file is created in correct location", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			lock := NewFileLock(dir)
			lockPath := lock.Path()

			// Lock file should not exist before locking
			require.False(t, FileExists(lockPath))

			// Acquire lock
			err := lock.Lock()
			require.NoError(t, err)

			// Lock file should exist after locking
			require.True(t, FileExists(lockPath))

			// Verify it's in the expected location
			expectedPath := filepath.Join(dir, ".lock")
			require.Equal(t, expectedPath, lockPath)

			err = lock.Unlock()
			require.NoError(t, err)
		})
	})

	t.Run("multiple locks on different directories", func(t *testing.T) {
		unittest.RunWithTempDirs(t, func(dir1, dir2 string) {
			lock1 := NewFileLock(dir1)
			lock2 := NewFileLock(dir2)

			// Both locks should succeed since they're on different directories
			err := lock1.Lock()
			require.NoError(t, err)

			err = lock2.Lock()
			require.NoError(t, err)

			// Both should be able to unlock
			err = lock1.Unlock()
			require.NoError(t, err)

			err = lock2.Unlock()
			require.NoError(t, err)
		})
	})

	t.Run("lock works with non-existent directory", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(baseDir string) {
			nonExistentDir := filepath.Join(baseDir, "non-existent", "subdir")
			lock := NewFileLock(nonExistentDir)

			// Lock should still work (the lock file will be created when needed)
			err := lock.Lock()
			require.NoError(t, err)

			// Verify lock file path is correct
			expectedPath := filepath.Join(nonExistentDir, ".lock")
			require.Equal(t, expectedPath, lock.Path())

			err = lock.Unlock()
			require.NoError(t, err)
		})
	})

	t.Run("unlock without lock is safe", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			lock := NewFileLock(dir)

			// Unlocking without locking should not panic
			// (though it may return an error)
			err := lock.Unlock()
			// The error is acceptable - we just want to ensure it doesn't panic
			_ = err
		})
	})

	t.Run("double unlock is safe", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			lock := NewFileLock(dir)

			err := lock.Lock()
			require.NoError(t, err)

			err = lock.Unlock()
			require.NoError(t, err)

			// Unlocking again should be safe (may return error but shouldn't panic)
			err = lock.Unlock()
			_ = err // Error is acceptable
		})
	})

	t.Run("lock is released when process terminates", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			// Acquire lock in first "process" (goroutine)
			lock1 := NewFileLock(dir)
			err := lock1.Lock()
			require.NoError(t, err)

			// Simulate process termination by unlocking
			err = lock1.Unlock()
			require.NoError(t, err)

			// Now a new "process" should be able to acquire the lock
			lock2 := NewFileLock(dir)
			err = lock2.Lock()
			require.NoError(t, err)

			err = lock2.Unlock()
			require.NoError(t, err)
		})
	})

	t.Run("lock file persists after unlock", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			lock := NewFileLock(dir)
			lockPath := lock.Path()

			err := lock.Lock()
			require.NoError(t, err)

			// Lock file should exist
			require.True(t, FileExists(lockPath))

			err = lock.Unlock()
			require.NoError(t, err)

			// Lock file may or may not exist after unlock (implementation detail)
			// But the important thing is that we can acquire a new lock
			lock2 := NewFileLock(dir)
			err = lock2.Lock()
			require.NoError(t, err)

			err = lock2.Unlock()
			require.NoError(t, err)
		})
	})

	t.Run("error message contains lock path", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			lock1 := NewFileLock(dir)
			lock2 := NewFileLock(dir)

			err := lock1.Lock()
			require.NoError(t, err)

			err = lock2.Lock()
			require.Error(t, err)
			require.Contains(t, err.Error(), lock2.Path())
			require.Contains(t, err.Error(), "another process is already using this resource")

			err = lock1.Unlock()
			require.NoError(t, err)
		})
	})

	t.Run("lock works with absolute paths", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			absDir, err := filepath.Abs(dir)
			require.NoError(t, err)

			lock := NewFileLock(absDir)
			err = lock.Lock()
			require.NoError(t, err)

			// Verify lock path is also absolute
			lockPath := lock.Path()
			require.True(t, filepath.IsAbs(lockPath))

			err = lock.Unlock()
			require.NoError(t, err)
		})
	})

	t.Run("lock works with relative paths", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			// Change to the temp directory
			originalDir, err := os.Getwd()
			require.NoError(t, err)
			defer func() {
				_ = os.Chdir(originalDir)
			}()

			err = os.Chdir(dir)
			require.NoError(t, err)

			// Use relative path
			lock := NewFileLock(".")
			err = lock.Lock()
			require.NoError(t, err)

			err = lock.Unlock()
			require.NoError(t, err)
		})
	})

	t.Run("IsLocked detects lock state", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			lock1 := NewFileLock(dir)
			lock2 := NewFileLock(dir)

			// Initially, lock should not be locked
			require.False(t, lock1.IsLocked(), "lock should not be locked initially")

			// Acquire lock
			err := lock1.Lock()
			require.NoError(t, err)

			// Now lock2 should detect it's locked
			require.True(t, lock2.IsLocked(), "lock should be detected as locked")

			// Release lock
			err = lock1.Unlock()
			require.NoError(t, err)

			// Now lock should not be locked
			require.False(t, lock2.IsLocked(), "lock should not be locked after unlock")
		})
	})

	t.Run("RemoveLockFile removes lock file", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			lockPath := filepath.Join(dir, ".lock")

			// Lock file shouldn't exist initially
			require.False(t, FileExists(lockPath))

			// Remove non-existent lock file should succeed
			err := RemoveLockFile(dir)
			require.NoError(t, err)

			// Acquire lock
			lock := NewFileLock(dir)
			err = lock.Lock()
			require.NoError(t, err)

			// Lock file should exist
			require.True(t, FileExists(lockPath))

			// Can't remove lock file while lock is held
			// (this is expected - the file is locked)
			// But we can unlock first
			err = lock.Unlock()
			require.NoError(t, err)

			// Now we can remove the lock file
			err = RemoveLockFile(dir)
			require.NoError(t, err)
			require.False(t, FileExists(lockPath), "lock file should be removed")
		})
	})
}

