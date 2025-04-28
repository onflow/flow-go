package operation

import "sync"

// BatchLocks is a struct that holds the locks acquired by a batch,
// which is used to prevent re-entrant deadlock.
// BatchLocks is not safe for concurrent use by multiple goroutines.
// Deprecated: BatchLocks exists to provide deadlock protection as a temporary measure during
// the course of development of the Pebble database layer -- to be replaced prior to release with
// a system without reliance on globally unique mutex references.
type BatchLocks struct {
	// CAUTION: this map is keyed by the pointer address of the mutex. Users must ensure
	// that only one reference exists to the relevant lock.
	acquiredLocks map[*sync.Mutex]struct{}
}

func NewBatchLocks() *BatchLocks {
	return &BatchLocks{
		acquiredLocks: nil, // lazy initialization
	}
}

// Lock tries to acquire a given lock on behalf of the batch.
//
// If the batch has already acquired this lock earlier (recorded in acquiredLocks),
// it skips locking again to avoid unnecessary blocking, allowing the caller to proceed immediately.
//
// If the lock has not been acquired yet, it blocks until the lock is acquired,
// and then records the lock in the acquiredLocks map to indicate ownership.
//
// It also registers a callback to ensure that when the batch operation is finished,
// the lock is properly released and removed from acquiredLocks.
//
// Parameters:
//   - lock: The *sync.Mutex to acquire. The common usage of this lock is to prevent
//     dirty reads so that the batch writes is writing the correct data.
//     In other words, this Lock method is to prevent re-entrant deadlock, while this lock
//     mutex is used to prevent dirty reads.
//   - callback: A Callbacks collection to which the unlock operation is appended
//     so that locks are safely released once the batch processing is complete.
//
// CAUTION: Since locks are identified by pointer address, callers must ensure that no other references exist for the input lock.
func (l *BatchLocks) Lock(lock *sync.Mutex, callback *Callbacks) {
	// if the lock is already acquired by this same batch from previous db operations,
	// then it will not be blocked and can continue updating the batch,
	if l.acquiredLocks != nil {
		if _, ok := l.acquiredLocks[lock]; ok {
			// the batch is already holding the lock
			// so we can just return and the caller is unblock to continue
			return
		}
	}

	// batch never hold this lock before, then trying to acquire it
	// this will block until the lock is acquired
	lock.Lock()

	if l.acquiredLocks == nil {
		l.acquiredLocks = make(map[*sync.Mutex]struct{})
	}

	// once we acquire the lock, we need to add it to the acquired locks so that
	// other operations from this batch can be unblocked
	l.acquiredLocks[lock] = struct{}{}

	// we need to make sure that the lock is released when the batch is done
	callback.AddCallback(func(error) {
		delete(l.acquiredLocks, lock)
		lock.Unlock()
	})
}
