package engine

import (
	"context"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"go.uber.org/atomic"
	"sync"
	"testing"
)

type Trapdoor struct {
	activated bool
	closed    bool
	cnd       *sync.Cond
}

func NewTrapdoor(ctx context.Context) *Trapdoor {
	lock := sync.Mutex{}
	trapdoor := &Trapdoor{
		activated: false,
		closed:    false,
		cnd:       sync.NewCond(&lock),
	}

	go func() {
		<-ctx.Done()
		trapdoor.cnd.L.Lock()
		trapdoor.closed = true
		trapdoor.cnd.Broadcast()
		trapdoor.cnd.L.Unlock()
	}()

	return trapdoor
}

func (td *Trapdoor) Activate() {
	td.cnd.L.Lock()
	defer td.cnd.L.Unlock()
	if !td.activated {
		td.activated = true
		td.cnd.Signal()
	}
}

func (td *Trapdoor) Pass() bool {
	td.cnd.L.Lock()
	for !td.activated && !td.closed {
		td.cnd.Wait() // only return when awoken by Broadcast or Signal
	}
	td.activated = false
	closed := td.closed
	td.cnd.L.Unlock()
	return !closed
}

type TrapdoorEngine struct {
	trapdoor        *Trapdoor
	ctx             context.Context
	cancel          context.CancelFunc
	queue           *fifoqueue.FifoQueue
	extraQueue      *fifoqueue.FifoQueue
	targetNumEvents uint64
	eventsProcessed *atomic.Uint64
}

func (e *TrapdoorEngine) loop() {
	for {
		passed := e.trapdoor.Pass()
		if !passed {
			return
		}
		e.processAvailableMessages()
	}
}

func (e *TrapdoorEngine) processAvailableMessages() {
	for {
		_, ok := e.queue.Pop()
		if ok {
			if e.eventsProcessed.Add(1) >= e.targetNumEvents {
				e.cancel()
			}
			continue
		}

		_, ok = e.extraQueue.Pop()
		if ok {
			if e.eventsProcessed.Add(1) >= e.targetNumEvents {
				e.cancel()
			}
			continue
		}
		return
	}
}

func engineTrapdoorNotifier() {
	queue, _ := fifoqueue.NewFifoQueue()
	extraQueue, _ := fifoqueue.NewFifoQueue()
	ctx, cancel := context.WithCancel(context.Background())
	targetNumEvents := uint64(1000)
	engine := &TrapdoorEngine{
		trapdoor:        NewTrapdoor(ctx),
		ctx:             ctx,
		cancel:          cancel,
		queue:           queue,
		extraQueue:      extraQueue,
		targetNumEvents: targetNumEvents * 2,
		eventsProcessed: atomic.NewUint64(0),
	}
	numWorkers := 4

	var wg sync.WaitGroup
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			engine.loop()
		}()
	}

	//start := time.Now()
	go func() {
		for i := uint64(0); i <= targetNumEvents; i++ {
			queue.Push(struct{}{})
			engine.trapdoor.Activate()
		}
	}()
	go func() {
		for i := uint64(0); i <= targetNumEvents; i++ {
			extraQueue.Push(struct{}{})
			engine.trapdoor.Activate()
		}
	}()
	wg.Wait()
	//duration := time.Since(start)
	// Nanoseconds as int64
	//fmt.Println(duration.Nanoseconds())
}

func BenchmarkTrapdoorEngine(b *testing.B) {
	for i := 0; i < b.N; i++ {
		engineTrapdoorNotifier()
	}
}

func TestNotifiers(t *testing.T) {
	//engineCurrentNotifier()
	//engineTrapdoorNotifier()
	//engineSemaNotifier()
	waitingQueueEngine()
}
