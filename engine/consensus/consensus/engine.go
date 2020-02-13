// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// Engine is the consensus engine, responsible for handling communication for
// the embedded consensus algorithm.
type Engine struct {
	unit     *engine.Unit   // used to control startup/shutdown
	log      zerolog.Logger // used to log relevant actions with context
	me       module.Local
	state    protocol.State
	payloads storage.Payloads
	hotstuff hotstuff.HotStuff
}

// New creates a new consensus propagation engine.
func New(log zerolog.Logger, net module.Network, me module.Local, state protocol.State, payloads storage.Payloads, hotstuff hotstuff.HotStuff) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:     engine.NewUnit(),
		log:      log.With().Str("engine", "consensus").Logger(),
		me:       me,
		state:    state,
		payloads: payloads,
		hotstuff: hotstuff,
	}

	// register the engine with the network layer and store the conduit
	_, err := net.Register(engine.ProtocolConsensus, e)
	if err != nil {
		return nil, errors.Wrap(err, "could not register engine")
	}

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the consensus, we consider it ready right away.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
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

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch entity := event.(type) {
	case *messages.BlockProposal:
		return e.onBlockProposal(originID, entity)
	case *messages.BlockVote:
		return e.onBlockVote(originID, entity)
	default:
		return errors.Errorf("invalid event type (%T)", event)
	}
}

// onBlockProposal handles incoming block proposals.
func (e *Engine) onBlockProposal(originID flow.Identifier, block *messages.BlockProposal) error {

	// check if the payload in itself is consistent with the header payload hash
	if block.Payload.Hash() != block.Header.PayloadHash {
		return fmt.Errorf("block proposal has invalid payload hash")
	}

	// store all of the block contents
	err := e.payloads.Store(block.Payload)
	if err != nil {
		return fmt.Errorf("could not store block payload: %w", err)
	}

	// see if the block is a valid extension of the protocol state
	err = e.state.Mutate().Extend(block.Header.ID())
	if err != nil {
		return fmt.Errorf("could not extend protocol state: %w", err)
	}

	e.hotstuff.SubmitProposal(block.Header)

	return nil
}

// onBlockVote handles incoming block votes.
func (e *Engine) onBlockVote(originID flow.Identifier, vote *messages.BlockVote) error {

	// NOTE: The signer index here is not necessary, as we always know the origin
	// of any message on the network, including votes, so we already can look up
	// the public key upon reception. However, as we decided to work with signer
	// indices inside of hotstuff, we now have to send it along, because the
	// mapping between index and node / public key might not be known before the
	// block is received.
	sig := types.SingleSignature{
		Raw:      vote.Signature,
		SignerID: vote.Signer,
	}

	// NOTE: The view number is not really be needed, as the hotstuff algorithm
	// is able to look up the view number by block ID at the moment it processes
	// the vote. No vote can be processed without having the related proposal
	// with its included view number anyway.
	hsVote := types.Vote{
		View:      vote.View,
		BlockID:   vote.BlockID,
		Signature: &sig,
	}

	// ideally, this call could just be: blockID, voterID, signature
	e.hotstuff.SubmitVote(&hsVote)

	return nil
}
