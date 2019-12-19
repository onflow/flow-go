// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package generator

import (
	"math/rand"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Engine is a process for generating random collections.
type Engine struct {
	log      zerolog.Logger
	tar      Target
	interval time.Duration
	wg       *sync.WaitGroup
	stop     chan struct{}
}

// New creates a new generator engine.
func New(log zerolog.Logger, tar Target) (*Engine, error) {
	e := &Engine{
		log:      log,
		tar:      tar,
		interval: time.Second,
		wg:       &sync.WaitGroup{},
		stop:     make(chan struct{}),
	}
	return e, nil
}

// Ready will start up engine processes and return a channel that is closed when
// the engine is up and running.
func (e *Engine) Ready() <-chan struct{} {
	ready := make(chan struct{})
	e.wg.Add(1)
	go e.generate()
	go func() {
		close(ready)
	}()
	return ready
}

// Done will shut down engine processes and return a channel that is closed when
// the engine has finished cleanup.
func (e *Engine) Done() <-chan struct{} {
	done := make(chan struct{})
	close(e.stop)
	go func() {
		e.wg.Wait()
		close(done)
	}()
	return done
}

// generate will generate a new collection at regular intervals.
func (e *Engine) generate() {

	// defer the waitgroup signal to unblock shutdown
	defer e.wg.Done()

	// we randomize interval from 50% to 150%
	min := uint64(e.interval) * 5 / 10
	max := uint64(e.interval) * 15 / 10

	// first sleep is between zero and possible range between min and max
	dur := (rand.Uint64() % (max - min))

GenerateLoop:
	for {
		select {
		case <-time.After(time.Duration(dur)):

			// generate a guaranteed collection with a random hash
			hash := make([]byte, 32)
			_, _ = rand.Read(hash)
			coll := &flow.GuaranteedCollection{
				CollectionHash: hash,
			}

			e.log.Info().
				Hex("collection_hash", hash).
				Msg("generated guaranteed collection")

			// submit to the engine
			e.tar.Submit(coll)

			// wait from min to max time again
			dur = (rand.Uint64() % (max - min)) + min

		case <-e.stop:
			break GenerateLoop
		}
	}
}
