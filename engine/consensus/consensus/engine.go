// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/coldstuff"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// Engine is the consensus engine, responsible for handling communication for
// the embedded consensus algorithm.
type Engine struct {
	unit      *engine.Unit   // used to control startup/shutdown
	log       zerolog.Logger // used to log relevant actions with context
	me        module.Local
	state     protocol.State
	headers   storage.Headers
	payloads  storage.Payloads
	coldstuff coldstuff.ColdStuff
	con       network.Conduit
}

// New creates a new consensus propagation engine.
func New(log zerolog.Logger, net module.Network, me module.Local, state protocol.State, headers storage.Headers, payloads storage.Payloads, coldstuff coldstuff.ColdStuff) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:      engine.NewUnit(),
		log:       log.With().Str("engine", "consensus").Logger(),
		me:        me,
		state:     state,
		headers:   headers,
		payloads:  payloads,
		coldstuff: coldstuff,
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
func (e *Engine) SendVote(vote *messages.BlockVote, recipientID flow.Identifier) error {

	// send the vote the desired recipient
	err := e.con.Submit(vote, recipientID)
	if err != nil {
		return fmt.Errorf("could not send vote: %w", err)
	}

	return nil
}

// BroadcastProposal will propagate a block proposal to all non-local consensus nodes.
func (e *Engine) BroadcastProposal(header *flow.Header) error {

	// retrieve the payload for the block
	payload, err := e.payloads.ByPayloadHash(header.PayloadHash)
	if err != nil {
		return fmt.Errorf("could not retrieve payload for proposal: %w", err)
	}

	// retrieve all consensus nodes without our ID
	recipients, err := e.state.AtBlockID(header.ParentID).Identities(
		filter.HasRole(flow.RoleConsensus),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	)
	if err != nil {
		return fmt.Errorf("could not get consensus recipients: %w", err)
	}

	// NOTE: some fields are not needed for the message
	// - proposer ID is conveyed over the network message
	// - the payload hash is deduced from the payload
	msg := messages.BlockProposal{
		Header:  header,
		Payload: payload,
	}

	// broadcast the proposal to consensus nodes
	err = e.con.Submit(&msg, recipients.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not send proposal message: %w", err)
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
func (e *Engine) onBlockProposal(originID flow.Identifier, msg *messages.BlockProposal) error {

	// retrieve the parent for the block for parent view
	// TODO: instead of erroring when missing, simply cache
	parent, err := e.headers.ByBlockID(msg.Header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	// store all of the block contents
	err = e.payloads.Store(msg.Payload)
	if err != nil {
		return fmt.Errorf("could not store block payload: %w", err)
	}

	// insert the header into the database
	err = e.headers.Store(msg.Header)
	if err != nil {
		return fmt.Errorf("could not store header: %w", err)
	}

	// see if the block is a valid extension of the protocol state
	err = e.state.Mutate().Extend(msg.Header.ID())
	if err != nil {
		return fmt.Errorf("could not extend protocol state: %w", err)
	}

	// submit the model to hotstuff for processing
	e.coldstuff.SubmitProposal(msg.Header, parent.View)

	return nil
}

// onBlockVote handles incoming block votes.
func (e *Engine) onBlockVote(originID flow.Identifier, msg *messages.BlockVote) error {

	// forward the vote to hotstuff for processing
	e.coldstuff.SubmitVote(originID, msg.BlockID, msg.View, msg.Signature)

	return nil
}
