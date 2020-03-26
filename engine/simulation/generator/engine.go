// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package generator

import (
	"math/rand"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network"
)

// Engine is a process for generating random collections.
type Engine struct {
	unit     *engine.Unit
	log      zerolog.Logger
	target   network.Engine
	interval time.Duration
}

// New creates a new generator engine.
func New(log zerolog.Logger, target network.Engine) (*Engine, error) {
	e := &Engine{
		unit:     engine.NewUnit(),
		log:      log.With().Str("engine", "generator").Logger(),
		target:   target,
		interval: time.Second,
	}
	return e, nil
}

// Ready will start up engine processes and return a channel that is closed when
// the engine is up and running.
func (e *Engine) Ready() <-chan struct{} {
	e.unit.Launch(e.generate)
	return e.unit.Ready()
}

// Done will shut down engine processes and return a channel that is closed when
// the engine has finished cleanup.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// generate will generate a new collection at regular intervals.
func (e *Engine) generate() {

	// we randomize interval from 50% to 150%
	min := uint64(e.interval) * 5 / 10
	max := uint64(e.interval) * 15 / 10

	// first sleep is between zero and possible range between min and max
	dur := (rand.Uint64() % (max - min))

GenerateLoop:
	for {
		select {
		case <-time.After(time.Duration(dur)):

			// generate a collection guarantee with a random hash
			var collID flow.Identifier
			_, _ = rand.Read(collID[:])
			coll := &flow.CollectionGuarantee{
				CollectionID: collID,
			}

			e.log.Info().
				Hex("collection_hash", collID[:]).
				Msg("generated collection guarantee")

			// submit to the engine
			e.target.SubmitLocal(coll)

			// wait from min to max time again
			dur = (rand.Uint64() % (max - min)) + min

		case <-e.unit.Quit():
			break GenerateLoop
		}
	}
}
