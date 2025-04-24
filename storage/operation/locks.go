package operation

import "sync"

type Locks struct {
	acquiredLocks map[*sync.Mutex]struct{}
}

func NewLocks() *Locks {
	return &Locks{
		acquiredLocks: make(map[*sync.Mutex]struct{}),
	}
}

func (l *Locks) Lock(lock *sync.Mutex, callback *Callbacks) {
	if _, ok := l.acquiredLocks[lock]; ok {
		// the batch is already holding the lock
		return
	}

	// batch never hold this lock before, then trying to acquire it
	// this will block until the lock is acquired
	lock.Lock()
	// once we acquire the lock, we need to add it to the acquired locks so that
	// other operations from this batch can be unblocked
	l.acquiredLocks[lock] = struct{}{}

	// we need to make sure that the lock is released when the batch is done
	callback.AddCallback(func(error) {
		delete(l.acquiredLocks, lock)
		lock.Unlock()
	})
}
