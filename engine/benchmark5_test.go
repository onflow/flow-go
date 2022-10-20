package engine

import (
	"context"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"
	"os"
	"sync"
	"testing"
)

type WaitingQueueEngine struct {
	ctx             context.Context
	cancel          context.CancelFunc
	targetNumEvents uint64
	eventsProcessed *atomic.Uint64
}

func (e *WaitingQueueEngine) loop(log zerolog.Logger, queue *WaitingFifoQueue) {
	for {
		_, ok := queue.Pop()
		if ok {
			if e.eventsProcessed.Add(1) >= e.targetNumEvents {
				e.cancel()
			}
			continue
		}
		return
	}
}

func waitingQueueEngine() {
	ctx, cancel := context.WithCancel(context.Background())
	queue, _ := NewWaitingFifoQueue(ctx)
	extraQueue, _ := NewWaitingFifoQueue(ctx)
	targetNumEvents := uint64(1000)
	engine := &WaitingQueueEngine{
		ctx:             ctx,
		cancel:          cancel,
		targetNumEvents: targetNumEvents * 2,
		eventsProcessed: atomic.NewUint64(0),
	}
	numWorkers := 4

	log := zerolog.New(os.Stderr).Level(zerolog.NoLevel)

	var wg sync.WaitGroup
	wg.Add(numWorkers * 2)
	for i := 0; i < numWorkers; i++ {
		logger := log.With().Int("worker", i).Logger()
		go func(logger zerolog.Logger) {
			defer wg.Done()
			engine.loop(logger, queue)
		}(logger)
		go func() {
			defer wg.Done()
			engine.loop(logger, extraQueue)
		}()
	}

	//start := time.Now()
	go func() {
		for i := uint64(0); i <= targetNumEvents; i++ {
			queue.Push(struct{}{})
		}
	}()
	go func() {
		for i := uint64(0); i <= targetNumEvents; i++ {
			extraQueue.Push(struct{}{})
		}
	}()
	wg.Wait()
	//duration := time.Since(start)
	// Nanoseconds as int64
	//fmt.Println(duration.Nanoseconds())
}

func BenchmarkWaitingQueueEngine(b *testing.B) {
	for i := 0; i < b.N; i++ {
		waitingQueueEngine()
	}
}

//b1 - 1000      - 4     - 198239 ns/op
//b2 - 1000      - 4     - 205373 ns/op
//b3 - 1000      - 4     - 240547 ns/op
//b4 - 1000      - 4     - 924907 ns/op
//b5 - 1000      - 4     - 137634 ns/op
//
//b1 - 1000x2  - 4     - 470330 ns/op
//b2 - 1000x2  - 4     - 580251 ns/op
//b3 - 1000x2  - 4     - 580995 ns/op
//b4 - 1000x2  - 4     - 2336128 ns/op
//b5 - 1000x2  - 4x2   - 332127 ns/op
