// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package subzero

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// Engine implements a simulated consensus algorithm. It's similar to a
// one-chain BFT consensus algorithm, finalizing blocks immediately upon
// collecting the first quorum. In order to keep nodes in sync, the quorum is
// set at the totality of the stake in the network.
type Engine struct {
	unit     *engine.Unit
	log      zerolog.Logger
	prov     network.Engine
	blocks   storage.Blocks
	state    protocol.State
	me       module.Local
	pool     mempool.Guarantees
	interval time.Duration
}

// New initializes a new coldstuff consensus engine, using the injected network
// and the injected memory pool to forward the injected protocol state.
func New(log zerolog.Logger, prov network.Engine, blocks storage.Blocks, state protocol.State, me module.Local, pool mempool.Guarantees) (*Engine, error) {

	// initialize the engine with dependencies
	e := &Engine{
		unit:     engine.NewUnit(),
		log:      log.With().Str("engine", "subzero").Logger(),
		prov:     prov,
		blocks:   blocks,
		state:    state,
		me:       me,
		pool:     pool,
		interval: 10 * time.Second,
	}

	return e, nil
}

// Ready returns a channel that will close when the coldstuff engine has
// successfully started.
func (e *Engine) Ready() <-chan struct{} {
	e.unit.Launch(e.consent)
	return e.unit.Ready()
}

// Done returns a channel that will close when the coldstuff engine has
// successfully stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(originID, event)
		if err != nil {
			e.log.Error().Err(err).Msg("could not process submitted event")
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

// process processes events for the proposal engine on the collection node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch event.(type) {
	default:
		return errors.Errorf("invalid event type (%T)", event)
	}
}

// consent will start the consensus algorithm on the engine. As we need to
// process events sequentially, all submissions are queued in channels and then
// processed here.
func (e *Engine) consent() {

	// each iteration of the loop represents one (successful or failed) round of
	// the consensus algorithm
ConsentLoop:
	for {
		select {

		// break the loop and shut down
		case <-e.unit.Quit():
			break ConsentLoop

		// start the next consensus round
		case <-time.After(e.interval):

			log := e.log

			block, err := e.createBlock()
			if err != nil {
				log.Error().Err(err).Msg("could not create block")
				continue ConsentLoop
			}

			blockID := block.ID()

			log = log.With().
				Uint64("block_number", block.Number).
				Hex("block_id", blockID[:]).
				Int("num_guarantees", len(block.Guarantees)).
				Logger()

			log.Info().Msg("block created")

			err = e.commitBlock(block)
			if err != nil {
				e.log.Error().Err(err).Msg("could not commit block")
				continue ConsentLoop
			}

			log = log.With().
				Uint("mempool_size", e.pool.Size()).
				Logger()

			log.Info().Msg("block committed")

			e.prov.SubmitLocal(block)
		}
	}
}

// createBlock will create a block. It will be signed by the one consensus node,
// which will stand in for all of the consensus nodes finding consensus on the
// block.
func (e *Engine) createBlock() (*flow.Block, error) {

	// get the latest finalized block to build on
	head, err := e.state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get head: %w", err)
	}

	// get the collection guarantees from the collection pool
	guarantees := e.pool.All()

	// create the block content with the collection guarantees
	payload := flow.Payload{
		Identities: nil,
		Guarantees: guarantees,
	}

	// create the block header from the current head
	header := flow.Header{
		Number:      head.Number + 1,
		ParentID:    head.ID(),
		PayloadHash: payload.Hash(),
		Timestamp:   time.Now().UTC(),
	}

	// create the new block using header, payload & content
	block := flow.Block{
		Header:  header,
		Payload: payload,
	}

	return &block, nil
}

// commitBlock will insert the block into our local block database, then it
// will extend the current blockchain state with it and finalize it.
func (e *Engine) commitBlock(block *flow.Block) error {

	// store the block in our database
	err := e.blocks.Store(block)
	if err != nil {
		return errors.Wrap(err, "could not store block")
	}

	// extend our blockchain state with the block
	err = e.state.Mutate().Extend(block.ID())
	if err != nil {
		return errors.Wrap(err, "could not extend state")
	}

	// remove the block collection guarantees from the memory pool
	for _, guarantee := range block.Guarantees {
		ok := e.pool.Rem(guarantee.ID())
		if !ok {
			return errors.Errorf("guarantee missing from pool (%x)", guarantee.ID())
		}
	}

	// finalize the blockchain state with the block
	err = e.state.Mutate().Finalize(block.ID())
	if err != nil {
		return errors.Wrap(err, "could not finalize state")
	}

	return nil
}
