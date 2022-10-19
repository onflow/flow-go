package engine

import (
	"context"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"
	"os"
	"sync"
	"testing"
)

type SemaTrapdoor struct {
	maxWeight uint64
	activated uint64
	closed    bool
	cnd       *sync.Cond
}

func NewSemaTrapdoor(workers uint64, ctx context.Context) *SemaTrapdoor {
	lock := sync.Mutex{}
	trapdoor := &SemaTrapdoor{
		activated: 0,
		maxWeight: workers,
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

func (td *SemaTrapdoor) Activate() {
	td.cnd.L.Lock()
	defer td.cnd.L.Unlock()
	if td.activated < td.maxWeight {
		td.activated++
		td.cnd.Signal()
	}
}

func (td *SemaTrapdoor) Pass() bool {
	td.cnd.L.Lock()
	for td.activated == 0 && !td.closed {
		td.cnd.Wait() // only return when awoken by Broadcast or Signal
	}
	if td.activated > 0 {
		td.activated--
	}
	closed := td.closed
	td.cnd.L.Unlock()
	return !closed
}

type SemaEngine struct {
	trapdoor        *SemaTrapdoor
	ctx             context.Context
	cancel          context.CancelFunc
	queue           *fifoqueue.FifoQueue
	extraQueue      *fifoqueue.FifoQueue
	targetNumEvents uint64
	eventsProcessed *atomic.Uint64
}

func (e *SemaEngine) loop(log zerolog.Logger) {
	for {
		passed := e.trapdoor.Pass()
		if !passed {
			return
		}
		e.processAvailableMessages()
	}
}

func (e *SemaEngine) processAvailableMessages() {
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

func engineSemaNotifier() {
	queue, _ := fifoqueue.NewFifoQueue()
	extraQueue, _ := fifoqueue.NewFifoQueue()
	ctx, cancel := context.WithCancel(context.Background())
	numWorkers := 4
	targetNumEvents := uint64(1000)
	engine := &SemaEngine{
		trapdoor:        NewSemaTrapdoor(uint64(numWorkers), ctx),
		ctx:             ctx,
		cancel:          cancel,
		queue:           queue,
		extraQueue:      extraQueue,
		targetNumEvents: targetNumEvents * 2,
		eventsProcessed: atomic.NewUint64(0),
	}

	log := zerolog.New(os.Stderr).Level(zerolog.NoLevel)

	var wg sync.WaitGroup
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func(log zerolog.Logger) {
			log.Debug().Msg("starting worker loop")
			defer wg.Done()
			engine.loop(log)
		}(log.With().Int("worker", i).Logger())
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

func BenchmarkSemaEngine(b *testing.B) {
	for i := 0; i < b.N; i++ {
		engineSemaNotifier()
	}
}
