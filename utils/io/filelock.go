package io

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

// FileLock represents an exclusive file lock that prevents multiple processes
// from accessing the same resource. If another process tries to acquire the lock,
// it will fail and should crash.
type FileLock struct {
	lockFile *flock.Flock
	path     string
}

// NewFileLock creates a new file lock at the specified path.
// The lock file will be created in the same directory as the path.
// If path is a directory, the lock file will be created inside it.
// If the directory doesn't exist yet, it assumes the path is intended to be a directory.
func NewFileLock(path string) *FileLock {
	// Determine the lock file path
	// Always create the lock file in the specified path (treating it as a directory)
	// This ensures the lock is always in the WAL directory itself
	lockPath := filepath.Join(path, ".lock")

	return &FileLock{
		lockFile: flock.New(lockPath),
		path:     lockPath,
	}
}

// Lock acquires an exclusive lock on the file. This will block until the lock
// can be acquired. If the lock cannot be acquired (e.g., another process holds it),
// it returns an error. The process should crash in this case.
func (fl *FileLock) Lock() error {
	// Ensure the directory exists before trying to create the lock file
	dir := filepath.Dir(fl.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for lock file %s: %w", fl.path, err)
	}

	locked, err := fl.lockFile.TryLock()
	if err != nil {
		return fmt.Errorf("failed to acquire file lock at %s: %w", fl.path, err)
	}
	if !locked {
		return fmt.Errorf("cannot acquire exclusive lock on %s: another process is already using this resource", fl.path)
	}
	return nil
}

// Unlock releases the file lock.
func (fl *FileLock) Unlock() error {
	if err := fl.lockFile.Unlock(); err != nil {
		return fmt.Errorf("failed to release file lock at %s: %w", fl.path, err)
	}
	return nil
}

// Path returns the path to the lock file.
func (fl *FileLock) Path() string {
	return fl.path
}

// IsLocked checks if the lock file exists and is currently locked by another process.
// It returns true if the lock cannot be acquired (meaning another process holds it),
// and false if the lock can be acquired (meaning no process holds it).
func (fl *FileLock) IsLocked() bool {
	locked, err := fl.lockFile.TryLock()
	if err != nil {
		// If there's an error, assume it's locked to be safe
		return true
	}
	if locked {
		// We successfully acquired it, so it wasn't locked
		// Release it immediately
		_ = fl.lockFile.Unlock()
		return false
	}
	// Couldn't acquire it, so it's locked
	return true
}

// RemoveLockFile removes the lock file if it exists.
// This should only be used when you're certain no process is holding the lock,
// as removing the file while a lock is held can cause issues.
func RemoveLockFile(lockDir string) error {
	lockPath := filepath.Join(lockDir, ".lock")
	if !FileExists(lockPath) {
		return nil
	}
	return os.Remove(lockPath)
}
