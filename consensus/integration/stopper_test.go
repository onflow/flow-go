package integration_test

import (
	"fmt"
	"sync"
	"time"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/model/flow"
)

type StopperConsumer struct {
	notifications.NoopConsumer
}

func (c *StopperConsumer) OnStartingTimeout(info *model.TimerInfo) {
	threshold := 30 * time.Second
	if info.Duration > threshold {
		panic(fmt.Sprintf("stop,%v", info.Duration))
	}
}

type Stopper struct {
	sync.Mutex
	running  map[flow.Identifier]struct{}
	nodes    []*Node
	stopping bool
	stopped  chan struct{}
	onStop   func(nodes []*Node) bool
}

// How to stop nodes?
// We can stop each node as soon as it enters a certain view. But the problem
// is if some fast nodes reaches a view earlier and gets stopped, it won't
// be available for other nodes to sync, and slow nodes will never be able
// to catch up.
// a better strategy is to wait until all nodes has entered a certain view,
// then stop them all.
//
// stopAtView - if all node's curview have reached this view, then all nodes will be terminated.
// stopAtCount - if any node has finalized a total of "stopAtCount" blocks, then all nodes will be
// terminated.
// for example: NewStopper(100, 10000) means all nodes will be terminated if all nodes have passed
// view 100 or any node has finalized 10000 blocks.
func NewStopper(onStop func([]*Node) bool) *Stopper {
	return &Stopper{
		running:  make(map[flow.Identifier]struct{}),
		nodes:    make([]*Node, 0),
		stopping: false,
		stopped:  make(chan struct{}),
		onStop:   onStop,
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

	if s.onStop(s.nodes) {
		return
	}

	// keep track of remaining running nodes
	delete(s.running, id)

	// if there is no running nodes, stop all
	if len(s.running) == 0 {
		// terminating all nodes in a different goroutine,
		// which will block hotstuff eventloop from exiting, and
		// cause deadlock
		go s.stopAll()
	}
}

func (s *Stopper) stopAll() {
	// has been stopped before
	if s.stopping {
		return
	}

	s.stopping = true

	// wait until all nodes has been shut down
	var wg sync.WaitGroup
	for i := 0; i < len(s.nodes); i++ {
		wg.Add(1)
		// stop compliance will also stop both hotstuff and synchronization engine
		go func(i int) {
			// TODO better to wait until it's done, but needs to figure out why hotstuff.Done doesn't finish
			<-s.nodes[i].compliance.Done()
			wg.Done()
		}(i)
	}
	wg.Wait()
	close(s.stopped)
}
