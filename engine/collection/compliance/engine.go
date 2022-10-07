package compliance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus/sealing/counters"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// defaultBlockQueueCapacity maximum capacity of inbound queue for `messages.ClusterBlockProposal`s
const defaultBlockQueueCapacity = 10_000

// defaultVoteQueueCapacity maximum capacity of inbound queue for `messages.ClusterBlockVote`s
const defaultVoteQueueCapacity = 1000

// defaultTimeoutObjectsQueueCapacity maximum capacity of inbound queue for `messages.ClusterTimeoutObject`s
const defaultTimeoutObjectsQueueCapacity = 1000

// Engine is a wrapper struct for `Core` which implements cluster consensus algorithm.
// Engine is responsible for handling incoming messages, queueing for processing, broadcasting proposals.
type Engine struct {
	log      zerolog.Logger
	metrics  module.EngineMetrics
	me       module.Local
	headers  storage.Headers
	payloads storage.ClusterPayloads
	state    protocol.State
	core     *Core
	// quues for inbound messages
	pendingBlocks engine.MessageStore
	// TODO remove pendingVotes and pendingTimeouts - we will pass these directly to the Aggregator
	pendingVotes    engine.MessageStore
	pendingTimeouts engine.MessageStore
	messageHandler  *engine.MessageHandler
	// tracking finalized view
	finalizedView              counters.StrictMonotonousCounter
	finalizationEventsNotifier engine.Notifier
	con                        network.Conduit
	cluster                    flow.IdentityList // consensus participants in our cluster

	cm *component.ComponentManager
	component.Component
}

var _ network.MessageProcessor = (*Engine)(nil)
var _ component.Component = (*Engine)(nil)

func NewEngine(
	log zerolog.Logger,
	net network.Network,
	me module.Local,
	state protocol.State,
	payloads storage.ClusterPayloads,
	core *Core,
) (*Engine, error) {
	engineLog := log.With().Str("cluster_compliance", "engine").Logger()

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

	// FIFO queue for block proposals
	blocksQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultBlockQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) {
			core.mempoolMetrics.MempoolEntries(metrics.ResourceClusterBlockProposalQueue, uint(len))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound receipts: %w", err)
	}
	pendingBlocks := &engine.FifoMessageStore{
		FifoQueue: blocksQueue,
	}

	// FIFO queue for block votes
	votesQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultVoteQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) { core.mempoolMetrics.MempoolEntries(metrics.ResourceClusterBlockVoteQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound approvals: %w", err)
	}
	pendingVotes := &engine.FifoMessageStore{FifoQueue: votesQueue}

	// FIFO queue for timeout objects
	// TODO(active-pacemaker): update metrics
	timeoutObjectsQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultTimeoutObjectsQueueCapacity))
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound timeout objects: %w", err)
	}
	pendingTimeouts := &engine.FifoMessageStore{FifoQueue: timeoutObjectsQueue}

	// define message queueing behaviour
	handler := engine.NewMessageHandler(
		engineLog,
		engine.NewNotifier(),
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.ClusterBlockProposal)
				if ok {
					core.engineMetrics.MessageReceived(metrics.EngineClusterCompliance, metrics.MessageClusterBlockProposal)
				}
				return ok
			},
			Store: pendingBlocks,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*events.SyncedClusterBlock)
				if ok {
					core.engineMetrics.MessageReceived(metrics.EngineClusterCompliance, metrics.MessageSyncedClusterBlock)
				}
				return ok
			},
			Map: func(msg *engine.Message) (*engine.Message, bool) {
				syncedBlock := msg.Payload.(*events.SyncedClusterBlock)
				msg = &engine.Message{
					OriginID: msg.OriginID,
					Payload: &messages.ClusterBlockProposal{
						Header:  syncedBlock.Block.Header,
						Payload: syncedBlock.Block.Payload,
					},
				}
				return msg, true
			},
			Store: pendingBlocks,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.ClusterBlockVote)
				if ok {
					core.engineMetrics.MessageReceived(metrics.EngineClusterCompliance, metrics.MessageClusterBlockVote)
				}
				return ok
			},
			Store: pendingVotes,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.ClusterTimeoutObject)
				// TODO(active-pacemaker): update metrics
				//if ok {
				//core.metrics.MessageReceived(metrics.EngineClusterCompliance, metrics.MessageClusterBlockVote)
				//}
				return ok
			},
			Store: pendingTimeouts,
		},
	)

	eng := &Engine{
		log:                        engineLog,
		metrics:                    core.engineMetrics,
		me:                         me,
		headers:                    core.headers,
		payloads:                   payloads,
		state:                      state,
		core:                       core,
		pendingBlocks:              pendingBlocks,
		pendingVotes:               pendingVotes,
		pendingTimeouts:            pendingTimeouts,
		messageHandler:             handler,
		finalizationEventsNotifier: engine.NewNotifier(),
		con:                        nil,
		cluster:                    currentCluster,
	}

	// register network conduit
	chainID, err := core.state.Params().ChainID()
	if err != nil {
		return nil, fmt.Errorf("could not get chain ID: %w", err)
	}
	conduit, err := net.Register(channels.ConsensusCluster(chainID), eng)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	eng.con = conduit

	// create the component manager and worker threads
	eng.cm = component.NewComponentManagerBuilder().
		AddWorker(eng.processMessagesLoop).
		AddWorker(eng.finalizationProcessingLoop).
		Build()
	eng.Component = eng.cm

	return eng, nil
}

// WithConsensus adds the consensus algorithm to the engine. This must be
// called before the engine can start.
func (e *Engine) WithConsensus(hot module.HotStuff) *Engine {
	e.core.hotstuff = hot
	return e
}

// WithSync adds the block requester to the engine. This must be
// called before the engine can start.
func (e *Engine) WithSync(sync module.BlockRequester) *Engine {
	e.core.sync = sync
	return e
}

func (e *Engine) Start(ctx irrecoverable.SignalerContext) {
	if e.core.hotstuff == nil {
		ctx.Throw(fmt.Errorf("must initialize compliance engine with hotstuff engine"))
	}
	if e.core.sync == nil {
		ctx.Throw(fmt.Errorf("must initialize compliance engine with sync engine"))
	}

	e.log.Info().Msg("starting hotstuff")
	e.core.hotstuff.Start(ctx)
	e.log.Info().Msg("hotstuff started")

	e.log.Info().Msg("starting compliance engine")
	e.Component.Start(ctx)
	e.log.Info().Msg("compliance engine started")
}

// Ready returns a ready channel that is closed once the engine has fully started.
// For the consensus engine, we wait for hotstuff to start.
func (e *Engine) Ready() <-chan struct{} {
	// NOTE: this will create long-lived goroutines each time Ready is called
	// Since Ready is called infrequently, that is OK. If the call frequency changes, change this code.
	return util.AllReady(e.cm, e.core.hotstuff)
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	// NOTE: this will create long-lived goroutines each time Done is called
	// Since Done is called infrequently, that is OK. If the call frequency changes, change this code.
	return util.AllDone(e.cm, e.core.hotstuff)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	err := e.messageHandler.Process(originID, event)
	if err != nil {
		if engine.IsIncompatibleInputTypeError(err) {
			e.log.Warn().Msgf("%v delivered unsupported message %T through %v", originID, event, channel)
			return nil
		}
		return fmt.Errorf("unexpected error while processing engine message: %w", err)
	}
	return nil
}

// processMessagesLoop processes available block, vote, and timeout messages as they are queued.
func (e *Engine) processMessagesLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	doneSignal := ctx.Done()
	newMessageSignal := e.messageHandler.GetNotifier()
	for {
		select {
		case <-doneSignal:
			return
		case <-newMessageSignal:
			err := e.processAvailableMessages(ctx)
			if err != nil {
				ctx.Throw(err)
			}
		}
	}
}

// processAvailableMessages processes any available messages from the inbound queues.
// Only returns when all inbound queues are empty (or the engine is terminated).
// No errors expected during normal operations.
func (e *Engine) processAvailableMessages(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, ok := e.pendingBlocks.Get()
		if ok {
			err := e.core.OnBlockProposal(msg.OriginID, msg.Payload.(*messages.ClusterBlockProposal))
			if err != nil {
				return fmt.Errorf("could not handle block proposal: %w", err)
			}
			continue
		}

		msg, ok = e.pendingVotes.Get()
		if ok {
			err := e.core.OnBlockVote(msg.OriginID, msg.Payload.(*messages.ClusterBlockVote))
			if err != nil {
				return fmt.Errorf("could not handle block vote: %w", err)
			}
			continue
		}

		msg, ok = e.pendingTimeouts.Get()
		if ok {
			err := e.core.OnTimeoutObject(msg.OriginID, msg.Payload.(*messages.ClusterTimeoutObject))
			if err != nil {
				return fmt.Errorf("could not handle timeout object: %w", err)
			}
			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

// SendVote will send a vote to the desired node.
func (e *Engine) SendVote(blockID flow.Identifier, view uint64, sigData []byte, recipientID flow.Identifier) error {

	log := e.log.With().
		Hex("collection_id", blockID[:]).
		Uint64("collection_view", view).
		Hex("recipient_id", recipientID[:]).
		Logger()
	log.Info().Msg("processing vote transmission request from hotstuff")

	// build the vote message
	vote := &messages.ClusterBlockVote{
		BlockID: blockID,
		View:    view,
		SigData: sigData,
	}

	// spawn a goroutine to asynchronously send the vote
	// we do this so that network operations do not block the HotStuff EventLoop
	go func() {
		// send the vote the desired recipient
		err := e.con.Unicast(vote, recipientID)
		if err != nil {
			log.Warn().Err(err).Msg("could not send vote")
			return
		}
		e.metrics.MessageSent(metrics.EngineClusterCompliance, metrics.MessageClusterBlockVote)
		log.Info().Msg("collection vote transmitted")
	}()

	return nil
}

// BroadcastTimeout submits a cluster timeout object to all collection nodes in our cluster
// No errors are expected during normal operation.
func (e *Engine) BroadcastTimeout(timeout *model.TimeoutObject) error {
	logContext := e.log.With().
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

	// spawn a goroutine to asynchronously broadcast the timeout object
	// we do this so that network operations do not block the HotStuff EventLoop
	go func() {
		// Retrieve all collection nodes in our cluster (excluding myself).
		// Note: retrieving the final state requires a time-intensive database read.
		//       Therefore, we execute this in a separate routine, because
		//       `BroadcastTimeout` is directly called by the consensus core logic.
		recipients, err := e.state.Final().Identities(filter.And(
			filter.In(e.cluster),
			filter.Not(filter.HasNodeID(e.me.NodeID())),
		))
		if err != nil {
			e.log.Fatal().Err(err).Msg("could not get cluster members for broadcasting timeout")
		}

		// create the proposal message for the collection
		msg := &messages.ClusterTimeoutObject{
			View:       timeout.View,
			NewestQC:   timeout.NewestQC,
			LastViewTC: timeout.LastViewTC,
			SigData:    timeout.SigData,
		}

		err = e.con.Publish(msg, recipients.NodeIDs()...)
		if errors.Is(err, network.EmptyTargetList) {
			return
		}
		if err != nil {
			log.Err(err).Msg("could not broadcast timeout")
			return
		}
		log.Info().Msg("cluster timeout was broadcast")

		// TODO(active-pacemaker): update metrics
		//e.metrics.MessageSent(metrics.EngineClusterCompliance, metrics.MessageClusterBlockProposal)
		//e.core.collectionMetrics.ClusterBlockProposed(block)
	}()

	return nil
}

// BroadcastProposalWithDelay submits a cluster block proposal (effectively a proposal
// for the next collection) to all the collection nodes in our cluster.
// No errors are expected during normal operation.
func (e *Engine) BroadcastProposalWithDelay(header *flow.Header, delay time.Duration) error {
	// first, check that we are the proposer of the block
	if header.ProposerID != e.me.NodeID() {
		return fmt.Errorf("cannot broadcast proposal with non-local proposer (%x)", header.ProposerID)
	}

	// get the parent of the block
	parent, err := e.headers.ByBlockID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	// fill in the fields that can't be populated by HotStuff
	// TODO clean this up - currently we set these fields in builder, then lose them in HotStuff, then need to set them again here
	header.ChainID = parent.ChainID
	header.Height = parent.Height + 1

	// retrieve the payload for the block
	payload, err := e.payloads.ByBlockID(header.ID())
	if err != nil {
		return fmt.Errorf("could not get payload for block: %w", err)
	}

	log := e.log.With().
		Str("chain_id", header.ChainID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", logging.ID(header.ID())).
		Hex("parent_id", header.ParentID[:]).
		Hex("ref_block", payload.ReferenceBlockID[:]).
		Int("transaction_count", payload.Collection.Len()).
		Hex("parent_signer_indices", header.ParentVoterIndices).
		Dur("delay", delay).
		Logger()

	log.Debug().Msg("processing cluster broadcast request from hotstuff")

	// spawn a goroutine to asynchronously broadcast the proposal - we do this
	// to introduce a pre-proposal delay without blocking the Hotstuff EventLoop thread
	go func() {
		select {
		case <-time.After(delay):
		case <-e.cm.ShutdownSignal():
			return
		}

		// retrieve all collection nodes in our cluster
		recipients, err := e.state.Final().Identities(filter.And(
			filter.In(e.cluster),
			filter.Not(filter.HasNodeID(e.me.NodeID())),
		))
		if err != nil {
			e.log.Fatal().Err(err).Msg("could not get cluster members for broadcasting collection proposal")
		}

		// forward collection proposal to node's local consensus instance
		e.core.hotstuff.SubmitProposal(header, parent.View) // non-blocking

		// create the proposal message for the collection
		msg := &messages.ClusterBlockProposal{
			Header:  header,
			Payload: payload,
		}

		err = e.con.Publish(msg, recipients.NodeIDs()...)
		if err != nil {
			if errors.Is(err, network.EmptyTargetList) {
				return
			}
			log.Err(err).Msg("could not send proposal message")
		} else {
			e.metrics.MessageSent(metrics.EngineClusterCompliance, metrics.MessageClusterBlockProposal)
		}

		log.Info().Msg("cluster proposal was broadcast")

		block := &cluster.Block{
			Header:  header,
			Payload: payload,
		}
		e.core.collectionMetrics.ClusterBlockProposed(block)
	}()

	return nil
}

// BroadcastProposal will propagate a block proposal to all non-local consensus nodes.
// Note the header has incomplete fields, because it was converted from a hotstuff.
// No errors are expected during normal operation.
func (e *Engine) BroadcastProposal(header *flow.Header) error {
	return e.BroadcastProposalWithDelay(header, 0)
}

// OnFinalizedBlock implements the `OnFinalizedBlock` callback from the `hotstuff.FinalizationConsumer`
// It informs sealing.Core about finalization of the respective block.
//
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnFinalizedBlock(block *model.Block) {
	if e.finalizedView.Set(block.View) {
		e.finalizationEventsNotifier.Notify()
	}
}

// finalizationProcessingLoop is a separate goroutine that performs processing of finalization events
func (e *Engine) finalizationProcessingLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	doneSignal := ctx.Done()
	blockFinalizedSignal := e.finalizationEventsNotifier.Channel()
	for {
		select {
		case <-doneSignal:
			return
		case <-blockFinalizedSignal:
			e.core.ProcessFinalizedView(e.finalizedView.Value())
		}
	}
}
