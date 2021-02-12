package compliance

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/fifoqueue"
	"github.com/onflow/flow-go/utils/logging"
)

type Event struct {
	OriginID flow.Identifier
	Msg      interface{}
}

type (
	EventSink chan *Event // Channel to push pending events
)

// defaultBlockQueueCapacity maximum capacity of block proposals queue
const defaultBlockQueueCapacity = 10000

// defaultVoteQueueCapacity maximum capacity of block votes queue
const defaultVoteQueueCapacity = 10000

// Engine is a wrapper struct for `Core` which implements consensus algorithm.
// Engine is responsible for handling incoming messages, queueing for processing, broadcasting proposals.
type Engine struct {
	unit             *engine.Unit
	log              zerolog.Logger
	mempool          module.MempoolMetrics
	metrics          module.EngineMetrics
	me               module.Local
	headers          storage.Headers
	payloads         storage.Payloads
	tracer           module.Tracer
	state            protocol.State
	prov             network.Engine
	core             *Core
	pendingEventSink EventSink
	blockSink        EventSink
	voteSink         EventSink
	pendingBlocks    *fifoqueue.FifoQueue
	pendingVotes     *fifoqueue.FifoQueue
	con              network.Conduit
}

func NewEngine(
	net module.Network,
	me module.Local,
	prov network.Engine,
	core *Core) (*Engine, error) {
	e := &Engine{
		unit:             engine.NewUnit(),
		log:              core.log,
		me:               me,
		mempool:          core.mempool,
		metrics:          core.metrics,
		headers:          core.headers,
		payloads:         core.payloads,
		state:            core.state,
		tracer:           core.tracer,
		prov:             prov,
		core:             core,
		pendingEventSink: make(EventSink),
		blockSink:        make(EventSink),
		voteSink:         make(EventSink),
	}

	var err error
	// register the core with the network layer and store the conduit
	e.con, err = net.Register(engine.ConsensusCommittee, e)
	if err != nil {
		return nil, fmt.Errorf("could not register core: %w", err)
	}

	// FIFO queue for block proposals
	e.pendingBlocks, err = fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultBlockQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) { e.mempool.MempoolEntries(metrics.ResourceBlockProposalQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound receipts: %w", err)
	}

	// FIFO queue for block votes
	e.pendingVotes, err = fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultVoteQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) { e.mempool.MempoolEntries(metrics.ResourceBlockVoteQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound approvals: %w", err)
	}

	return e, nil
}

// WithConsensus adds the consensus algorithm to the engine. This must be
// called before the engine can start.
func (e *Engine) WithConsensus(hot module.HotStuff) *Engine {
	e.core.hotstuff = hot
	return e
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For consensus engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	if e.core.hotstuff == nil {
		panic("must initialize compliance engine with hotstuff engine")
	}
	var wg sync.WaitGroup
	wg.Add(2)
	e.unit.Launch(func() {
		wg.Done()
		e.processEvents()
	})
	e.unit.Launch(func() {
		wg.Done()
		e.consumeEvents()
	})
	return e.unit.Ready(func() {
		<-e.core.hotstuff.Ready()
		wg.Wait()
	})
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(func() {
		e.log.Debug().Msg("shutting down hotstuff eventloop")
		<-e.core.hotstuff.Done()
		e.log.Debug().Msg("all components have been shut down")
	})
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	err := e.Process(originID, event)
	if err != nil {
		engine.LogError(e.log, err)
	}
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	e.pendingEventSink <- &Event{
		OriginID: originID,
		Msg:      event,
	}
	return nil
}

// processEvents is processor of pending events which drives events from networking layer to business logic in `Core`.
// Effectively consumes messages from networking layer and dispatches them into corresponding sinks which are connected with `Core`.
// Should be run as a separate goroutine.
func (e *Engine) processEvents() {
	// takes pending event from one of the queues
	// nil sink means nothing to send, this prevents blocking on select
	fetchEvent := func() (*Event, EventSink, *fifoqueue.FifoQueue) {
		if val, ok := e.pendingBlocks.Front(); ok {
			return val.(*Event), e.blockSink, e.pendingBlocks
		}
		if val, ok := e.pendingVotes.Front(); ok {
			return val.(*Event), e.voteSink, e.pendingVotes
		}
		return nil, nil, nil
	}

	for {
		pendingEvent, sink, fifo := fetchEvent()
		select {
		case event := <-e.pendingEventSink:
			e.processPendingEvent(event)
		case sink <- pendingEvent:
			fifo.Pop()
			continue
		case <-e.unit.Quit():
			return
		}
	}
}

// processPendingEvent saves pending event in corresponding queue for further processing by `Core`.
// While this function runs in separate goroutine it shouldn't do heavy processing to maintain efficient data polling/pushing.
func (e *Engine) processPendingEvent(event *Event) {
	// skip any message as long as we don't have the dependencies
	if e.core.hotstuff == nil || e.core.sync == nil {
		return
	}

	switch event.Msg.(type) {
	case *events.SyncedBlock:
		e.metrics.MessageReceived(metrics.EngineCompliance, metrics.MessageSyncedBlock)
		// a block that is synced has to come locally, from the synchronization engine
		// the block itself will contain the proposer to indicate who created it
		if event.OriginID != e.me.NodeID() {
			log.Warn().Msgf("synced block with non-local origin (local: %x, origin: %x)", e.me.NodeID(), event.OriginID)
			return
		}
		e.pendingBlocks.Push(event)
	case *messages.BlockProposal:
		e.metrics.MessageReceived(metrics.EngineCompliance, metrics.MessageBlockProposal)
		e.pendingBlocks.Push(event)
	case *messages.BlockVote:
		e.metrics.MessageReceived(metrics.EngineCompliance, metrics.MessageBlockVote)
		e.pendingVotes.Push(event)
	}
}

// consumeEvents consumes events that are ready to be processed.
func (e *Engine) consumeEvents() {
	processBlock := func(event *Event) error {
		var err error
		switch t := event.Msg.(type) {
		case *events.SyncedBlock:
			proposal := &messages.BlockProposal{
				Header:  t.Block.Header,
				Payload: t.Block.Payload,
			}
			err = e.core.OnBlockProposal(event.OriginID, proposal)
			e.metrics.MessageHandled(metrics.EngineCompliance, metrics.MessageSyncedBlock)

		case *messages.BlockProposal:
			err = e.core.OnBlockProposal(event.OriginID, t)
			e.metrics.MessageHandled(metrics.EngineCompliance, metrics.MessageBlockProposal)
		}
		return err
	}

	for {
		var err error
		select {
		case event := <-e.blockSink:
			err = processBlock(event)
		case event := <-e.voteSink:
			err = e.core.OnBlockVote(event.OriginID, event.Msg.(*messages.BlockVote))
			e.metrics.MessageHandled(metrics.EngineCompliance, metrics.MessageBlockVote)
		case <-e.unit.Quit():
			return
		}
		if err != nil {
			// Public methods of `Core` are supposed to handle all errors internally.
			// Here if error happens it means that internal state is corrupted or we have caught
			// exception while processing. In such case best just to abort the node.
			e.log.Fatal().Err(err).Msgf("fatal internal error in matching core logic")
		}
	}
}

// SendVote will send a vote to the desired node.
func (e *Engine) SendVote(blockID flow.Identifier, view uint64, sigData []byte, recipientID flow.Identifier) error {

	log := e.log.With().
		Hex("block_id", blockID[:]).
		Uint64("block_view", view).
		Hex("recipient_id", recipientID[:]).
		Logger()
	log.Info().Msg("processing vote transmission request from hotstuff")

	// build the vote message
	vote := &messages.BlockVote{
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
		e.metrics.MessageSent(metrics.EngineCompliance, metrics.MessageBlockVote)
		log.Info().Msg("block vote transmitted")
	})

	return nil
}

// BroadcastProposalWithDelay will propagate a block proposal to all non-local consensus nodes.
// Note the header has incomplete fields, because it was converted from a hotstuff.
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
	header.ChainID = parent.ChainID
	header.Height = parent.Height + 1

	// retrieve the payload for the block
	payload, err := e.payloads.ByBlockID(header.ID())
	if err != nil {
		return fmt.Errorf("could not retrieve payload for proposal: %w", err)
	}

	log := e.log.With().
		Str("chain_id", header.ChainID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", logging.Entity(header)).
		Hex("parent_id", header.ParentID[:]).
		Hex("payload_hash", header.PayloadHash[:]).
		Int("gaurantees_count", len(payload.Guarantees)).
		Int("seals_count", len(payload.Seals)).
		Int("receipts_count", len(payload.Receipts)).
		Time("timestamp", header.Timestamp).
		Hex("proposer", header.ProposerID[:]).
		Int("num_signers", len(header.ParentVoterIDs)).
		Dur("delay", delay).
		Logger()

	log.Info().Msg("processing proposal broadcast request from hotstuff")

	for _, g := range payload.Guarantees {
		if span, ok := e.tracer.GetSpan(g.CollectionID, trace.CONProcessCollection); ok {
			childSpan := e.tracer.StartSpanFromParent(span, trace.CONCompBroadcastProposalWithDelay)
			defer childSpan.Finish()
		}
	}

	// retrieve all consensus nodes without our ID
	recipients, err := e.state.AtBlockID(header.ParentID).Identities(filter.And(
		filter.HasRole(flow.RoleConsensus),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	))
	if err != nil {
		return fmt.Errorf("could not get consensus recipients: %w", err)
	}

	e.unit.LaunchAfter(delay, func() {

		go e.core.hotstuff.SubmitProposal(header, parent.View)

		// NOTE: some fields are not needed for the message
		// - proposer ID is conveyed over the network message
		// - the payload hash is deduced from the payload
		proposal := &messages.BlockProposal{
			Header:  header,
			Payload: payload,
		}

		// broadcast the proposal to consensus nodes
		err = e.con.Publish(proposal, recipients.NodeIDs()...)
		if err != nil {
			log.Error().Err(err).Msg("could not send proposal message")
		}

		e.metrics.MessageSent(metrics.EngineCompliance, metrics.MessageBlockProposal)

		log.Info().Msg("block proposal broadcasted")

		// submit the proposal to the provider engine to forward it to other
		// node roles
		e.prov.SubmitLocal(proposal)
	})

	return nil
}

// BroadcastProposal will propagate a block proposal to all non-local consensus nodes.
// Note the header has incomplete fields, because it was converted from a hotstuff.
func (e *Engine) BroadcastProposal(header *flow.Header) error {
	return e.BroadcastProposalWithDelay(header, 0)
}
