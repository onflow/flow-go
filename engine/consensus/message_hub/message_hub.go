package message_hub

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// defaultMessageHubRequestsWorkers number of workers to dispatch events for requests
const defaultMessageHubRequestsWorkers = 5

// defaultProposalQueueCapacity number of pending outgoing proposals stored in queue
const defaultProposalQueueCapacity = 3

// defaultVoteQueueCapacity number of pending outgoing votes stored in queue
const defaultVoteQueueCapacity = 20

// defaultTimeoutQueueCapacity number of pending outgoing timeouts stored in queue
const defaultTimeoutQueueCapacity = 3

// packedVote is a helper structure to pack recipientID and vote into one structure to pass through fifoqueue.FifoQueue
type packedVote struct {
	recipientID flow.Identifier
	vote        *messages.BlockVote
}

// MessageHub is a central module for handling incoming and outgoing messages via consensus channel.
// It performs message routing for incoming messages by matching them by type and sending to respective engine.
// For incoming messages handling processing looks like this:
//
//	   +-------------------+      +------------+
//	-->| Consensus-Channel |----->| MessageHub |
//	   +-------------------+      +------+-----+
//	                         ------------|------------
//	   +------+---------+    |    +------+-----+     |    +------+------------+
//	   | VoteAggregator |----+    | Compliance |     +----| TimeoutAggregator |
//	   +----------------+         +------------+          +------+------------+
//	          vote                     block                  timeout object
//
// MessageHub acts as communicator and handles hotstuff.Consumer communication events to send votes, broadcast timeouts
// and proposals. It is responsible for communication between consensus participants.
// It implements hotstuff.Consumer interface and needs to be subscribed for notifications via pub/sub.
// All communicator events are handled on worker thread to prevent sender from blocking.
// For outgoing messages processing logic looks like this:
//
//	+-------------------+      +------------+      +----------+      +------------------------+
//	| Consensus-Channel |<-----| MessageHub |<-----| Consumer |<-----|        Hotstuff        |
//	+-------------------+      +------+-----+      +----------+      +------------------------+
//	                                                  pub/sub          vote, timeout, proposal
//
// MessageHub is safe to use in concurrent environment.
type MessageHub struct {
	*component.ComponentManager
	notifications.NoopConsumer
	log                        zerolog.Logger
	me                         module.Local
	engineMetrics              module.EngineMetrics
	state                      protocol.State
	payloads                   storage.Payloads
	con                        network.Conduit
	pushBlocksCon              network.Conduit
	ownOutboundMessageNotifier engine.Notifier
	ownOutboundVotes           *fifoqueue.FifoQueue // queue for handling outgoing vote transmissions
	ownOutboundProposals       *fifoqueue.FifoQueue // queue for handling outgoing proposal transmissions
	ownOutboundTimeouts        *fifoqueue.FifoQueue // queue for handling outgoing timeout transmissions

	// injected dependencies
	compliance        consensus.Compliance       // handler of incoming block proposals
	hotstuff          module.HotStuff            // used to submit proposals that were previously broadcast
	voteAggregator    hotstuff.VoteAggregator    // handler of incoming votes
	timeoutAggregator hotstuff.TimeoutAggregator // handler of incoming timeouts
}

var _ network.MessageProcessor = (*MessageHub)(nil)
var _ hotstuff.CommunicatorConsumer = (*MessageHub)(nil)

// NewMessageHub constructs new instance of message hub
// No errors are expected during normal operations.
func NewMessageHub(log zerolog.Logger,
	engineMetrics module.EngineMetrics,
	net network.EngineRegistry,
	me module.Local,
	compliance consensus.Compliance,
	hotstuff module.HotStuff,
	voteAggregator hotstuff.VoteAggregator,
	timeoutAggregator hotstuff.TimeoutAggregator,
	state protocol.State,
	payloads storage.Payloads,
) (*MessageHub, error) {
	ownOutboundVotes, err := fifoqueue.NewFifoQueue(defaultVoteQueueCapacity)
	if err != nil {
		return nil, fmt.Errorf("could not initialize votes queue")
	}
	ownOutboundProposals, err := fifoqueue.NewFifoQueue(defaultProposalQueueCapacity)
	if err != nil {
		return nil, fmt.Errorf("could not initialize blocks queue")
	}
	ownOutboundTimeouts, err := fifoqueue.NewFifoQueue(defaultTimeoutQueueCapacity)
	if err != nil {
		return nil, fmt.Errorf("could not initialize timeouts queue")
	}
	hub := &MessageHub{
		log:                        log.With().Str("engine", "message_hub").Logger(),
		me:                         me,
		engineMetrics:              engineMetrics,
		state:                      state,
		payloads:                   payloads,
		compliance:                 compliance,
		hotstuff:                   hotstuff,
		voteAggregator:             voteAggregator,
		timeoutAggregator:          timeoutAggregator,
		ownOutboundMessageNotifier: engine.NewNotifier(),
		ownOutboundVotes:           ownOutboundVotes,
		ownOutboundProposals:       ownOutboundProposals,
		ownOutboundTimeouts:        ownOutboundTimeouts,
	}

	// register with the network layer and store the conduit
	hub.con, err = net.Register(channels.ConsensusCommittee, hub)
	if err != nil {
		return nil, fmt.Errorf("could not register core: %w", err)
	}

	// register with the network layer and store the conduit
	hub.pushBlocksCon, err = net.Register(channels.PushBlocks, hub)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

	componentBuilder := component.NewComponentManagerBuilder()
	// This implementation tolerates if the networking layer sometimes blocks on send requests.
	// We use by default 5 go-routines here. This is fine, because outbound messages are temporally sparse
	// under normal operations. Hence, the go-routines should mostly be asleep waiting for work.
	for i := 0; i < defaultMessageHubRequestsWorkers; i++ {
		componentBuilder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			hub.queuedMessagesProcessingLoop(ctx)
		})
	}
	hub.ComponentManager = componentBuilder.Build()
	return hub, nil
}

// queuedMessagesProcessingLoop orchestrates dispatching of previously queued messages
func (h *MessageHub) queuedMessagesProcessingLoop(ctx irrecoverable.SignalerContext) {
	notifier := h.ownOutboundMessageNotifier.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-notifier:
			err := h.sendOwnMessages(ctx)
			if err != nil {
				ctx.Throw(fmt.Errorf("internal error processing queued messages: %w", err))
				return
			}
		}
	}
}

// sendOwnMessages is a function which dispatches previously queued messages on worker thread
// This function is called whenever we have queued messages ready to be dispatched.
// No errors are expected during normal operations.
func (h *MessageHub) sendOwnMessages(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, ok := h.ownOutboundProposals.Pop()
		if ok {
			block := msg.(*flow.Header)
			err := h.sendOwnProposal(block)
			if err != nil {
				return fmt.Errorf("could not process queued block %v: %w", block.ID(), err)
			}
			continue
		}

		msg, ok = h.ownOutboundVotes.Pop()
		if ok {
			packed := msg.(*packedVote)
			err := h.sendOwnVote(packed)
			if err != nil {
				return fmt.Errorf("could not process queued vote: %w", err)
			}
			continue
		}

		msg, ok = h.ownOutboundTimeouts.Pop()
		if ok {
			err := h.sendOwnTimeout(msg.(*model.TimeoutObject))
			if err != nil {
				return fmt.Errorf("coult not process queued timeout: %w", err)
			}
			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

// sendOwnTimeout propagates the timeout to the consensus committee (excluding myself)
// No errors are expected during normal operations.
func (h *MessageHub) sendOwnTimeout(timeout *model.TimeoutObject) error {
	log := timeout.LogContext(h.log).Logger()
	log.Info().Msg("processing timeout broadcast request from hotstuff")

	// Retrieve all consensus nodes (excluding myself).
	// CAUTION: We must include also nodes with weight zero, because otherwise
	//          TCs might not be constructed at epoch switchover.
	recipients, err := h.state.Final().Identities(filter.And(
		filter.Not(filter.HasParticipationStatus(flow.EpochParticipationStatusEjected)),
		filter.HasRole[flow.Identity](flow.RoleConsensus),
		filter.Not(filter.HasNodeID[flow.Identity](h.me.NodeID())),
	))
	if err != nil {
		return fmt.Errorf("could not get consensus recipients for broadcasting timeout: %w", err)
	}

	// create the timeout message
	msg := &messages.TimeoutObject{
		View:        timeout.View,
		NewestQC:    timeout.NewestQC,
		LastViewTC:  timeout.LastViewTC,
		SigData:     timeout.SigData,
		TimeoutTick: timeout.TimeoutTick,
	}
	err = h.con.Publish(msg, recipients.NodeIDs()...)
	if err != nil {
		if !errors.Is(err, network.EmptyTargetList) {
			log.Err(err).Msg("could not broadcast timeout")
		}
		return nil
	}
	log.Info().Msg("consensus timeout was broadcast")
	h.engineMetrics.MessageSent(metrics.EngineConsensusMessageHub, metrics.MessageTimeoutObject)

	return nil
}

// sendOwnVote propagates the vote via unicast to another node that is the next leader
// No errors are expected during normal operations.
func (h *MessageHub) sendOwnVote(packed *packedVote) error {
	log := h.log.With().
		Hex("block_id", packed.vote.BlockID[:]).
		Uint64("block_view", packed.vote.View).
		Hex("recipient_id", packed.recipientID[:]).
		Logger()
	log.Info().Msg("processing vote transmission request from hotstuff")

	// send the vote the desired recipient
	err := h.con.Unicast(packed.vote, packed.recipientID)
	if err != nil {
		log.Err(err).Msg("could not send vote")
		return nil
	}
	h.engineMetrics.MessageSent(metrics.EngineConsensusMessageHub, metrics.MessageBlockVote)
	log.Info().Msg("block vote transmitted")

	return nil
}

// sendOwnProposal propagates the block proposal to the consensus committee and submits to non-consensus network:
//   - broadcast to all other consensus participants (excluding myself)
//   - broadcast to all non-consensus participants
//
// No errors are expected during normal operations.
func (h *MessageHub) sendOwnProposal(header *flow.Header) error {
	// first, check that we are the proposer of the block
	if header.ProposerID != h.me.NodeID() {
		return fmt.Errorf("cannot broadcast proposal with non-local proposer (%x)", header.ProposerID)
	}

	// retrieve the payload for the block
	payload, err := h.payloads.ByBlockID(header.ID())
	if err != nil {
		return fmt.Errorf("could not retrieve payload for proposal: %w", err)
	}

	log := h.log.With().
		Str("chain_id", header.ChainID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", logging.Entity(header)).
		Hex("parent_id", header.ParentID[:]).
		Hex("payload_hash", header.PayloadHash[:]).
		Int("guarantees_count", len(payload.Guarantees)).
		Int("seals_count", len(payload.Seals)).
		Int("receipts_count", len(payload.Receipts)).
		Time("timestamp", header.Timestamp).
		Hex("signers", header.ParentVoterIndices).
		//Dur("delay", delay).
		Logger()

	log.Debug().Msg("processing proposal broadcast request from hotstuff")

	// Retrieve all consensus nodes (excluding myself).
	// CAUTION: We must also include nodes that are joining, because otherwise new consensus
	//          nodes for the next epoch are left out. As most nodes might be interested in
	//          new proposals, we simply broadcast to all non-ejected nodes (excluding myself).
	// Note: retrieving the final state requires a time-intensive database read.
	//       Therefore, we execute this in a separate routine, because
	//       `OnOwnTimeout` is directly called by the consensus core logic.
	allIdentities, err := h.state.AtBlockID(header.ParentID).Identities(filter.And(
		filter.Not(filter.HasParticipationStatus(flow.EpochParticipationStatusEjected)),
		filter.Not(filter.HasNodeID[flow.Identity](h.me.NodeID())),
	))
	if err != nil {
		return fmt.Errorf("could not get identities for broadcasting proposal: %w", err)
	}

	consRecipients := allIdentities.Filter(filter.HasRole[flow.Identity](flow.RoleConsensus))

	// NOTE: some fields are not needed for the message
	// - proposer ID is conveyed over the network message
	// - the payload hash is deduced from the payload
	proposal := messages.NewBlockProposal(&flow.Block{
		Header:  header,
		Payload: payload,
	})

	// broadcast the proposal to consensus nodes
	err = h.con.Publish(proposal, consRecipients.NodeIDs()...)
	if err != nil {
		if !errors.Is(err, network.EmptyTargetList) {
			log.Err(err).Msg("could not send proposal message")
		}
		return nil
	}
	log.Info().Msg("block proposal was broadcast")

	// submit proposal to non-consensus nodes
	h.provideProposal(proposal, allIdentities.Filter(filter.Not(filter.HasRole[flow.Identity](flow.RoleConsensus))))
	h.engineMetrics.MessageSent(metrics.EngineConsensusMessageHub, metrics.MessageBlockProposal)

	return nil
}

// provideProposal is used when we want to broadcast a local block to the rest  of the
// network (non-consensus nodes).
func (h *MessageHub) provideProposal(proposal *messages.BlockProposal, recipients flow.IdentityList) {
	header := proposal.Block.Header
	blockID := header.ID()
	log := h.log.With().
		Uint64("block_view", header.View).
		Hex("block_id", blockID[:]).
		Hex("parent_id", header.ParentID[:]).
		Logger()
	log.Info().Msg("block proposal submitted for propagation")

	// submit the block to the targets
	err := h.pushBlocksCon.Publish(proposal, recipients.NodeIDs()...)
	if err != nil {
		h.log.Err(err).Msg("failed to broadcast block")
		return
	}

	log.Info().Msg("block proposal propagated to non-consensus nodes")
}

// OnOwnVote propagates the vote to relevant recipient(s):
//   - [common case] vote is queued and is sent via unicast to another node that is the next leader by worker
//   - [special case] this node is the next leader: vote is directly forwarded to the node's internal `VoteAggregator`
func (h *MessageHub) OnOwnVote(blockID flow.Identifier, view uint64, sigData []byte, recipientID flow.Identifier) {
	vote := &messages.BlockVote{
		BlockID: blockID,
		View:    view,
		SigData: sigData,
	}

	// special case: I am the next leader
	if recipientID == h.me.NodeID() {
		h.forwardToOwnVoteAggregator(vote, h.me.NodeID()) // forward vote to my own `voteAggregator`
		return
	}

	// common case: someone else is leader
	packed := &packedVote{
		recipientID: recipientID,
		vote:        vote,
	}
	if ok := h.ownOutboundVotes.Push(packed); ok {
		h.ownOutboundMessageNotifier.Notify()
	} else {
		h.engineMetrics.OutboundMessageDropped(metrics.EngineConsensusMessageHub, metrics.MessageBlockVote)
	}
}

// OnOwnTimeout forwards timeout to node's internal `timeoutAggregator` and queues timeout for
// subsequent propagation to all consensus participants (excluding this node)
func (h *MessageHub) OnOwnTimeout(timeout *model.TimeoutObject) {
	h.forwardToOwnTimeoutAggregator(timeout) // forward timeout to my own `timeoutAggregator`
	if ok := h.ownOutboundTimeouts.Push(timeout); ok {
		h.ownOutboundMessageNotifier.Notify()
	} else {
		h.engineMetrics.OutboundMessageDropped(metrics.EngineConsensusMessageHub, metrics.MessageTimeoutObject)
	}
}

// OnOwnProposal directly forwards proposal to HotStuff core logic (skipping compliance engine as we assume our
// own proposals to be correct) and queues proposal for subsequent propagation to all consensus participants (including this node).
// The proposal will only be placed in the queue, after the specified delay (or dropped on shutdown signal).
func (h *MessageHub) OnOwnProposal(proposal *flow.Header, targetPublicationTime time.Time) {
	go func() {
		select {
		case <-time.After(time.Until(targetPublicationTime)):
		case <-h.ShutdownSignal():
			return
		}

		hotstuffProposal := model.ProposalFromFlow(proposal)
		// notify vote aggregator that new block proposal is available, in case we are next leader
		h.voteAggregator.AddBlock(hotstuffProposal) // non-blocking

		// TODO(active-pacemaker): replace with pub/sub?
		// submit proposal to our own processing pipeline
		h.hotstuff.SubmitProposal(hotstuffProposal) // non-blocking

		if ok := h.ownOutboundProposals.Push(proposal); ok {
			h.ownOutboundMessageNotifier.Notify()
		} else {
			h.engineMetrics.OutboundMessageDropped(metrics.EngineConsensusMessageHub, metrics.MessageBlockProposal)
		}
	}()
}

// Process handles incoming messages from consensus channel. After matching message by type, sends it to the correct
// component for handling.
// No errors are expected during normal operations.
func (h *MessageHub) Process(channel channels.Channel, originID flow.Identifier, message interface{}) error {
	switch msg := message.(type) {
	case *messages.BlockProposal:
		h.compliance.OnBlockProposal(flow.Slashable[*messages.BlockProposal]{
			OriginID: originID,
			Message:  msg,
		})
	case *messages.BlockVote:
		h.forwardToOwnVoteAggregator(msg, originID)
	case *messages.TimeoutObject:
		t := &model.TimeoutObject{
			View:        msg.View,
			NewestQC:    msg.NewestQC,
			LastViewTC:  msg.LastViewTC,
			SignerID:    originID,
			SigData:     msg.SigData,
			TimeoutTick: msg.TimeoutTick,
		}
		h.forwardToOwnTimeoutAggregator(t)
	default:
		h.log.Warn().
			Bool(logging.KeySuspicious, true).
			Hex("origin_id", logging.ID(originID)).
			Str("message_type", fmt.Sprintf("%T", message)).
			Str("channel", channel.String()).
			Msgf("delivered unsupported message type")
	}
	return nil
}

// forwardToOwnVoteAggregator converts vote to generic `model.Vote`, logs vote and forwards it to own `voteAggregator`.
// Per API convention, timeoutAggregator` is non-blocking, hence, this call returns quickly.
func (h *MessageHub) forwardToOwnVoteAggregator(vote *messages.BlockVote, originID flow.Identifier) {
	h.engineMetrics.MessageReceived(metrics.EngineConsensusMessageHub, metrics.MessageBlockVote)
	v := &model.Vote{
		View:     vote.View,
		BlockID:  vote.BlockID,
		SignerID: originID,
		SigData:  vote.SigData,
	}
	h.log.Info().
		Uint64("block_view", v.View).
		Hex("block_id", v.BlockID[:]).
		Hex("voter", v.SignerID[:]).
		Str("vote_id", v.ID().String()).
		Msg("block vote received, forwarding block vote to hotstuff vote aggregator")
	h.voteAggregator.AddVote(v)
}

// forwardToOwnTimeoutAggregator logs timeout and forwards it to own `timeoutAggregator`.
// Per API convention, timeoutAggregator` is non-blocking, hence, this call returns quickly.
func (h *MessageHub) forwardToOwnTimeoutAggregator(t *model.TimeoutObject) {
	h.engineMetrics.MessageReceived(metrics.EngineConsensusMessageHub, metrics.MessageTimeoutObject)
	h.log.Info().
		Hex("origin_id", t.SignerID[:]).
		Uint64("view", t.View).
		Str("timeout_id", t.ID().String()).
		Msg("timeout received, forwarding timeout to hotstuff timeout aggregator")
	h.timeoutAggregator.AddTimeout(t)
}
