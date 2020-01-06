package blocks

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/storage"
)

// An Engine receives and saves incoming blocks.
type Engine struct {
	unit        *engine.Unit
	log         zerolog.Logger
	conduit     network.Conduit
	me          module.Local
	blocks      storage.Blocks
	collections storage.Collections
}

func New(logger zerolog.Logger, net module.Network, me module.Local, blocks storage.Blocks, collections storage.Collections) (*Engine, error) {
	eng := Engine{
		unit:        engine.NewUnit(),
		log:         logger,
		me:          me,
		blocks:      blocks,
		collections: collections,
	}

	con, err := net.Register(engine.ExecutionBlockIngestion, &eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register engine")
	}

	eng.conduit = con

	return &eng, nil
}

func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(originID, event)
		if err != nil {
			e.log.Error().Err(err).Msg("could not process submitted event")
		}
	})
}

func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// Ready returns a channel that will close when the engine has
// successfully started.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a channel that will close when the engine has
// successfully stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

func (e *Engine) Process(originID flow.Identifier, event interface{}) error {

	return e.unit.Do(func() error {
		var err error
		switch v := event.(type) {
		case flow.Block:
			err = e.handleBlock(v)
		case flow.Collection:
			err = e.handleCollection(v)
		default:
			err = errors.Errorf("invalid event type (%T)", event)
		}
		if err != nil {
			return errors.Wrap(err, "could not process event")
		}
		return nil
	})
}

func (e *Engine) handleBlock(block flow.Block) error {
	e.log.Debug().
		Hex("block_hash", block.Hash()).
		Uint64("block_number", block.Number).
		Msg("received block")

	err := e.blocks.Save(&block)

	if err != nil {
		return fmt.Errorf("could not save block: %w", err)
	}
	return nil
}

func (e *Engine) handleCollection(collection flow.Collection) error {
	e.log.Debug().
		Hex("collection_hash", collection.Fingerprint()).
		Msg("received collection")

	err := e.collections.Save(&collection)
	if err != nil {
		return fmt.Errorf("could not save collection: %w", err)
	}
	return nil
}
