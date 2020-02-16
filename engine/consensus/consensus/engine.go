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
	"github.com/dapperlabs/flow-go/network"
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
	con      network.Conduit
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
	con, err := net.Register(engine.ProtocolConsensus, e)
	if err != nil {
		return nil, errors.Wrap(err, "could not register engine")
	}
	e.con = con

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

// SendVote will send a vote to the desired node.
func (e *Engine) SendVote(vote *types.Vote, recipientID flow.Identifier) error {

	// check that we are sending our own vote
	if vote.Signature.SignerID != e.me.NodeID() {
		return fmt.Errorf("can only send votes signed locally")
	}

	// NOTE: the signer ID will be conveyed through the message origin ID;
	// we can thus leave it out of the message to conserve bandwidth

	// NOTE: the view is redundant because we can deduce it from the block ID
	// on the receiver end in the CCL; however, we can optimize processing by
	// including it, so that we can check the signature before having the block

	// convert the vote to a vote message
	msg := messages.BlockVote{
		View:      vote.View,
		Signature: vote.Signature.Raw,
	}

	// send the vote the desired recipient
	err := e.con.Submit(&msg, recipientID)
	if err != nil {
		return fmt.Errorf("could not send vote: %w", err)
	}

	return nil
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
func (e *Engine) onBlockVote(originID flow.Identifier, msg *messages.BlockVote) error {

	// rebuild the signature using the origin ID and the transmitted signature
	sig := types.SingleSignature{
		Raw:      msg.Signature,
		SignerID: originID,
	}

	// rebuild the vote in the type prefered by hotstuff
	// NOTE: the view number is somewhat redundant, as we can locally check the view
	// of the referenced block ID; however, we might not have the block yet, so sending
	// the view along will allow us to validate the signature immediately and cache votes
	// appropriately within hotstuff
	vote := types.Vote{
		BlockID:   msg.BlockID,
		View:      msg.View,
		Signature: &sig,
	}

	// forward the vote to hotstuff for processing
	e.hotstuff.SubmitVote(&vote)

	return nil
}
