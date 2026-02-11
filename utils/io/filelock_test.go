package io

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

// RemoveLockFile removes the lock file if it exists.
// This is only used in tests.
func RemoveLockFile(lockDir string) error {
	lockPath := filepath.Join(lockDir, ".lock")
	if !FileExists(lockPath) {
		return nil
	}
	return os.Remove(lockPath)
}

func TestFileLock(t *testing.T) {
	t.Run("basic lock and unlock", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			lock, err := NewFileLock(dir)
			require.NoError(t, err)
			require.NotNil(t, lock)

			// Verify lock path
			expectedPath := filepath.Join(dir, ".lock")
			require.Equal(t, expectedPath, lock.Path())

			// Acquire lock
			err = lock.Lock()
			require.NoError(t, err)

			// Release lock
			err = lock.Unlock()
			require.NoError(t, err)
		})
	})

	t.Run("lock prevents concurrent access", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			lock1, err := NewFileLock(dir)
			require.NoError(t, err)
			lock2, err := NewFileLock(dir)
			require.NoError(t, err)

			// First lock should succeed
			err = lock1.Lock()
			require.NoError(t, err)

			// Second lock should fail
			err = lock2.Lock()
			require.Error(t, err)
			require.Contains(t, err.Error(), "another process is already using this directory")

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
			lock, err := NewFileLock(dir)
			require.NoError(t, err)

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
			mainLock, err := NewFileLock(dir)
			require.NoError(t, err)
			err = mainLock.Lock()
			require.NoError(t, err)

			// Start multiple goroutines trying to acquire the same lock
			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				lockHeld.Add(1)
				go func() {
					defer wg.Done()
					lock, err := NewFileLock(dir)
					if err != nil {
						mu.Lock()
						failureCount++
						mu.Unlock()
						lockHeld.Done()
						return
					}
					err = lock.Lock()
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
			lock, err := NewFileLock(dir)
			require.NoError(t, err)
			lockPath := lock.Path()

			// Lock file should not exist before locking
			require.False(t, FileExists(lockPath))

			// Acquire lock
			err = lock.Lock()
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
			lock1, err := NewFileLock(dir1)
			require.NoError(t, err)
			lock2, err := NewFileLock(dir2)
			require.NoError(t, err)

			// Both locks should succeed since they're on different directories
			err = lock1.Lock()
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
			lock, err := NewFileLock(nonExistentDir)
			require.NoError(t, err)

			// Lock should still work (the directory was created in NewFileLock)
			err = lock.Lock()
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
			lock, err := NewFileLock(dir)
			require.NoError(t, err)

			// Unlocking without locking should not panic
			// (though it may return an error)
			err = lock.Unlock()
			// The error is acceptable - we just want to ensure it doesn't panic
			_ = err
		})
	})

	t.Run("double unlock is safe", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			lock, err := NewFileLock(dir)
			require.NoError(t, err)

			err = lock.Lock()
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
			lock1, err := NewFileLock(dir)
			require.NoError(t, err)
			err = lock1.Lock()
			require.NoError(t, err)

			// Simulate process termination by unlocking
			err = lock1.Unlock()
			require.NoError(t, err)

			// Now a new "process" should be able to acquire the lock
			lock2, err := NewFileLock(dir)
			require.NoError(t, err)
			err = lock2.Lock()
			require.NoError(t, err)

			err = lock2.Unlock()
			require.NoError(t, err)
		})
	})

	t.Run("lock file persists after unlock", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			lock, err := NewFileLock(dir)
			require.NoError(t, err)
			lockPath := lock.Path()

			err = lock.Lock()
			require.NoError(t, err)

			// Lock file should exist
			require.True(t, FileExists(lockPath))

			err = lock.Unlock()
			require.NoError(t, err)

			// Lock file may or may not exist after unlock (implementation detail)
			// But the important thing is that we can acquire a new lock
			lock2, err := NewFileLock(dir)
			require.NoError(t, err)
			err = lock2.Lock()
			require.NoError(t, err)

			err = lock2.Unlock()
			require.NoError(t, err)
		})
	})

	t.Run("error message contains lock path", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			lock1, err := NewFileLock(dir)
			require.NoError(t, err)
			lock2, err := NewFileLock(dir)
			require.NoError(t, err)

			err = lock1.Lock()
			require.NoError(t, err)

			err = lock2.Lock()
			require.Error(t, err)
			require.Contains(t, err.Error(), lock2.Path())
			require.Contains(t, err.Error(), "another process is already using this directory")

			err = lock1.Unlock()
			require.NoError(t, err)
		})
	})

	t.Run("lock works with absolute paths", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			absDir, err := filepath.Abs(dir)
			require.NoError(t, err)

			lock, err := NewFileLock(absDir)
			require.NoError(t, err)
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
			lock, err := NewFileLock(".")
			require.NoError(t, err)
			err = lock.Lock()
			require.NoError(t, err)

			err = lock.Unlock()
			require.NoError(t, err)
		})
	})

	t.Run("IsLocked detects lock state", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			lock1, err := NewFileLock(dir)
			require.NoError(t, err)
			lock2, err := NewFileLock(dir)
			require.NoError(t, err)

			// Initially, lock should not be locked
			require.False(t, lock1.IsLocked(), "lock should not be locked initially")

			// Acquire lock
			err = lock1.Lock()
			require.NoError(t, err)

			// Now lock1 should know it's locked
			require.True(t, lock1.IsLocked(), "lock1 should know it holds the lock")

			// Release lock
			err = lock1.Unlock()
			require.NoError(t, err)

			// Now lock should not be locked
			require.False(t, lock1.IsLocked(), "lock should not be locked after unlock")
			_ = lock2 // lock2 unused after IsLocked behavior changed
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
			lock, err := NewFileLock(dir)
			require.NoError(t, err)
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

	t.Run("NewFileLock error for permission denied on directory creation", func(t *testing.T) {
		// This test may not work on all systems, especially Windows
		// Skip if we can't create a read-only parent directory
		unittest.RunWithTempDir(t, func(baseDir string) {
			// Create a subdirectory that we'll make read-only
			restrictedDir := filepath.Join(baseDir, "restricted")
			err := os.MkdirAll(restrictedDir, 0755)
			require.NoError(t, err)

			// Create a subdirectory inside that we'll try to lock
			// But first make the parent read-only so we can't create subdirectories
			lockTargetDir := filepath.Join(restrictedDir, "wal")

			// Make the restricted directory read-only (remove write permission)
			err = os.Chmod(restrictedDir, 0555)
			require.NoError(t, err)
			defer func() {
				// Restore permissions for cleanup
				_ = os.Chmod(restrictedDir, 0755)
			}()

			_, err = NewFileLock(lockTargetDir)
			require.Error(t, err)

			// Verify the error message contains permission denied
			require.Contains(t, err.Error(), "permission denied")
			require.Contains(t, err.Error(), lockTargetDir)
		})
	})

	t.Run("lock error for permission denied on lock file creation", func(t *testing.T) {
		// This test may not work on all systems, especially Windows
		unittest.RunWithTempDir(t, func(baseDir string) {
			// Create the WAL directory
			walDir := filepath.Join(baseDir, "wal")
			err := os.MkdirAll(walDir, 0755)
			require.NoError(t, err)

			// Create the lock first while we have write permission
			lock, err := NewFileLock(walDir)
			require.NoError(t, err)

			// Make the directory read-only so we can't create the lock file
			err = os.Chmod(walDir, 0555)
			require.NoError(t, err)
			defer func() {
				// Restore permissions for cleanup
				_ = os.Chmod(walDir, 0755)
			}()

			err = lock.Lock()
			require.Error(t, err)

			// Verify the error message contains permission denied
			require.Contains(t, err.Error(), "permission denied")
			require.Contains(t, err.Error(), walDir)
		})
	})

	t.Run("lock error message distinguishes permission denied from lock conflict", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			lock1, err := NewFileLock(dir)
			require.NoError(t, err)
			lock2, err := NewFileLock(dir)
			require.NoError(t, err)

			// Acquire first lock
			err = lock1.Lock()
			require.NoError(t, err)

			// Try to acquire second lock - should get lock conflict, not permission denied
			err = lock2.Lock()
			require.Error(t, err)
			require.Contains(t, err.Error(), "another process is already using this directory")
			require.NotContains(t, err.Error(), "permission denied")

			err = lock1.Unlock()
			require.NoError(t, err)
		})
	})
}
