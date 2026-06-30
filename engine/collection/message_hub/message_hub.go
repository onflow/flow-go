package message_hub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/collection"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	clusterkv "github.com/onflow/flow-go/state/cluster"
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
	vote        *messages.ClusterBlockVote
}

// MessageHub is a central module for handling incoming and outgoing messages via cluster consensus channel.
// It performs message routing for incoming messages by matching them by type and sending to respective engine.
// For incoming messages handling processing looks like this:
//
//	   +-------------------+      +------------+
//	-->|  Cluster-Channel  |----->| MessageHub |
//	   +-------------------+      +------+-----+
//	                         ------------|------------
//	   +------+---------+    |    +------+-----+     |    +------+------------+
//	   | VoteAggregator |----+    | Compliance |     +----| TimeoutAggregator |
//	   +----------------+         +------------+          +------+------------+
//	          vote                     block                  timeout object
//
// MessageHub acts as communicator and handles hotstuff.Consumer communication events to send votes, broadcast timeouts
// and proposals. It is responsible for communication between cluster consensus participants.
// It implements hotstuff.Consumer interface and needs to be subscribed for notifications via pub/sub.
// All communicator events are handled on worker thread to prevent sender from blocking.
// For outgoing messages processing logic looks like this:
//
//	+-------------------+      +------------+      +----------+      +------------------------+
//	|  Cluster-Channel  |<-----| MessageHub |<-----| Consumer |<-----|        Hotstuff        |
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
	payloads                   storage.ClusterPayloads
	con                        network.Conduit
	ownOutboundMessageNotifier engine.Notifier
	ownOutboundVotes           *fifoqueue.FifoQueue // queue for handling outgoing vote transmissions
	ownOutboundProposals       *fifoqueue.FifoQueue // queue for handling outgoing proposal transmissions
	ownOutboundTimeouts        *fifoqueue.FifoQueue // queue for handling outgoing timeout transmissions
	clusterIdentityFilter      flow.IdentityFilter[flow.Identity]

	// injected dependencies
	compliance        collection.Compliance      // handler of incoming block proposals
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
	compliance collection.Compliance,
	hotstuff module.HotStuff,
	voteAggregator hotstuff.VoteAggregator,
	timeoutAggregator hotstuff.TimeoutAggregator,
	state protocol.State,
	clusterState clusterkv.State,
	payloads storage.ClusterPayloads,
) (*MessageHub, error) {
	// find my cluster for the current epoch
	epoch, err := state.Final().Epochs().Current()
	if err != nil {
		return nil, fmt.Errorf("could not get current epoch: %w", err)
	}
	clusters, err := epoch.Clustering()
	if err != nil {
		return nil, fmt.Errorf("could not get clusters: %w", err)
	}
	currentCluster, _, found := clusters.ByNodeID(me.NodeID())
	if !found {
		return nil, fmt.Errorf("could not find cluster for self")
	}

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
		log:                        log.With().Str("engine", "cluster_message_hub").Logger(),
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
		clusterIdentityFilter: filter.And(
			filter.Adapt(filter.In(currentCluster)),
			filter.Not(filter.HasNodeID[flow.Identity](me.NodeID())),
		),
	}

	// register network conduit
	chainID := clusterState.Params().ChainID()
	conduit, err := net.Register(channels.ConsensusCluster(chainID), hub)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	hub.con = conduit

	var workers sync.WaitGroup
	componentBuilder := component.NewComponentManagerBuilder()
	// This implementation tolerates if the networking layer sometimes blocks on send requests.
	// We use by default 5 go-routines here. This is fine, because outbound messages are temporally sparse
	// under normal operations. Hence, the go-routines should mostly be asleep waiting for work.
	for i := 0; i < defaultMessageHubRequestsWorkers; i++ {
		workers.Add(1)
		componentBuilder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			defer workers.Done()
			ready()
			hub.queuedMessagesProcessingLoop(ctx)
		})
	}
	componentBuilder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		ready()
		// ensure we clean up the network Conduit when shutting down
		workers.Wait()
		// close the network conduit
		err := hub.con.Close()
		if err != nil {
			ctx.Throw(fmt.Errorf("could not close network conduit: %w", err))
		}
	})
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
			proposal := msg.(*flow.ProposalHeader)
			err := h.sendOwnProposal(proposal)
			if err != nil {
				return fmt.Errorf("could not process queued proposal %v: %w", proposal.Header.ID(), err)
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
	log.Debug().Msg("processing timeout broadcast request from hotstuff")

	// Retrieve all collection nodes in our cluster (excluding myself).
	recipients, err := h.state.Final().Identities(h.clusterIdentityFilter)
	if err != nil {
		return fmt.Errorf("could not get cluster members for broadcasting timeout: %w", err)
	}
	// create the timeout message
	msg := (*messages.ClusterTimeoutObject)(timeout)

	err = h.con.Publish(msg, recipients.NodeIDs()...)
	if err != nil {
		if !errors.Is(err, network.EmptyTargetList) {
			log.Err(err).Msg("could not broadcast timeout")
		}
		return nil
	}
	log.Debug().Msg("cluster timeout was broadcast")
	h.engineMetrics.MessageSent(metrics.EngineCollectionMessageHub, metrics.MessageTimeoutObject)

	return nil
}

// sendOwnVote propagates the vote via unicast to another node that is the next leader
// No errors are expected during normal operations.
func (h *MessageHub) sendOwnVote(packed *packedVote) error {
	log := h.log.With().
		Hex("collection_id", packed.vote.BlockID[:]).
		Uint64("collection_view", packed.vote.View).
		Hex("recipient_id", packed.recipientID[:]).
		Logger()
	log.Debug().Msg("processing vote transmission request from hotstuff")

	// send the vote the desired recipient
	err := h.con.Unicast(packed.vote, packed.recipientID)
	if err != nil {
		log.Err(err).Msg("could not send vote")
		return nil
	}
	log.Debug().Msg("collection vote transmitted")
	h.engineMetrics.MessageSent(metrics.EngineCollectionMessageHub, metrics.MessageBlockVote)

	return nil
}

// sendOwnProposal propagates the block proposal to the consensus committee by broadcasting to all other cluster participants (excluding myself)
// No errors are expected during normal operations.
func (h *MessageHub) sendOwnProposal(proposal *flow.ProposalHeader) error {
	header := proposal.Header
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
		Hex("block_id", logging.ID(header.ID())).
		Hex("parent_id", header.ParentID[:]).
		Hex("ref_block", payload.ReferenceBlockID[:]).
		Int("transaction_count", payload.Collection.Len()).
		Hex("parent_signer_indices", header.ParentVoterIndices).
		Logger()

	log.Debug().Msg("processing cluster broadcast request from hotstuff")

	// retrieve all collection nodes in our cluster
	recipients, err := h.state.Final().Identities(h.clusterIdentityFilter)
	if err != nil {
		return fmt.Errorf("could not get cluster members for broadcasting collection proposal")
	}

	block, err := cluster.NewBlock(
		cluster.UntrustedBlock{
			HeaderBody: header.HeaderBody,
			Payload:    *payload,
		},
	)
	if err != nil {
		return fmt.Errorf("could not build cluster block: %w", err)
	}

	// create the proposal message for the collection
	blockProposal := &cluster.UntrustedProposal{
		Block:           *block,
		ProposerSigData: proposal.ProposerSigData,
	}
	if _, err = cluster.NewProposal(*blockProposal); err != nil {
		return fmt.Errorf("could not build cluster proposal: %w", err)
	}

	message := (*messages.ClusterProposal)(blockProposal)
	// broadcast the proposal to consensus nodes
	err = h.con.Publish(message, recipients.NodeIDs()...)
	if err != nil {
		if !errors.Is(err, network.EmptyTargetList) {
			log.Err(err).Msg("could not send proposal message")
		}
		return nil
	}
	log.Info().Msg("cluster proposal was broadcast")
	h.engineMetrics.MessageSent(metrics.EngineCollectionMessageHub, metrics.MessageBlockProposal)

	return nil
}

// OnOwnVote propagates the vote to relevant recipient(s):
//   - [common case] vote is queued and is sent via unicast to another node that is the next leader by worker
//   - [special case] this node is the next leader: vote is directly forwarded to the node's internal `VoteAggregator`
func (h *MessageHub) OnOwnVote(vote *model.Vote, recipientID flow.Identifier) {
	// special case: I am the next leader
	if recipientID == h.me.NodeID() {
		h.forwardToOwnVoteAggregator(vote) // forward vote to my own `voteAggregator`
		return
	}

	// common case: someone else is leader
	packed := &packedVote{
		recipientID: recipientID,
		vote: &messages.ClusterBlockVote{
			BlockID: vote.BlockID,
			View:    vote.View,
			SigData: vote.SigData,
		},
	}
	if ok := h.ownOutboundVotes.Push(packed); ok {
		h.ownOutboundMessageNotifier.Notify()
	} else {
		h.engineMetrics.OutboundMessageDropped(metrics.EngineCollectionMessageHub, metrics.MessageBlockVote)
	}
}

// OnOwnTimeout forwards timeout to node's internal `timeoutAggregator` and queues timeout for
// subsequent propagation to all consensus participants (excluding this node)
func (h *MessageHub) OnOwnTimeout(timeout *model.TimeoutObject) {
	h.forwardToOwnTimeoutAggregator(timeout) // forward timeout to my own `timeoutAggregator`
	if ok := h.ownOutboundTimeouts.Push(timeout); ok {
		h.ownOutboundMessageNotifier.Notify()
	} else {
		h.engineMetrics.OutboundMessageDropped(metrics.EngineCollectionMessageHub, metrics.MessageTimeoutObject)
	}
}

// OnOwnProposal directly forwards proposal to HotStuff core logic(skipping compliance engine as we assume our
// own proposals to be correct) and queues proposal for subsequent propagation to all consensus participants (including this node).
// The proposal will only be placed in the queue, after the specified delay (or dropped on shutdown signal).
func (h *MessageHub) OnOwnProposal(proposal *flow.ProposalHeader, targetPublicationTime time.Time) {
	go func() {
		select {
		case <-time.After(time.Until(targetPublicationTime)):
		case <-h.ShutdownSignal():
			return
		}

		hotstuffProposal := model.SignedProposalFromFlow(proposal)
		// notify vote aggregator that new block proposal is available, in case we are next leader
		h.voteAggregator.AddBlock(hotstuffProposal) // non-blocking

		// TODO(active-pacemaker): replace with pub/sub?
		// submit proposal to our own processing pipeline
		h.hotstuff.SubmitProposal(hotstuffProposal) // non-blocking

		if ok := h.ownOutboundProposals.Push(proposal); ok {
			h.ownOutboundMessageNotifier.Notify()
		} else {
			h.engineMetrics.OutboundMessageDropped(metrics.EngineCollectionMessageHub, metrics.MessageBlockProposal)
		}
	}()
}

// Process handles incoming messages from consensus channel. After matching message by type, sends it to the correct
// component for handling.
// No errors are expected during normal operations.
//
// TODO(BFT, #7620): This function should not return an error. The networking layer's responsibility is fulfilled
// once it delivers a message to an engine. It does not possess the context required to handle
// errors that may arise during an engine's processing of the message, as error handling for
// message processing falls outside the domain of the networking layer.
//
// Some of the current error returns signal Byzantine behavior, such as forged or malformed
// messages. These cases must be logged and routed to a dedicated violation reporting consumer.
func (h *MessageHub) Process(channel channels.Channel, originID flow.Identifier, message interface{}) error {
	switch msg := message.(type) {
	case *cluster.Proposal:
		h.compliance.OnClusterBlockProposal(flow.Slashable[*cluster.Proposal]{
			OriginID: originID,
			Message:  msg,
		})
	case *flow.BlockVote:
		vote, err := model.NewVote(model.UntrustedVote{
			View:     msg.View,
			BlockID:  msg.BlockID,
			SignerID: originID,
			SigData:  msg.SigData,
		})
		if err != nil {
			// TODO(BFT, #7620): Replace this log statement with a call to the protocol violation consumer.
			h.log.Warn().
				Hex("origin_id", originID[:]).
				Hex("block_id", msg.BlockID[:]).
				Uint64("view", msg.View).
				Err(err).Msgf("received invalid cluster vote message")
			return nil
		}

		h.forwardToOwnVoteAggregator(vote)
	case *model.TimeoutObject:
		h.forwardToOwnTimeoutAggregator(msg)
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
func (h *MessageHub) forwardToOwnVoteAggregator(vote *model.Vote) {
	h.engineMetrics.MessageReceived(metrics.EngineCollectionMessageHub, metrics.MessageBlockVote)
	h.log.Debug().
		Uint64("block_view", vote.View).
		Hex("block_id", vote.BlockID[:]).
		Hex("voter", vote.SignerID[:]).
		Str("vote_id", vote.ID().String()).
		Msg("block vote received, forwarding block vote to hotstuff vote aggregator")
	h.voteAggregator.AddVote(vote)
}

// forwardToOwnTimeoutAggregator logs timeout and forwards it to own `timeoutAggregator`.
// Per API convention, timeoutAggregator` is non-blocking, hence, this call returns quickly.
func (h *MessageHub) forwardToOwnTimeoutAggregator(t *model.TimeoutObject) {
	h.engineMetrics.MessageReceived(metrics.EngineCollectionMessageHub, metrics.MessageTimeoutObject)
	h.log.Debug().
		Hex("signer_id", t.SignerID[:]).
		Uint64("view", t.View).
		Uint64("newest_qc_view", t.NewestQC.View).
		Msg("timeout received, forwarding timeout to hotstuff timeout aggregator")
	h.timeoutAggregator.AddTimeout(t)
}
