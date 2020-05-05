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
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// Engine implements a simulated consensus algorithm. It's similar to a
// one-chain BFT consensus algorithm, finalizing blocks immediately upon
// collecting the first quorum. In order to keep nodes in sync, the quorum is
// set at the totality of the stake in the network.
type Engine struct {
	unit      *engine.Unit
	log       zerolog.Logger
	prov      network.Engine
	headers   storage.Headers
	payloads  storage.Payloads
	state     protocol.State
	me        module.Local
	builder   module.Builder
	finalizer module.Finalizer
	interval  time.Duration
}

// New initializes a new coldstuff consensus engine, using the injected network
// and the injected memory pool to forward the injected protocol state.
func New(log zerolog.Logger, prov network.Engine, headers storage.Headers, payloads storage.Payloads, state protocol.State, me module.Local, builder module.Builder, finalizer module.Finalizer) (*Engine, error) {

	// initialize the engine with dependencies
	e := &Engine{
		unit:      engine.NewUnit(),
		log:       log.With().Str("engine", "subzero").Logger(),
		prov:      prov,
		headers:   headers,
		payloads:  payloads,
		state:     state,
		me:        me,
		builder:   builder,
		finalizer: finalizer,
		interval:  10 * time.Second,
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

			proposal, err := e.createProposal()
			if err != nil {
				log.Error().Err(err).Msg("could not create proposal")
				continue ConsentLoop
			}

			proposalID := proposal.ID()

			log = log.With().
				Uint64("proposal_number", proposal.View).
				Hex("proposal_id", proposalID[:]).
				Logger()

			log.Info().Msg("proposal created")

			err = e.commitProposal(proposal)
			if err != nil {
				e.log.Error().Err(err).Msg("could not commit proposal")
				continue ConsentLoop
			}

			log.Info().Msg("proposal committed")

			payload, err := e.payloads.ByBlockID(proposalID)
			if err != nil {
				e.log.Error().Err(err).Msg("could retrieve payload hash")
				continue ConsentLoop
			}
			block := flow.Block{
				Header:  proposal,
				Payload: payload,
			}

			// Broadcast block
			e.prov.SubmitLocal(&block)
		}
	}
}

// createProposal will create a header. It will be signed by the one consensus node,
// which will stand in for all of the consensus nodes finding consensus on the
// block.
func (e *Engine) createProposal() (*flow.Header, error) {

	// get the latest finalized block to build on
	parent, err := e.state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get parent: %w", err)
	}

	// define the block header build function
	setProposer := func(header *flow.Header) error {
		header.ProposerID = e.me.NodeID()
		return nil
	}

	// create the proposal payload on top of the parent
	proposal, err := e.builder.BuildOn(parent.ID(), setProposer)
	if err != nil {
		return nil, fmt.Errorf("could not create block: %w", err)
	}

	return proposal, nil
}

// commitProposal will insert the block into our local block database, then it
// will extend the current blockchain state with it and finalize it.
func (e *Engine) commitProposal(proposal *flow.Header) error {

	// extend our blockchain state with the proposal
	err := e.state.Mutate().Extend(proposal.ID())
	if err != nil {
		return fmt.Errorf("could not extend state: %w", err)
	}

	// finalize the blockchain state with the proposal
	err = e.finalizer.MakeFinal(proposal.ID())
	if err != nil {
		return fmt.Errorf("could not finalize state: %w", err)
	}

	return nil
}
