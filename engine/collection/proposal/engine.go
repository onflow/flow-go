// Package proposal implements an engine for proposing and guaranteeing
// collections and submitting them to consensus nodes.
package proposal

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/network"
	clusterkv "github.com/dapperlabs/flow-go/state/cluster"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine is the collection proposal engine, which packages pending
// transactions into collections and sends them to consensus nodes.
type Engine struct {
	unit           *engine.Unit
	log            zerolog.Logger
	colMetrics     module.CollectionMetrics
	engMetrics     module.EngineMetrics
	mempoolMetrics module.MempoolMetrics
	con            network.Conduit
	me             module.Local
	protoState     protocol.State  // flow-wide protocol chain state
	clusterState   clusterkv.State // cluster-specific chain state
	validator      module.TransactionValidator
	pool           mempool.Transactions
	transactions   storage.Transactions
	headers        storage.Headers
	payloads       storage.ClusterPayloads
	pending        module.PendingClusterBlockBuffer // pending block cache
	participants   flow.IdentityList                // consensus participants in our cluster

	hotstuff module.HotStuff
}

func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	colMetrics module.CollectionMetrics,
	engMetrics module.EngineMetrics,
	mempoolMetrics module.MempoolMetrics,
	protoState protocol.State,
	clusterState clusterkv.State,
	validator module.TransactionValidator,
	pool mempool.Transactions,
	transactions storage.Transactions,
	headers storage.Headers,
	payloads storage.ClusterPayloads,
	cache module.PendingClusterBlockBuffer,
) (*Engine, error) {

	participants, err := protocol.ClusterFor(protoState.Final(), me.NodeID())
	if err != nil {
		return nil, fmt.Errorf("could not get cluster participants: %w", err)
	}

	e := &Engine{
		unit:           engine.NewUnit(),
		log:            log.With().Str("engine", "proposal").Logger(),
		colMetrics:     colMetrics,
		engMetrics:     engMetrics,
		mempoolMetrics: mempoolMetrics,
		me:             me,
		protoState:     protoState,
		clusterState:   clusterState,
		validator:      validator,
		pool:           pool,
		transactions:   transactions,
		headers:        headers,
		payloads:       payloads,
		pending:        cache,
		participants:   participants,
	}

	con, err := net.Register(engine.ProtocolClusterConsensus, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con

	return e, nil
}

// WithConsensus adds the consensus algorithm to the engine. This must be
// called before the engine can start.
func (e *Engine) WithConsensus(hot module.HotStuff) *Engine {
	e.hotstuff = hot
	return e
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For proposal engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	if e.hotstuff == nil {
		panic("cannot start proposal engine without consensus algorithm")
	}

	return e.unit.Ready(func() {
		<-e.hotstuff.Ready()
	})
}

// Done returns a done channel that is closed once the engine has fully stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(func() {
		<-e.hotstuff.Done()
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
	e.unit.Launch(func() {
		err := e.process(originID, event)
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

// process processes events for the proposal engine on the collection node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {

	//TODO currently it is possible for network messages to be received
	// and passed to the engine before the engine has been setup
	// (ie had Ready called). This is a quickfix to get around the issue
	// but ultimately we should start receiving over the network only
	// once all the engines are ready.
	if e.hotstuff == nil {
		return fmt.Errorf("missing hotstuff dependency")
	}

	// just process one event at a time for now

	switch ev := event.(type) {
	case *messages.ClusterBlockProposal:
		e.before(metrics.MessageClusterBlockProposal)
		defer e.after(metrics.MessageClusterBlockProposal)
		return e.onBlockProposal(originID, ev)
	case *messages.ClusterBlockVote:
		e.before(metrics.MessageClusterBlockVote)
		defer e.after(metrics.MessageClusterBlockVote)
		return e.onBlockVote(originID, ev)
	case *messages.ClusterBlockRequest:
		e.before(metrics.MessageClusterBlockRequest)
		defer e.after(metrics.MessageClusterBlockRequest)
		return e.onBlockRequest(originID, ev)
	case *messages.ClusterBlockResponse:
		e.before(metrics.MessageClusterBlockResponse)
		defer e.after(metrics.MessageClusterBlockResponse)
		return e.onBlockResponse(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *Engine) before(msg string) {
	e.engMetrics.MessageReceived(metrics.EngineProposal, msg)
	e.unit.Lock()
}

func (e *Engine) after(msg string) {
	e.unit.Unlock()
	e.engMetrics.MessageHandled(metrics.EngineProposal, msg)
}

// SendVote will send a vote to the desired node.
func (e *Engine) SendVote(blockID flow.Identifier, view uint64, sigData []byte, recipientID flow.Identifier) error {

	// build the vote message
	vote := &messages.ClusterBlockVote{
		BlockID: blockID,
		View:    view,
		SigData: sigData,
	}

	err := e.con.Submit(vote, recipientID)
	if err != nil {
		return fmt.Errorf("could not send vote: %w", err)
	}

	e.engMetrics.MessageSent(metrics.EngineProposal, metrics.MessageClusterBlockVote)

	return nil
}

// BroadcastProposal submits a cluster block proposal (effectively a proposal
// for the next collection) to all the collection nodes in our cluster.
func (e *Engine) BroadcastProposal(header *flow.Header) error {
	return e.BroadcastProposalWithDelay(header, 0)
}

// BroadcastProposalWithDelay submits a cluster block proposal (effectively a proposal
// for the next collection) to all the collection nodes in our cluster.
func (e *Engine) BroadcastProposalWithDelay(header *flow.Header, delay time.Duration) error {

	log := e.log.With().
		Hex("block_id", logging.ID(header.ID())).
		Uint64("block_height", header.Height).
		Logger()

	log.Debug().Msg("preparing to broadcast proposal from hotstuff")

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

	// retrieve the payload for the block
	payload, err := e.payloads.ByBlockID(header.ID())
	if err != nil {
		return fmt.Errorf("could not get payload for block: %w", err)
	}

	log = log.With().Int("collection_size", payload.Collection.Len()).Logger()

	// retrieve all collection nodes in our cluster
	recipients, err := e.protoState.Final().Identities(filter.And(
		filter.In(e.participants),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	))
	if err != nil {
		return fmt.Errorf("could not get cluster members: %w", err)
	}

	e.unit.LaunchAfter(delay, func() {
		// create the proposal message for the collection
		msg := &messages.ClusterBlockProposal{
			Header:  header,
			Payload: payload,
		}

		err = e.con.Submit(msg, recipients.NodeIDs()...)
		if err != nil {
			log.Error().Err(err).Msg("could not broadcast proposal")
			return
		}

		log.Debug().
			Str("recipients", fmt.Sprintf("%v", recipients.NodeIDs())).
			Msg("broadcast proposal from hotstuff")

		e.engMetrics.MessageSent(metrics.EngineProposal, metrics.MessageClusterBlockProposal)
		e.colMetrics.ClusterBlockProposed(cluster.FromProposal(msg))
	})

	return nil
}

// onBlockProposal handles proposals for new blocks.
func (e *Engine) onBlockProposal(originID flow.Identifier, proposal *messages.ClusterBlockProposal) error {

	header := proposal.Header
	payload := proposal.Payload

	log := e.log.With().
		Hex("origin_id", logging.ID(originID)).
		Hex("block_id", logging.ID(header.ID())).
		Uint64("block_height", header.Height).
		Int("collection_size", payload.Collection.Len()).
		Str("chain_id", header.ChainID).
		Hex("parent_id", logging.ID(header.ParentID)).
		Logger()

	log.Debug().Msg("received proposal")

	e.prunePendingCache()

	// retrieve the parent block
	// if the parent is not in storage, it has not yet been processed
	parent, err := e.headers.ByBlockID(header.ParentID)
	if errors.Is(err, storage.ErrNotFound) {
		log.Debug().Msg("processing block as pending")
		return e.processPendingProposal(originID, proposal)
	}
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	// validate any transactions we haven't yet seen
	var merr *multierror.Error
	for _, tx := range payload.Collection.Transactions {
		if !e.pool.Has(tx.ID()) {
			err = e.validator.ValidateTransaction(tx)
			if err != nil {
				merr = multierror.Append(merr, err)
			}
		}
	}
	if err := merr.ErrorOrNil(); err != nil {
		return fmt.Errorf("cannot validate block proposal (id=%x) with invalid transactions: %w", header.ID(), err)
	}

	// create a cluster block representing the proposal
	block := cluster.Block{
		Header:  header,
		Payload: payload,
	}

	// extend the state with the proposal -- if it is an invalid extension,
	// we will throw an error here
	err = e.clusterState.Mutate().Extend(&block)
	if err != nil {
		return fmt.Errorf("could not extend cluster state: %w", err)
	}

	// submit the proposal to hotstuff for processing
	e.hotstuff.SubmitProposal(header, parent.View)

	// report proposed (case that we are follower)
	e.colMetrics.ClusterBlockProposed(cluster.FromProposal(proposal))

	blockID := header.ID()

	children, ok := e.pending.ByParentID(blockID)
	if !ok {
		return nil
	}

	e.log.Debug().
		Int("children", len(children)).
		Msg("processing pending children")

	var result *multierror.Error
	for _, child := range children {
		proposal := &messages.ClusterBlockProposal{
			Header:  child.Header,
			Payload: child.Payload,
		}
		err := e.onBlockProposal(child.OriginID, proposal)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	// remove children from cache
	e.pending.DropForParent(blockID)

	return result.ErrorOrNil()
}

// onBlockVote handles votes for blocks by passing them to the core consensus
// algorithm
func (e *Engine) onBlockVote(originID flow.Identifier, vote *messages.ClusterBlockVote) error {
	e.hotstuff.SubmitVote(originID, vote.BlockID, vote.View, vote.SigData)
	return nil
}

// onBlockRequest handles requests from other nodes for blocks we have.
// We always respond to these requests if we have the block in question.
func (e *Engine) onBlockRequest(originID flow.Identifier, req *messages.ClusterBlockRequest) error {

	log := e.log.With().
		Hex("origin_id", logging.ID(originID)).
		Hex("req_block_id", logging.ID(req.BlockID)).
		Logger()

	log.Debug().Msg("received block request")

	// retrieve the block header
	header, err := e.headers.ByBlockID(req.BlockID)
	if err != nil {
		return fmt.Errorf("could not find requested block: %w", err)
	}

	// retrieve the block payload
	payload, err := e.payloads.ByBlockID(header.ID())
	if err != nil {
		return fmt.Errorf("could not find requested block: %w", err)
	}

	proposal := &messages.ClusterBlockProposal{
		Header:  header,
		Payload: payload,
	}

	res := &messages.ClusterBlockResponse{
		Proposal: proposal,
		Nonce:    req.Nonce,
	}

	err = e.con.Submit(res, originID)
	if err != nil {
		return fmt.Errorf("could not send block response: %w", err)
	}

	e.engMetrics.MessageSent(metrics.EngineProposal, metrics.MessageClusterBlockResponse)

	return nil
}

// onBlockResponse handles responses to queries for particular blocks we have made.
func (e *Engine) onBlockResponse(originID flow.Identifier, res *messages.ClusterBlockResponse) error {

	header := res.Proposal.Header
	e.log.Debug().
		Hex("origin_id", logging.ID(originID)).
		Hex("block_id", logging.ID(header.ID())).
		Uint64("block_height", header.Height).
		Msg("received block response")

	// process the block response as we would a regular proposal
	err := e.onBlockProposal(originID, res.Proposal)
	if err != nil {
		return fmt.Errorf("could not process block response: %w", err)
	}

	return nil
}

// processPendingProposal handles proposals where the parent is missing.
func (e *Engine) processPendingProposal(originID flow.Identifier, proposal *messages.ClusterBlockProposal) error {

	pendingBlock := &cluster.PendingBlock{
		OriginID: originID,
		Header:   proposal.Header,
		Payload:  proposal.Payload,
	}

	// cache the block
	e.pending.Add(pendingBlock)

	// request the missing block - typically this is the pending block's direct
	// parent, but in general we walk the block's pending ancestors until we find
	// the oldest one that is missing its parent.
	missingID := proposal.Header.ParentID
	for {
		nextBlock, ok := e.pending.ByID(missingID)
		if !ok {
			break
		}
		missingID = nextBlock.Header.ParentID
	}

	req := &messages.ClusterBlockRequest{
		BlockID: missingID,
		Nonce:   rand.Uint64(),
	}

	// select a set of other nodes to request the missing block from
	// always including the sender of the pending block
	recipients := e.participants.
		Filter(filter.Not(filter.HasNodeID(originID, e.me.NodeID()))).
		Sample(1).
		NodeIDs()
	recipients = append(recipients, originID)

	err := e.con.Submit(req, recipients...)
	if err != nil {
		return fmt.Errorf("could not send block request: %w", err)
	}

	e.engMetrics.MessageSent(metrics.EngineProposal, metrics.MessageClusterBlockRequest)

	// NOTE: at this point, if we can't find the parent, we should probably think about a way
	// to blacklist him, as this can be exploited by sending us lots of children without parent;
	// a second mitigation strategy is to put a strict limit on children we cache, and possibly a
	// limit on children we cache coming from a single other node

	return nil
}

// prunePendingCache prunes the pending block cache.
func (e *Engine) prunePendingCache() {

	// retrieve the finalized height
	final, err := e.clusterState.Final().Head()
	if err != nil {
		e.log.Warn().Err(err).Msg("could not get finalized head to prune pending blocks")
		return
	}

	// remove all pending blocks at or below the finalized height
	e.pending.PruneByHeight(final.Height)

	// always record the metric
	e.mempoolMetrics.MempoolEntries(metrics.ResourceClusterProposal, e.pending.Size())
}
