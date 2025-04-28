package operation

import "sync"

// BatchLocks is a struct that holds the locks acquired by a batch,
// which is used to prevent re-entrant deadlock.
type BatchLocks struct {
	acquiredLocks map[*sync.Mutex]struct{}
}

func NewLocks() *BatchLocks {
	return &BatchLocks{
		acquiredLocks: nil, // lazy initialization
	}
}

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
