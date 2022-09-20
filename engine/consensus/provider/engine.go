// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package provider

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
)

// Engine represents the provider engine, used to spread block proposals across
// the flow system, to non-consensus nodes. It makes sense to use a separate
// engine to isolate the consensus algorithm from other processes, which allows
// to create a different underlying protocol for consensus nodes, which have a
// higher priority to receive block proposals, and other nodes
type Engine struct {
	unit    *engine.Unit         // used for concurrency & shutdown
	log     zerolog.Logger       // used to log relevant actions with context
	message module.EngineMetrics // used to track sent & received messages
	tracer  module.Tracer
	con     network.Conduit // used to talk to other nodes on the network
	state   protocol.State  // used to access the  protocol state
	me      module.Local    // used to access local node information
}

var _ network.MessageProcessor = (*Engine)(nil)
var _ consensus.ProposalProvider = (*Engine)(nil)

// New creates a new block provider engine.
func New(
	log zerolog.Logger,
	message module.EngineMetrics,
	tracer module.Tracer,
	net network.Network,
	state protocol.State,
	me module.Local,
) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:    engine.NewUnit(),
		log:     log.With().Str("engine", "provider").Logger(),
		message: message,
		tracer:  tracer,
		state:   state,
		me:      me,
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(channels.PushBlocks, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the provider engine, we consider the engine up and running
// upon initialization.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the ingestion engine, it only waits for all submit goroutines to end.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// ProvideProposal asynchronously submits our proposal to all non-consensus nodes.
func (e *Engine) ProvideProposal(proposal *messages.BlockProposal) {
	e.unit.Launch(func() {
		err := e.broadcastProposal(proposal)
		if err != nil {
			// TODO: once error handling in broadcastProposal is updated, this can ctx.Throw instead of logging
			e.log.Err(err).Msg("unexpected error while broadcasting proposal")
		}
	})
}

// Process logs a warning for all received messages, as this engine's channel
// is send-only - no inbound messages are expected.
func (e *Engine) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	e.log.Warn().
		Str("channel", channel.String()).
		Str("origin_id", originID.String()).
		Msgf("received unexpected message (%T), dropping...", event)
	return nil
}

// broadcastProposal is used when we want to broadcast a local block to the rest  of the
// network (non-consensus nodes). We broadcast to consensus nodes in compliance engine.
// TODO error handling - differentiate expected errors in Snapshot
func (e *Engine) broadcastProposal(proposal *messages.BlockProposal) error {
	// sanity check: we should only broadcast proposals that we proposed here
	if e.me.NodeID() != proposal.Header.ProposerID {
		return fmt.Errorf("sanity check failed: attempted to provide another node's proposal (%x!=%x)", e.me.NodeID(), proposal.Header.ProposerID)
	}

	blockID := proposal.Header.ID()
	log := e.log.With().
		Uint64("block_view", proposal.Header.View).
		Hex("block_id", blockID[:]).
		Hex("parent_id", proposal.Header.ParentID[:]).
		Logger()
	log.Info().Msg("block proposal submitted for propagation")

	// broadcast to unejected non-consensus nodes
	recipients, err := e.state.AtBlockID(blockID).Identities(filter.And(
		filter.Not(filter.Ejected),
		filter.Not(filter.HasRole(flow.RoleConsensus)),
	))
	if err != nil {
		return fmt.Errorf("could not get recipients: %w", err)
	}

	// submit the block to the targets
	err = e.con.Publish(proposal, recipients.NodeIDs()...)
	if err != nil {
		e.log.Err(err).Msg("failed to broadcast block")
		return nil
	}
	e.message.MessageSent(metrics.EngineConsensusProvider, metrics.MessageBlockProposal)
	log.Info().Msg("block proposal propagated to non-consensus nodes")

	return nil
}
