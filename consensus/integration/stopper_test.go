package integration_test

import (
	"fmt"
	"sync"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
)

// Stopper is responsible for detecting a stopping condition, and stopping all nodes.
//
// Design motivation:
//   - We can stop each node as soon as it enters a certain view. But the problem
//     is if some fast node reaches a view earlier and gets stopped, it won't
//     be available for other nodes to sync, and slow nodes will never be able
//     to catch up.
//   - A better strategy is to wait until all nodes have entered a certain view,
//     then stop them all - this is what the Stopper does.
type Stopper struct {
	sync.Mutex
	running  map[flow.Identifier]struct{}
	stopping *atomic.Bool
	// finalizedCount is the number of blocks which must be finalized (by each node)
	// before the stopFunc is called
	finalizedCount uint
	// tolerate is the number of nodes which we will tolerate NOT finalizing the
	// expected number of blocks
	tolerate int
	stopFunc func()
	stopped  chan struct{}
}

func NewStopper(finalizedCount uint, tolerate int) *Stopper {
	return &Stopper{
		running:        make(map[flow.Identifier]struct{}),
		stopping:       atomic.NewBool(false),
		finalizedCount: finalizedCount,
		tolerate:       tolerate,
		stopped:        make(chan struct{}),
	}
}

// AddNode registers a node with the Stopper, so that the stopping condition is
// adjusted to account for this node (ie. we will now also wait for the added
// node to finalize the desired number of blocks).
func (s *Stopper) AddNode(n *Node) {
	s.Lock()
	defer s.Unlock()
	s.running[n.id.ID()] = struct{}{}
}

// WithStopFunc adds a function to use to stop all nodes (typically the cancel function of the context used to start them).
// Caution: not safe for concurrent use by multiple goroutines.
func (s *Stopper) WithStopFunc(stop func()) {
	s.stopFunc = stop
}

// onFinalizedTotal is called via CounterConsumer each time a node finalizes a block.
// When called, the node with ID `id` has finalized `total` blocks.
func (s *Stopper) onFinalizedTotal(id flow.Identifier, total uint) {
	s.Lock()
	defer s.Unlock()

	if total < s.finalizedCount {
		return
	}

	// keep track of remaining running nodes
	delete(s.running, id)

	// if all the honest nodes have reached the total number of
	// finalized blocks, then stop all nodes
	if len(s.running) <= s.tolerate {
		go s.stopAll()
	}
}

// stopAll stops all registered nodes, using the provided stopFunc.
func (s *Stopper) stopAll() {
	// only allow one process to stop all nodes, and stop them exactly once
	if !s.stopping.CompareAndSwap(false, true) {
		return
	}
	if s.stopFunc == nil {
		panic("Stopper used without a stopFunc - use WithStopFunc to specify how to stop nodes once stop condition is reached")
	}
	fmt.Println("stopping all nodes...")
	s.stopFunc()
	close(s.stopped)
}
