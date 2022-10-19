package engine

import (
	"context"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"
	"os"
	"sync"
	"testing"
)

type ChannelFifoQueueEngine struct {
	ctx             context.Context
	cancel          context.CancelFunc
	queue           *ChannelFifoQueue
	extraQueue      *ChannelFifoQueue
	targetNumEvents uint64
	eventsProcessed *atomic.Uint64
}

func (e *ChannelFifoQueueEngine) loop(log zerolog.Logger) {
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-e.queue.HeadChannel():
			if e.eventsProcessed.Add(1) >= e.targetNumEvents {
				e.cancel()
			}
		case <-e.extraQueue.HeadChannel():
			if e.eventsProcessed.Add(1) >= e.targetNumEvents {
				e.cancel()
			}
		}
	}
}

func channelFifoQueueEngine() {
	queue, _ := NewChannelFifoQueue()
	extraQueue, _ := NewChannelFifoQueue()
	ctx, cancel := context.WithCancel(context.Background())
	targetNumEvents := uint64(1000)
	engine := &ChannelFifoQueueEngine{
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
			queue.TailChannel() <- struct{}{}
		}
	}()
	go func() {
		for i := uint64(0); i <= targetNumEvents; i++ {
			extraQueue.TailChannel() <- struct{}{}
		}
	}()
	wg.Wait()
	//duration := time.Since(start)
	// Nanoseconds as int64
	//fmt.Println(duration.Nanoseconds())
}

func BenchmarkChannelFifoQueueEngineNotifier(b *testing.B) {
	for i := 0; i < b.N; i++ {
		channelFifoQueueEngine()
	}
}
