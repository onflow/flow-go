package blocks

import (
	"sync"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/storage"
)

// Engine manages execution of transactions
type Engine struct {
	log     zerolog.Logger
	conduit network.Conduit
	me      module.Local
	wg      *sync.WaitGroup
	blocks  storage.Blocks
}

func New(logger zerolog.Logger, net module.Network, me module.Local, blocks storage.Blocks) (*Engine, error) {

	eng := Engine{
		log:    logger,
		me:     me,
		wg:     &sync.WaitGroup{},
		blocks: blocks,
	}

	con, err := net.Register(engine.Execution, &eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register engine")
	}

	eng.conduit = con

	return &eng, nil
}

// Ready returns a channel that will close when the engine has
// successfully started.
func (e *Engine) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		close(ready)
	}()
	return ready
}

// Done returns a channel that will close when the engine has
// successfully stopped.
func (e *Engine) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()
	return done
}

func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	var err error
	switch v := event.(type) {
	case flow.Block:
		err = e.handleBlock(v)
	default:
		err = errors.Errorf("invalid event type (%T)", event)
	}
	if err != nil {
		return errors.Wrap(err, "could not process event")
	}
	return nil
}

func (e *Engine) handleBlock(block flow.Block) error {
	e.log.Debug().
		Hex("block_hash", block.Hash()).
		Uint64("block_number", block.Number).
		Msg("received block")

	return e.blocks.Save(&block)
}
