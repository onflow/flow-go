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
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// defaultBlockQueueCapacity maximum capacity of block proposals queue
const defaultBlockQueueCapacity = 10000

// defaultVoteQueueCapacity maximum capacity of block votes queue
const defaultVoteQueueCapacity = 1000

// Engine is a wrapper struct for `Core` which implements cluster consensus algorithm.
// Engine is responsible for handling incoming messages, queueing for processing, broadcasting proposals.
type Engine struct {
	unit                       *engine.Unit
	lm                         *lifecycle.LifecycleManager
	log                        zerolog.Logger
	metrics                    module.EngineMetrics
	me                         module.Local
	headers                    storage.Headers
	payloads                   storage.ClusterPayloads
	state                      protocol.State
	core                       *Core
	pendingBlocks              engine.MessageStore
	pendingVotes               engine.MessageStore
	messageHandler             *engine.MessageHandler
	finalizedView              counters.StrictMonotonousCounter
	finalizationEventsNotifier engine.Notifier
	con                        network.Conduit
	stopHotstuff               context.CancelFunc
	cluster                    flow.IdentityList // consensus participants in our cluster
}

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

	// define message queueing behaviour
	handler := engine.NewMessageHandler(
		engineLog,
		engine.NewNotifier(),
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.ClusterBlockProposal)
				if ok {
					core.metrics.MessageReceived(metrics.EngineClusterCompliance, metrics.MessageClusterBlockProposal)
				}
				return ok
			},
			Store: pendingBlocks,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*events.SyncedClusterBlock)
				if ok {
					core.metrics.MessageReceived(metrics.EngineClusterCompliance, metrics.MessageSyncedClusterBlock)
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
					core.metrics.MessageReceived(metrics.EngineClusterCompliance, metrics.MessageClusterBlockVote)
				}
				return ok
			},
			Store: pendingVotes,
		},
	)

	eng := &Engine{
		unit:                       engine.NewUnit(),
		lm:                         lifecycle.NewLifecycleManager(),
		log:                        engineLog,
		metrics:                    core.metrics,
		me:                         me,
		headers:                    core.headers,
		payloads:                   payloads,
		state:                      state,
		core:                       core,
		pendingBlocks:              pendingBlocks,
		pendingVotes:               pendingVotes,
		messageHandler:             handler,
		finalizationEventsNotifier: engine.NewNotifier(),
		con:                        nil,
		cluster:                    currentCluster,
	}

	chainID, err := core.state.Params().ChainID()
	if err != nil {
		return nil, fmt.Errorf("could not get chain ID: %w", err)
	}

	// register network conduit
	conduit, err := net.Register(engine.ChannelConsensusCluster(chainID), eng)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	eng.con = conduit

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

// Ready returns a ready channel that is closed once the engine has fully
// started. For consensus engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	if e.core.hotstuff == nil {
		panic("must initialize compliance engine with hotstuff engine")
	}
	e.lm.OnStart(func() {
		e.unit.Launch(e.loop)
		e.unit.Launch(e.finalizationProcessingLoop)

		ctx, cancel := context.WithCancel(context.Background())
		signalerCtx, _ := irrecoverable.WithSignaler(ctx)
		e.stopHotstuff = cancel
		e.core.hotstuff.Start(signalerCtx)
		// wait for request handler to startup
		<-e.core.hotstuff.Ready()
	})
	return e.lm.Started()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	e.lm.OnStop(func() {
		e.log.Info().Msg("shutting down hotstuff eventloop")
		e.stopHotstuff()
		<-e.core.hotstuff.Done()
		e.log.Info().Msg("all components have been shut down")
		<-e.unit.Done()
	})
	return e.lm.Stopped()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	err := e.ProcessLocal(event)
	if err != nil {
		e.log.Fatal().Err(err).Msg("internal error processing event")
	}
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(channel network.Channel, originID flow.Identifier, event interface{}) {
	err := e.Process(channel, originID, event)
	if err != nil {
		e.log.Fatal().Err(err).Msg("internal error processing event")
	}
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.messageHandler.Process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
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

func (e *Engine) loop() {
	for {
		select {
		case <-e.unit.Quit():
			return
		case <-e.messageHandler.GetNotifier():
			err := e.processAvailableMessages()
			if err != nil {
				e.log.Fatal().Err(err).Msg("internal error processing queued message")
			}
		}
	}
}

func (e *Engine) processAvailableMessages() error {

	for {
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

	// TODO: this is a hot-fix to mitigate the effects of the following Unicast call blocking occasionally
	e.unit.Launch(func() {
		// send the vote the desired recipient
		err := e.con.Unicast(vote, recipientID)
		if err != nil {
			log.Warn().Err(err).Msg("could not send vote")
			return
		}
		e.metrics.MessageSent(metrics.EngineClusterCompliance, metrics.MessageClusterBlockVote)
		log.Info().Msg("collection vote transmitted")
	})

	return nil
}

// BroadcastProposalWithDelay submits a cluster block proposal (effectively a proposal
// for the next collection) to all the collection nodes in our cluster.
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
	//TODO clean this up - currently we set these fields in builder, then lose
	// them in HotStuff, then need to set them again here
	header.ChainID = parent.ChainID
	header.Height = parent.Height + 1

	log := e.log.With().
		Hex("block_id", logging.ID(header.ID())).
		Uint64("block_height", header.Height).
		Logger()

	log.Debug().Msg("preparing to broadcast proposal from hotstuff")

	// retrieve the payload for the block
	payload, err := e.payloads.ByBlockID(header.ID())
	if err != nil {
		return fmt.Errorf("could not get payload for block: %w", err)
	}

	log = log.With().Int("collection_size", payload.Collection.Len()).Logger()

	// retrieve all collection nodes in our cluster
	recipients, err := e.state.Final().Identities(filter.And(
		filter.In(e.cluster),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	))
	if err != nil {
		return fmt.Errorf("could not get cluster members: %w", err)
	}

	e.unit.LaunchAfter(delay, func() {

		go e.core.hotstuff.SubmitProposal(header, parent.View)

		// create the proposal message for the collection
		msg := &messages.ClusterBlockProposal{
			Header:  header,
			Payload: payload,
		}

		err := e.con.Publish(msg, recipients.NodeIDs()...)
		if errors.Is(err, network.EmptyTargetList) {
			return
		}
		if err != nil {
			log.Error().Err(err).Msg("could not broadcast proposal")
			return
		}

		log.Debug().
			Str("recipients", fmt.Sprintf("%v", recipients.NodeIDs())).
			Msg("broadcast proposal from hotstuff")

		e.metrics.MessageSent(metrics.EngineClusterCompliance, metrics.MessageClusterBlockProposal)
		block := &cluster.Block{
			Header:  header,
			Payload: payload,
		}
		e.core.collectionMetrics.ClusterBlockProposed(block)
	})

	return nil
}

// BroadcastProposal will propagate a block proposal to all non-local consensus nodes.
// Note the header has incomplete fields, because it was converted from a hotstuff.
func (e *Engine) BroadcastProposal(header *flow.Header) error {
	return e.BroadcastProposalWithDelay(header, 0)
}

// OnFinalizedBlock implements the `OnFinalizedBlock` callback from the `hotstuff.FinalizationConsumer`
//  (1) Informs sealing.Core about finalization of respective block.
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnFinalizedBlock(block *model.Block) {
	if e.finalizedView.Set(block.View) {
		e.finalizationEventsNotifier.Notify()
	}
}

// finalizationProcessingLoop is a separate goroutine that performs processing of finalization events
func (e *Engine) finalizationProcessingLoop() {
	finalizationNotifier := e.finalizationEventsNotifier.Channel()
	for {
		select {
		case <-e.unit.Quit():
			return
		case <-finalizationNotifier:
			e.core.ProcessFinalizedView(e.finalizedView.Value())
		}
	}
}
