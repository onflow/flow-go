package engine

import (
	"context"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"
	"os"
	"sync"
	"testing"
)

type ChannelQueueEngine struct {
	ctx             context.Context
	cancel          context.CancelFunc
	queue           chan struct{}
	extraQueue      chan struct{}
	targetNumEvents uint64
	eventsProcessed *atomic.Uint64
}

func (e *ChannelQueueEngine) loop(log zerolog.Logger) {
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-e.queue:
			if e.eventsProcessed.Add(1) >= e.targetNumEvents {
				e.cancel()
			}
			//case <-e.extraQueue:
			//	if e.eventsProcessed.Add(1) >= e.targetNumEvents {
			//		e.cancel()
			//	}
		}
	}
}

func channelQueueEngine() {
	queue := make(chan struct{}, 10000)
	extraQueue := make(chan struct{}, 10000)
	ctx, cancel := context.WithCancel(context.Background())
	targetNumEvents := uint64(1000)
	engine := &ChannelQueueEngine{
		ctx:             ctx,
		cancel:          cancel,
		queue:           queue,
		extraQueue:      extraQueue,
		targetNumEvents: targetNumEvents,
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
			queue <- struct{}{}
		}
	}()
	//go func() {
	//	for i := uint64(0); i <= targetNumEvents; i++ {
	//		extraQueue <- struct{}{}
	//	}
	//}()
	wg.Wait()
	//duration := time.Since(start)
	// Nanoseconds as int64
	//fmt.Println(duration.Nanoseconds())
}

func BenchmarkChannelQueueEngineNotifier(b *testing.B) {
	for i := 0; i < b.N; i++ {
		channelQueueEngine()
	}
}
