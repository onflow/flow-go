package unittest

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jordanschalm/lockctx"
)

// WithLock creates a lock context from the given manager, acquires the given lock, then executes the function `fn`.
// Error returns:
//   - Errors produced by the injected function `fn` are directly propagated to the caller.
//   - Errors during lock acquisition are wrapped in a StorageLockAcquisitionError.
func WithLock(t testing.TB, manager lockctx.Manager, lockID string, fn func(lctx lockctx.Context) error) error {
	t.Helper()

	lctx := manager.NewContext()
	defer lctx.Release()
	err := lctx.AcquireLock(lockID)
	if err != nil {
		return NewStorageLockAcquisitionErrorf("failed to acquire lock %s: %w", lockID, err)
	}

	return fn(lctx)
}

// StorageLockAcquisitionError indicates that acquiring a storage lock failed.
type StorageLockAcquisitionError struct {
	err error
}

func NewStorageLockAcquisitionErrorf(msg string, args ...interface{}) error {
	return StorageLockAcquisitionError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e StorageLockAcquisitionError) Unwrap() error {
	return e.err
}

func (e StorageLockAcquisitionError) Error() string {
	return e.err.Error()
}

func IsStorageLockAcquisitionError(err error) bool {
	var targetErr StorageLockAcquisitionError
	return errors.As(err, &targetErr)
}

// WithLocks creates a lock context from the given manager, acquires the given locks, then executes the function `fn`.
// The test will fail if we are unable to acquire any of the locks or if `fn` returns any non-nil error.
// Error returns:
//   - Errors produced by the injected function `fn` are directly propagated to the caller.
//   - Errors during lock acquisition are wrapped in a StorageLockAcquisitionError.
func WithLocks(t testing.TB, manager lockctx.Manager, lockIDs []string, fn func(lctx lockctx.Context) error) error {
	t.Helper()

	lctx := manager.NewContext()
	defer lctx.Release()

	for _, lockID := range lockIDs {
		err := lctx.AcquireLock(lockID)
		if err != nil {
			return NewStorageLockAcquisitionErrorf("failed to acquire lock %s: %w", lockID, err)
		}
	}

	return fn(lctx)
}
