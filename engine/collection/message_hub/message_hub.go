package message_hub

import (
	"context"
	"errors"
	"fmt"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	clusterkv "github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/rs/zerolog"
	"time"
)

type packedVote struct {
	recipientID flow.Identifier
	vote        *messages.ClusterBlockVote
}

type MessageHub struct {
	*component.ComponentManager
	notifications.NoopConsumer
	log                    zerolog.Logger
	me                     module.Local
	state                  protocol.State
	headers                storage.Headers
	payloads               storage.ClusterPayloads
	con                    network.Conduit
	queuedMessagesNotifier engine.Notifier
	queuedVotes            *fifoqueue.FifoQueue
	queuedProposals        *fifoqueue.FifoQueue
	queuedTimeouts         *fifoqueue.FifoQueue
	cluster                flow.IdentityList // consensus participants in our cluster

	// injected dependencies
	compliance        network.MessageProcessor
	prov              consensus.ProposalProvider
	hotstuff          module.HotStuff
	voteAggregator    hotstuff.VoteAggregator
	timeoutAggregator hotstuff.TimeoutAggregator
}

var _ network.MessageProcessor = (*MessageHub)(nil)
var _ hotstuff.CommunicatorConsumer = (*MessageHub)(nil)

func NewMessageHub(log zerolog.Logger,
	net network.Network,
	me module.Local,
	compliance network.MessageProcessor,
	prov consensus.ProposalProvider,
	hotstuff module.HotStuff,
	voteAggregator hotstuff.VoteAggregator,
	timeoutAggregator hotstuff.TimeoutAggregator,
	state protocol.State,
	clusterState clusterkv.State,
	headers storage.Headers,
	payloads storage.ClusterPayloads,
) (*MessageHub, error) {
	// find my cluster for the current epoch
	// TODO this should flow from cluster state as source of truth
	clusters, err := state.Final().Epochs().Current().Clustering()
	if err != nil {
		return nil, fmt.Errorf("could not get clusters: %w", err)
	}
	currentCluster, _, found := clusters.ByNodeID(me.NodeID())
	if !found {
		return nil, fmt.Errorf("could not find cluster for self")
	}

	queuedVotes, err := fifoqueue.NewFifoQueue()
	if err != nil {
		return nil, fmt.Errorf("could not initialize votes queue")
	}
	queuedProposals, err := fifoqueue.NewFifoQueue()
	if err != nil {
		return nil, fmt.Errorf("could not initialize votes queue")
	}
	queuedTimeouts, err := fifoqueue.NewFifoQueue()
	if err != nil {
		return nil, fmt.Errorf("could not initialize votes queue")
	}
	hub := &MessageHub{
		log:                    log,
		me:                     me,
		state:                  state,
		headers:                headers,
		payloads:               payloads,
		compliance:             compliance,
		prov:                   prov,
		hotstuff:               hotstuff,
		voteAggregator:         voteAggregator,
		timeoutAggregator:      timeoutAggregator,
		queuedMessagesNotifier: engine.NewNotifier(),
		queuedVotes:            queuedVotes,
		queuedProposals:        queuedProposals,
		queuedTimeouts:         queuedTimeouts,
		cluster:                currentCluster,
	}

	// register network conduit
	chainID, err := clusterState.Params().ChainID()
	if err != nil {
		return nil, fmt.Errorf("could not get chain ID: %w", err)
	}
	conduit, err := net.Register(channels.ConsensusCluster(chainID), hub)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	hub.con = conduit

	componentBuilder := component.NewComponentManagerBuilder()
	componentBuilder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		ready()
		hub.queuedMessagesProcessingLoop(ctx)
	})
	hub.ComponentManager = componentBuilder.Build()
	return hub, nil
}

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
			err := h.processQueuedBlock(block)
			if err != nil {
				return fmt.Errorf("could not process queued block %v: %w", block.ID(), err)
			}

			//h.log.Info().
			//	Uint64("view", block.Block.View).
			//	Hex("block_id", block.Block.BlockID[:]).
			//	Msg("block has been processed successfully")

			continue
		}

		msg, ok = h.queuedVotes.Pop()
		if ok {
			packed := msg.(*packedVote)
			err := h.processQueuedVote(packed)
			if err != nil {
				return fmt.Errorf("could not process queued vote: %w", err)
			}

			h.log.Info().
				Uint64("view", packed.vote.View).
				Hex("block_id", packed.vote.BlockID[:]).
				Msg("packed has been processed successfully")

			continue
		}

		msg, ok = h.queuedTimeouts.Pop()
		if ok {
			err := h.processQueuedTimeout(msg.(*model.TimeoutObject))
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

func (h *MessageHub) processQueuedTimeout(timeout *model.TimeoutObject) error {
	logContext := h.log.With().
		Uint64("timeout_newest_qc_view", timeout.NewestQC.View).
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
		filter.In(h.cluster),
		filter.Not(filter.HasNodeID(h.me.NodeID())),
	))
	if err != nil {
		return fmt.Errorf("could not get cluster members for broadcasting timeout: %w", err)
	}
	// create the timeout message
	msg := &messages.TimeoutObject{
		View:       timeout.View,
		NewestQC:   timeout.NewestQC,
		LastViewTC: timeout.LastViewTC,
		SigData:    timeout.SigData,
	}

	err = h.con.Publish(msg, recipients.NodeIDs()...)
	if errors.Is(err, network.EmptyTargetList) {
		return nil
	}
	if err != nil {
		log.Err(err).Msg("could not broadcast timeout")
		return nil
	}
	log.Info().Msg("cluster timeout was broadcast")

	// TODO(active-pacemaker): update metrics
	//e.metrics.MessageSent(metrics.EngineClusterCompliance, metrics.MessageClusterBlockProposal)
	//e.core.collectionMetrics.ClusterBlockProposed(block)

	return nil
}

func (h *MessageHub) processQueuedVote(packed *packedVote) error {
	log := h.log.With().
		Hex("collection_id", packed.vote.BlockID[:]).
		Uint64("collection_view", packed.vote.View).
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
	log.Info().Msg("collection vote transmitted")

	return nil
}

// processQueuedBlock performs actual processing of queued block proposals, this method is called from multiple
// concurrent goroutines.
func (h *MessageHub) processQueuedBlock(header *flow.Header) error {
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
	// TODO clean this up - currently we set these fields in builder, then lose them in HotStuff, then need to set them again here
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
		Hex("block_id", logging.ID(header.ID())).
		Hex("parent_id", header.ParentID[:]).
		Hex("ref_block", payload.ReferenceBlockID[:]).
		Int("transaction_count", payload.Collection.Len()).
		Hex("parent_signer_indices", header.ParentVoterIndices).
		Logger()

	log.Debug().Msg("processing cluster broadcast request from hotstuff")

	// retrieve all collection nodes in our cluster
	recipients, err := h.state.Final().Identities(filter.And(
		filter.In(h.cluster),
		filter.Not(filter.HasNodeID(h.me.NodeID())),
	))
	if err != nil {
		return fmt.Errorf("could not get cluster members for broadcasting collection proposal")
	}

	// TODO(active-pacemaker):
	h.hotstuff.SubmitProposal(header, parent.View) // non-blocking

	// create the proposal message for the collection
	proposal := &messages.ClusterBlockProposal{
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

	log.Info().Msg("cluster proposal was broadcast")

	//TODO(active-pacemaker): add metrics for ClusterBlockProposed
	return nil
}

func (h *MessageHub) SendVote(blockID flow.Identifier, view uint64, sigData []byte, recipientID flow.Identifier) {
	vote := &packedVote{
		recipientID: recipientID,
		vote: &messages.ClusterBlockVote{
			BlockID: blockID,
			View:    view,
			SigData: sigData,
		},
	}
	if ok := h.queuedVotes.Push(vote); ok {
		h.queuedMessagesNotifier.Notify()
	}
}

func (h *MessageHub) BroadcastTimeout(timeout *model.TimeoutObject) {
	if ok := h.queuedTimeouts.Push(timeout); ok {
		h.queuedMessagesNotifier.Notify()
	}
}

func (h *MessageHub) BroadcastProposalWithDelay(proposal *flow.Header, delay time.Duration) {
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

func (h *MessageHub) Process(channel channels.Channel, originID flow.Identifier, message interface{}) error {
	switch msg := message.(type) {
	case *events.SyncedClusterBlock:
		return h.compliance.Process(channel, originID, message)
	case *messages.ClusterBlockProposal:
		return h.compliance.Process(channel, originID, message)
	case *messages.ClusterBlockVote:
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

		// forward the vote to hotstuff for processing
		h.voteAggregator.AddVote(v)
	case *messages.ClusterTimeoutObject:
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
		// forward the timeout to hotstuff for processing
		h.timeoutAggregator.AddTimeout(t)
	default:
		h.log.Warn().Msgf("%v delivered unsupported message %T through %v", originID, message, channel)
	}
	return nil
}
