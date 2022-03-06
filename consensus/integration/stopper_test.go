package integration_test

import (
	"context"
	"fmt"
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
	stopped        chan struct{}
	cancel         context.CancelFunc
}

// How to stop nodes?
// We can stop each node as soon as it enters a certain view. But the problem
// is if some fast nodes reaches a view earlier and gets stopped, it won't
// be available for other nodes to sync, and slow nodes will never be able
// to catch up.
// a better strategy is to wait until all nodes has entered a certain view,
// then stop them all.
//
func NewStopper(finalizedCount uint, tolerate int, cancel context.CancelFunc) *Stopper {
	return &Stopper{
		running:        make(map[flow.Identifier]struct{}),
		nodes:          make([]*Node, 0),
		stopping:       atomic.NewBool(false),
		finalizedCount: finalizedCount,
		tolerate:       tolerate,
		stopped:        make(chan struct{}),
		cancel:         cancel,
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

func (s *Stopper) stopAll() {
	// only allow one process to stop all nodes,
	// and stop them exactly once
	if !s.stopping.CAS(false, true) {
		return
	}

	fmt.Println("stopping all nodes")

	s.cancel()

	// wait until all nodes has been shut down
	var wg sync.WaitGroup
	for _, node := range s.nodes {
		wg.Add(1)
		go func(node *Node) {
			node.Shutdown()
			wg.Done()
		}(node)
	}
	wg.Wait()

	close(s.stopped)
}
