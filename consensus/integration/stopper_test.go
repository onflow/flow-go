package integration_test

import (
	"sync"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/model/flow"
)

type StopperConsumer struct {
	notifications.NoopConsumer
}

type Stopper struct {
	sync.Mutex
	running        map[flow.Identifier]struct{}
	nodes          []*Node
	stopping       *atomic.Bool
	finalizedCount uint
	tolerate       int
	stopFunc       func()
	stopped        chan struct{}
}

// How to stop nodes?
// We can stop each node as soon as it enters a certain view. But the problem
// is if some fast nodes reaches a view earlier and gets stopped, it won't
// be available for other nodes to sync, and slow nodes will never be able
// to catch up.
// a better strategy is to wait until all nodes has entered a certain view,
// then stop them all.
//
func NewStopper(finalizedCount uint, tolerate int) *Stopper {
	return &Stopper{
		running:        make(map[flow.Identifier]struct{}),
		nodes:          make([]*Node, 0),
		stopping:       atomic.NewBool(false),
		finalizedCount: finalizedCount,
		tolerate:       tolerate,
		stopped:        make(chan struct{}),
	}
}

func (s *Stopper) AddNode(n *Node) *StopperConsumer {
	s.Lock()
	defer s.Unlock()
	s.running[n.id.ID()] = struct{}{}
	s.nodes = append(s.nodes, n)
	stopConsumer := &StopperConsumer{}
	return stopConsumer
}

// WithStopFunc adds a function to use to stop
func (s *Stopper) WithStopFunc(stop func()) {
	s.stopFunc = stop
}

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

// stopAll signals that all nodes should be stopped be closing the `stopped` channel.
// NOTE: it is the responsibility of the testing code listening on the channel to
// actually stop the nodes.
func (s *Stopper) stopAll() {
	// only allow one process to stop all nodes, and stop them exactly once
	if !s.stopping.CAS(false, true) {
		return
	}
	s.stopFunc()
	close(s.stopped)
}
