package matching

import (
	"sync"
)

// Trapdoor is a concurrency primitive to sleep worker routines, if
// no work is available. A routine providing work activates the
// trapdoor. Repeated activation of an already activated trapdoor
// are no-ops. Once activated, a single routine can pass the trapdoor
// and the trapdoor deactivates again.
//
// Activation and passing of the trapdoor are atomic operations.
// Trapdoor establishes a happens-before relationship:
// the completion of the Activate() call happens before the Pass()
// returns
type Trapdoor struct {
	activated bool
	cnd       *sync.Cond
}

func NewTrapdoor(activated bool) Trapdoor {
	lock := sync.Mutex{}
	return Trapdoor{
		activated: activated,
		cnd:       sync.NewCond(&lock),
	}
}

func (td *Trapdoor) Activate() {
	td.cnd.L.Lock()
	defer td.cnd.L.Unlock()
	if !td.activated {
		td.activated = true
		td.cnd.Signal()
	}
}

func (td *Trapdoor) Pass() {
	td.cnd.L.Lock()
	td.cnd.Wait() // only return when awoken by Broadcast or Signal
	td.activated = false
	td.cnd.L.Unlock()
}
