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
	onEnteringView func(view uint64)
}

func (c *StopperConsumer) OnEnteringView(view uint64) {
	c.onEnteringView(view)
}

func (c *StopperConsumer) OnStartingTimeout(info *model.TimerInfo) {
	threshold := 30 * time.Second
	if info.Duration > threshold {
		panic(fmt.Sprintf("stop,%v", info.Duration))
	}
}

type Stopper struct {
	sync.Mutex
	running     map[flow.Identifier]struct{}
	nodes       []*Node
	stopping    bool
	stopAtView  uint64
	stopAtCount uint
	stopped     chan struct{}
}

// How to stop nodes?
// We can stop each node as soon as it enters a certain view. But the problem
// is if some fast nodes reaches a view earlier and gets stopped, it won't
// be available for other nodes to sync, and slow nodes will never be able
// to catch up.
// a better strategy is to wait until all nodes has entered a certain view,
// then stop them all.
func NewStopper(stopAtView uint64, stopAtCount uint) *Stopper {
	return &Stopper{
		running:     make(map[flow.Identifier]struct{}),
		nodes:       make([]*Node, 0),
		stopping:    false,
		stopAtView:  stopAtView,
		stopAtCount: stopAtCount,
		stopped:     make(chan struct{}),
	}
}

func (s *Stopper) AddNode(n *Node) *StopperConsumer {
	s.Lock()
	defer s.Unlock()
	s.running[n.id.ID()] = struct{}{}
	s.nodes = append(s.nodes, n)
	stopConsumer := &StopperConsumer{
		onEnteringView: func(view uint64) {
			s.onEnteringView(n.id.ID(), view)
		},
	}
	return stopConsumer
}

func (s *Stopper) onEnteringView(id flow.Identifier, view uint64) {
	s.Lock()
	defer s.Unlock()

	if view < s.stopAtView {
		return
	}

	// keep track of remaining running nodes
	delete(s.running, id)

	// if there is no running nodes, stop all
	if len(s.running) == 0 {
		s.stopAll()
	}
}

func (s *Stopper) onFinalizedTotal(id flow.Identifier, total uint) {
	s.Lock()
	defer s.Unlock()

	if total < s.stopAtCount {
		return
	}

	// keep track of remaining running nodes
	delete(s.running, id)

	// if there is no running nodes, stop all
	if len(s.running) == 0 {
		s.stopAll()
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
			s.nodes[i].compliance.Done()
			s.nodes[i].hot.Done()
			// 2 seconds waiting is more than enough for local, but sometime still not enough on slow CI environment.
			// if there is error like this, it means we didn't wait long enough before closing the database
			// panic: runtime error: invalid memory address or nil pointer dereference
			// [signal SIGSEGV: segmentation violation code=0x1 addr=0x10 pc=0x94fc7a]
			// goroutine 147 [running]:
			// github.com/dgraph-io/badger/v2/skl.(*Skiplist).IncrRef(...)
			time.Sleep(2 * time.Second)
			wg.Done()
		}(i)
	}
	wg.Wait()
	close(s.stopped)
}
