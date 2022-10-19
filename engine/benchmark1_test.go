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

type Engine struct {
	notifier        Notifier
	ctx             context.Context
	cancel          context.CancelFunc
	queue           *fifoqueue.FifoQueue
	extraQueue      *fifoqueue.FifoQueue
	targetNumEvents uint64
	eventsProcessed *atomic.Uint64
}

func (e *Engine) loop(log zerolog.Logger) {
	notifier := e.notifier.Channel()
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-notifier:
			e.processAvailableMessages()
		}
	}
}

func (e *Engine) processAvailableMessages() {
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

func engineCurrentNotifier() {
	queue, _ := fifoqueue.NewFifoQueue()
	extraQueue, _ := fifoqueue.NewFifoQueue()
	ctx, cancel := context.WithCancel(context.Background())
	targetNumEvents := uint64(1000)
	engine := &Engine{
		notifier:        NewNotifier(),
		ctx:             ctx,
		cancel:          cancel,
		queue:           queue,
		extraQueue:      extraQueue,
		targetNumEvents: targetNumEvents * 2,
		eventsProcessed: atomic.NewUint64(0),
	}
	numWorkers := 4

	log := zerolog.New(os.Stderr).Level(zerolog.NoLevel)

	var wg sync.WaitGroup
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		log := log.With().Int("worker", i).Logger()
		go func() {
			defer wg.Done()
			engine.loop(log)
		}()
	}

	//start := time.Now()
	go func() {
		for i := uint64(0); i <= targetNumEvents; i++ {
			queue.Push(struct{}{})
			engine.notifier.Notify()
		}
	}()
	go func() {
		for i := uint64(0); i <= targetNumEvents; i++ {
			extraQueue.Push(struct{}{})
			engine.notifier.Notify()
		}
	}()
	wg.Wait()
	//duration := time.Since(start)
	// Nanoseconds as int64
	//fmt.Println(duration.Nanoseconds())
}

func BenchmarkEngineCurrentNotifier(b *testing.B) {
	for i := 0; i < b.N; i++ {
		engineCurrentNotifier()
	}
}
