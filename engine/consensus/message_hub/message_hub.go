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
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
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
	log                    zerolog.Logger
	me                     module.Local
	state                  protocol.State
	headers                storage.Headers
	payloads               storage.Payloads
	con                    network.Conduit
	pushBlocksCon          network.Conduit
	queuedMessagesNotifier engine.Notifier
	queuedVotes            *fifoqueue.FifoQueue // queue for handling outgoing vote transmissions
	queuedProposals        *fifoqueue.FifoQueue // queue for handling outgoing proposal transmissions
	queuedTimeouts         *fifoqueue.FifoQueue // queue for handling outgoing timeout transmissions

	// injected dependencies
	compliance        network.MessageProcessor   // handler of incoming block proposals
	hotstuff          module.HotStuff            // used to submit proposals that were previously broadcast
	voteAggregator    hotstuff.VoteAggregator    // handler of incoming votes
	timeoutAggregator hotstuff.TimeoutAggregator // handler of incoming timeouts
}

var _ network.MessageProcessor = (*MessageHub)(nil)
var _ hotstuff.CommunicatorConsumer = (*MessageHub)(nil)

// NewMessageHub constructs new instance of message hub
// No errors are expected during normal operations.
func NewMessageHub(log zerolog.Logger,
	net network.Network,
	me module.Local,
	compliance network.MessageProcessor,
	hotstuff module.HotStuff,
	voteAggregator hotstuff.VoteAggregator,
	timeoutAggregator hotstuff.TimeoutAggregator,
	state protocol.State,
	headers storage.Headers,
	payloads storage.Payloads,
) (*MessageHub, error) {
	queuedVotes, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultVoteQueueCapacity),
	)
	if err != nil {
		return nil, fmt.Errorf("could not initialize votes queue")
	}
	queuedProposals, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultProposalQueueCapacity),
	)
	if err != nil {
		return nil, fmt.Errorf("could not initialize blocks queue")
	}
	queuedTimeouts, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultTimeoutQueueCapacity),
	)
	if err != nil {
		return nil, fmt.Errorf("could not initialize timeouts queue")
	}
	hub := &MessageHub{
		log:                    log.With().Str("engine", "message_hub").Logger(),
		me:                     me,
		state:                  state,
		headers:                headers,
		payloads:               payloads,
		compliance:             compliance,
		hotstuff:               hotstuff,
		voteAggregator:         voteAggregator,
		timeoutAggregator:      timeoutAggregator,
		queuedMessagesNotifier: engine.NewNotifier(),
		queuedVotes:            queuedVotes,
		queuedProposals:        queuedProposals,
		queuedTimeouts:         queuedTimeouts,
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
	notifier := h.queuedMessagesNotifier.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-notifier:
			err := h.processQueuedMessages(ctx)
			if err != nil {
				ctx.Throw(fmt.Errorf("internal error processing queued messages: %w", err))
				return
			}
		}
	}
}

// processQueuedMessages is a function which dispatches previously queued messages on worker thread
// This function is called whenever we have queued messages ready to be dispatched.
// No errors are expected during normal operations.
func (h *MessageHub) processQueuedMessages(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, ok := h.queuedProposals.Pop()
		if ok {
			block := msg.(*flow.Header)
			err := h.processQueuedProposal(block)
			if err != nil {
				return fmt.Errorf("could not process queued block %v: %w", block.ID(), err)
			}

			continue
		}

		msg, ok = h.queuedVotes.Pop()
		if ok {
			packed := msg.(*packedVote)
			err := h.processQueuedVote(packed)
			if err != nil {
				return fmt.Errorf("could not process queued vote: %w", err)
			}

			continue
		}

		msg, ok = h.queuedTimeouts.Pop()
		if ok {
			err := h.processQueuedTimeout(msg.(*messages.TimeoutObject))
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

// processQueuedTimeout performs actual processing of model.TimeoutObject, as a result of successful invocation
// broadcasts timeout object to consensus committee.
// No errors are expected during normal operations.
func (h *MessageHub) processQueuedTimeout(timeout *messages.TimeoutObject) error {
	logContext := h.log.With().
		Uint64("timeout_newest_qc_view", timeout.NewestQC.View).
		Uint64("timeout_tick", timeout.Seq).
		Hex("timeout_newest_qc_block_id", timeout.NewestQC.BlockID[:]).
		Uint64("timeout_view", timeout.View)

	if timeout.LastViewTC != nil {
		logContext.
			Uint64("last_view_tc_view", timeout.LastViewTC.View).
			Uint64("last_view_tc_newest_qc_view", timeout.LastViewTC.NewestQC.View)
	}
	log := logContext.Logger()

	log.Info().Msg("processing timeout broadcast request from hotstuff")

	// Retrieve all consensus nodes (excluding myself).
	// CAUTION: We must include also nodes with weight zero, because otherwise
	//          TCs might not be constructed at epoch switchover.
	recipients, err := h.state.Final().Identities(filter.And(
		filter.HasRole(flow.RoleConsensus),
		filter.Not(filter.HasNodeID(h.me.NodeID())),
	))
	if err != nil {
		return fmt.Errorf("could not get consensus recipients for broadcasting timeout: %w", err)
	}

	err = h.con.Publish(timeout, recipients.NodeIDs()...)
	if errors.Is(err, network.EmptyTargetList) {
		return nil
	}
	if err != nil {
		log.Err(err).Msg("could not broadcast timeout")
		return nil
	}
	log.Info().Msg("consensus timeout was broadcast")

	// TODO(active-pacemaker): update metrics
	//e.metrics.MessageSent(metrics.EngineClusterCompliance, metrics.MessageClusterBlockProposal)
	//e.core.collectionMetrics.ClusterBlockProposed(block)

	return nil
}

// processQueuedVote performs actual processing of model.Vote, as a result of successful invocation
// sends vote to designated receiver.
// No errors are expected during normal operations.
func (h *MessageHub) processQueuedVote(packed *packedVote) error {
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
	// TODO(active-pacemaker): update metrics
	//h.engineMetrics.MessageSent(metrics.EngineCompliance, metrics.MessageBlockVote)
	log.Info().Msg("block vote transmitted")

	return nil
}

// processQueuedProposal performs actual processing of flow.Header, as a result of successful invocation
// broadcasts block proposal to consensus committee and rest of Flow network.
// No errors are expected during normal operations.
func (h *MessageHub) processQueuedProposal(header *flow.Header) error {
	// first, check that we are the proposer of the block
	if header.ProposerID != h.me.NodeID() {
		return fmt.Errorf("cannot broadcast proposal with non-local proposer (%x)", header.ProposerID)
	}

	// get the parent of the block
	parent, err := h.headers.ByBlockID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	// fill in the fields that can't be populated by HotStuff
	// TODO(active-pacemaker): will be not relevant after merging flow.Header change
	header.ChainID = parent.ChainID
	header.Height = parent.Height + 1

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

	// TODO(active-pacemaker): replace with pub/sub?
	h.hotstuff.SubmitProposal(header, parent.View) // non-blocking

	// Retrieve all consensus nodes (excluding myself).
	// CAUTION: We must include also nodes with weight zero, because otherwise
	//          new consensus nodes for the next epoch are left out.
	// Note: retrieving the final state requires a time-intensive database read.
	//       Therefore, we execute this in a separate routine, because
	//       `OnOwnTimeout` is directly called by the consensus core logic.
	allIdentities, err := h.state.AtBlockID(header.ParentID).Identities(filter.And(
		filter.Not(filter.HasNodeID(h.me.NodeID())),
	))
	if err != nil {
		return fmt.Errorf("could not get identities for broadcasting proposal: %w", err)
	}

	recipients := allIdentities.Filter(filter.HasRole(flow.RoleConsensus))

	// NOTE: some fields are not needed for the message
	// - proposer ID is conveyed over the network message
	// - the payload hash is deduced from the payload
	proposal := &messages.BlockProposal{
		Header:  header,
		Payload: payload,
	}

	// broadcast the proposal to consensus nodes
	err = h.con.Publish(proposal, recipients.NodeIDs()...)
	if errors.Is(err, network.EmptyTargetList) {
		return nil
	}
	if err != nil {
		log.Err(err).Msg("could not send proposal message")
		return nil
	}

	//TODO(active-pacemaker): update metrics
	//e.engineMetrics.MessageSent(metrics.EngineCompliance, metrics.MessageBlockProposal)

	log.Info().Msg("block proposal was broadcast")

	// submit proposal to non-consensus nodes
	h.provideProposal(proposal, allIdentities.Filter(filter.And(
		filter.Not(filter.Ejected),
		filter.Not(filter.HasRole(flow.RoleConsensus)),
	)))

	return nil
}

// provideProposal is used when we want to broadcast a local block to the rest  of the
// network (non-consensus nodes).
func (h *MessageHub) provideProposal(proposal *messages.BlockProposal, recipients flow.IdentityList) {

	blockID := proposal.Header.ID()
	log := h.log.With().
		Uint64("block_view", proposal.Header.View).
		Hex("block_id", blockID[:]).
		Hex("parent_id", proposal.Header.ParentID[:]).
		Logger()
	log.Info().Msg("block proposal submitted for propagation")

	// submit the block to the targets
	err := h.pushBlocksCon.Publish(proposal, recipients.NodeIDs()...)
	if err != nil {
		h.log.Err(err).Msg("failed to broadcast block")
		return
	}

	// TODO(active-pacemaker): update metrics
	//h.message.MessageSent(metrics.EngineConsensusProvider, metrics.MessageBlockProposal)
	log.Info().Msg("block proposal propagated to non-consensus nodes")
}

// OnOwnVote queues vote for subsequent sending
func (h *MessageHub) OnOwnVote(blockID flow.Identifier, view uint64, sigData []byte, recipientID flow.Identifier) {
	vote := &packedVote{
		recipientID: recipientID,
		vote: &messages.BlockVote{
			BlockID: blockID,
			View:    view,
			SigData: sigData,
		},
	}
	if ok := h.queuedVotes.Push(vote); ok {
		h.queuedMessagesNotifier.Notify()
	}
}

// OnOwnTimeout queues timeout for subsequent sending
func (h *MessageHub) OnOwnTimeout(timeout *model.TimeoutObject, timeoutTick uint64) {
	if ok := h.queuedTimeouts.Push(&messages.TimeoutObject{
		Seq:        timeoutTick,
		View:       timeout.View,
		NewestQC:   timeout.NewestQC,
		LastViewTC: timeout.LastViewTC,
		SigData:    timeout.SigData,
	}); ok {
		h.queuedMessagesNotifier.Notify()
	}
}

// OnOwnProposal queues proposal for subsequent sending
func (h *MessageHub) OnOwnProposal(proposal *flow.Header, delay time.Duration) {
	go func() {
		select {
		case <-time.After(delay):
		case <-h.ShutdownSignal():
			return
		}

		if ok := h.queuedProposals.Push(proposal); ok {
			h.queuedMessagesNotifier.Notify()
		}
	}()
}

// Process handles incoming messages from consensus channel. After matching message by type, sends it to the correct
// component for handling.
// No errors are expected during normal operations.
func (h *MessageHub) Process(channel channels.Channel, originID flow.Identifier, message interface{}) error {
	switch msg := message.(type) {
	case *events.SyncedBlock:
		return h.compliance.Process(channel, h.me.NodeID(), message)
	case *messages.BlockProposal:
		return h.compliance.Process(channel, h.me.NodeID(), message)
	case *messages.BlockVote:
		v := &model.Vote{
			View:     msg.View,
			BlockID:  msg.BlockID,
			SignerID: originID,
			SigData:  msg.SigData,
		}
		h.log.Info().
			Uint64("block_view", v.View).
			Hex("block_id", v.BlockID[:]).
			Hex("voter", v.SignerID[:]).
			Str("vote_id", v.ID().String()).
			Msg("block vote received, forwarding block vote to hotstuff vote aggregator")

		// forward the vote to aggregator for processing
		h.voteAggregator.AddVote(v)
	case *messages.TimeoutObject:
		t := &model.TimeoutObject{
			View:       msg.View,
			NewestQC:   msg.NewestQC,
			LastViewTC: msg.LastViewTC,
			SignerID:   originID,
			SigData:    msg.SigData,
		}
		h.log.Info().
			Hex("origin_id", originID[:]).
			Uint64("view", t.View).
			Str("timeout_id", t.ID().String()).
			Msg("timeout received, forwarding timeout to hotstuff timeout aggregator")
		// forward the timeout to aggregator for processing
		h.timeoutAggregator.AddTimeout(t)
	default:
		h.log.Warn().Msgf("%v delivered unsupported message %T through %v", originID, message, channel)
	}
	return nil
}
