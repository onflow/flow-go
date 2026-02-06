package io

import (
	"errors"
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
}

// NewFileLock creates a new file lock at the specified path.
// The lock file will be created in the same directory as the path.
// If path is a directory, the lock file will be created inside it.
// If the directory doesn't exist yet, it assumes the path is intended to be a directory.
func NewFileLock(path string) (*FileLock, error) {
	// Determine the lock file path
	// Always create the lock file in the specified path (treating it as a directory)
	// This ensures the lock is always in the WAL directory itself
	lockPath := filepath.Join(path, ".lock")

	// Ensure the directory exists before trying to create the lock file
	dir := filepath.Dir(lockPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("failed to create directory for lock file %s (permission denied): %w", lockPath, err)
		}
		return nil, fmt.Errorf("failed to create directory for lock file %s: %w", lockPath, err)
	}

	return &FileLock{
		lockFile: flock.New(lockPath),
	}, nil
}

// Lock acquires an exclusive lock on the file. This will block until the lock
// can be acquired. If the lock cannot be acquired (e.g., another process holds it),
// it returns an error. The process should crash in this case.
func (fl *FileLock) Lock() error {
	locked, err := fl.lockFile.TryLock()
	if err != nil {
		// Check if the error is due to permission denied
		var pathErr *os.PathError
		if errors.Is(err, os.ErrPermission) || (errors.As(err, &pathErr) && os.IsPermission(pathErr.Err)) {
			return fmt.Errorf("failed to acquire file lock at %s (permission denied): %w", fl.lockFile.Path(), err)
		}
		return fmt.Errorf("failed to acquire file lock at %s: %w", fl.lockFile.Path(), err)
	}
	if !locked {
		// Lock file exists and is held by another process
		return fmt.Errorf("cannot acquire exclusive lock at %s: another process is already using this directory", fl.lockFile.Path())
	}
	return nil
}

// Unlock releases the file lock.
func (fl *FileLock) Unlock() error {
	if err := fl.lockFile.Unlock(); err != nil {
		return fmt.Errorf("failed to release file lock at %s: %w", fl.lockFile.Path(), err)
	}
	return nil
}

// Close releases the file lock. Implements io.Closer.
func (fl *FileLock) Close() error {
	return fl.lockFile.Close()
}

// Path returns the path to the lock file.
func (fl *FileLock) Path() string {
	return fl.lockFile.Path()
}

// IsLocked returns true if this FileLock instance currently holds the lock.
func (fl *FileLock) IsLocked() bool {
	return fl.lockFile.Locked()
}
