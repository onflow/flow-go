// Package proposal implements an engine for proposing and guaranteeing
// collections and submitting them to consensus nodes.
package proposal

import (
	"errors"
	"fmt"
	"math/rand"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	clustermodel "github.com/dapperlabs/flow-go/model/cluster"
	coldstuffmodel "github.com/dapperlabs/flow-go/model/coldstuff"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/cluster"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine is the collection proposal engine, which packages pending
// transactions into collections and sends them to consensus nodes.
type Engine struct {
	unit         *engine.Unit
	log          zerolog.Logger
	metrics      module.Metrics
	con          network.Conduit
	me           module.Local
	protoState   protocol.State // flow-wide protocol chain state
	clusterState cluster.State  // cluster-specific chain state
	ingest       network.Engine
	pool         mempool.Transactions
	transactions storage.Transactions
	headers      storage.Headers
	payloads     storage.ClusterPayloads
	cache        module.PendingClusterBlockBuffer // pending block cache
	participants flow.IdentityList                // consensus participants in our cluster

	coldstuff module.ColdStuff
}

func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	protoState protocol.State,
	clusterState cluster.State,
	metrics module.Metrics,
	ingest network.Engine,
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
		unit:         engine.NewUnit(),
		log:          log.With().Str("engine", "proposal").Logger(),
		me:           me,
		protoState:   protoState,
		clusterState: clusterState,
		metrics:      metrics,
		ingest:       ingest,
		pool:         pool,
		transactions: transactions,
		headers:      headers,
		payloads:     payloads,
		cache:        cache,
		participants: participants,
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
func (e *Engine) WithConsensus(cold module.ColdStuff) *Engine {
	e.coldstuff = cold
	return e
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For proposal engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	if e.coldstuff == nil {
		panic("cannot start proposal engine without consensus algorithm")
	}

	return e.unit.Ready(func() {
		<-e.coldstuff.Ready()
	})
}

// Done returns a done channel that is closed once the engine has fully stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(func() {
		<-e.coldstuff.Done()
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
		//TODO currently it is possible for network messages to be received
		// and passed to the engine before the engine has been setup
		// (ie had Ready called). This is a quickfix to get around the issue
		// but ultimately we should start receiving over the network only
		// once all the engines are ready.

		if e.coldstuff == nil {
			return fmt.Errorf("missing coldstuff dependency")
		}
		return e.process(originID, event)
	})
}

// process processes events for the proposal engine on the collection node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *messages.ClusterBlockProposal:
		return e.onBlockProposal(originID, ev)
	case *messages.ClusterBlockVote:
		return e.onBlockVote(originID, ev)
	case *messages.ClusterBlockRequest:
		return e.onBlockRequest(originID, ev)
	case *messages.ClusterBlockResponse:
		return e.onBlockResponse(originID, ev)
	case *coldstuffmodel.Commit:
		return e.onBlockCommit(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
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

	return nil
}

// BroadcastProposal submits a cluster block proposal (effectively a proposal
// for the next collection) to all the collection nodes in our cluster.
func (e *Engine) BroadcastProposal(header *flow.Header) error {

	// first, check that we are the proposer of the block
	if header.ProposerID != e.me.NodeID() {
		return fmt.Errorf("cannot broadcast proposal with non-local proposer (%x)", header.ProposerID)
	}

	// retrieve the payload for the block
	payload, err := e.payloads.ByBlockID(header.ID())
	if err != nil {
		return fmt.Errorf("could not get payload for block: %w", err)
	}

	// retrieve all collection nodes in our cluster
	recipients, err := e.protoState.Final().Identities(
		filter.In(e.participants),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	)
	if err != nil {
		return fmt.Errorf("could not get cluster members: %w", err)
	}

	// create the proposal message for the collection
	msg := &messages.ClusterBlockProposal{
		Header:  header,
		Payload: payload,
	}

	err = e.con.Submit(msg, recipients.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not broadcast proposal: %w", err)
	}

	final, _ := e.clusterState.Final().Head()

	e.log.Debug().
		Hex("block_id", logging.ID(header.ID())).
		Uint64("block_height", header.Height).
		Hex("parent_id", logging.ID(header.ParentID)).
		Hex("final_id", logging.ID(final.ID())).
		Uint64("final_height", final.Height).
		Int("collection_size", len(payload.Collection.Transactions)).
		Msg("submitted proposal")

	e.metrics.StartCollectionToGuarantee(payload.Collection.Light())

	return nil
}

// BroadcastCommit broadcasts a commit message to all collection nodes in our
// cluster.
func (e *Engine) BroadcastCommit(commit *coldstuffmodel.Commit) error {

	// retrieve all collection nodes in our cluster
	recipients, err := e.protoState.Final().Identities(
		filter.In(e.participants),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	)
	if err != nil {
		return fmt.Errorf("could not get cluster members: %w", err)
	}

	err = e.con.Submit(commit, recipients.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not send commit message: %w", err)
	}

	return err
}

// onBlockProposal handles proposals for new blocks.
func (e *Engine) onBlockProposal(originID flow.Identifier, proposal *messages.ClusterBlockProposal) error {

	final, _ := e.clusterState.Final().Head()

	e.log.Debug().
		Hex("block_id", logging.ID(proposal.Header.ID())).
		Uint64("block_height", proposal.Header.Height).
		Hex("parent_id", logging.ID(proposal.Header.ParentID)).
		Hex("final_id", logging.ID(final.ID())).
		Uint64("final_height", final.Height).
		Msg("received proposal")

	// retrieve the parent block
	parent, err := e.headers.ByBlockID(proposal.Header.ParentID)
	if errors.Is(err, storage.ErrNotFound) {
		return e.processPendingProposal(originID, proposal)
	}
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	blockID := proposal.Header.ID()
	collection := proposal.Payload.Collection

	// validate any transactions we haven't yet seen
	var merr *multierror.Error
	for _, tx := range collection.Transactions {
		if !e.pool.Has(tx.ID()) {
			err = e.ingest.ProcessLocal(tx)
			if err != nil {
				merr = multierror.Append(merr, err)
			}
		}
	}
	if err := merr.ErrorOrNil(); err != nil {
		return fmt.Errorf("cannot validate block proposal (id=%x) with invalid transactions: %w", proposal.Header.ID(), err)
	}

	// store the payload
	err = e.payloads.Store(proposal.Header, proposal.Payload)
	if err != nil {
		return fmt.Errorf("could not store payload: %w", err)
	}

	// store the header
	err = e.headers.Store(proposal.Header)
	if err != nil {
		return fmt.Errorf("could not store header: %w", err)
	}

	// ensure the block is a valid extension of cluster state
	err = e.clusterState.Mutate().Extend(blockID)
	if err != nil {
		return fmt.Errorf("could not extend cluster state: %w", err)
	}

	// submit the proposal to hotstuff for processing
	e.coldstuff.SubmitProposal(proposal.Header, parent.View)

	children, ok := e.cache.ByParentID(blockID)
	if !ok {
		return nil
	}
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
	e.cache.DropForParent(blockID)

	return result.ErrorOrNil()
}

// onBlockVote handles votes for blocks by passing them to the core consensus
// algorithm
func (e *Engine) onBlockVote(originID flow.Identifier, vote *messages.ClusterBlockVote) error {
	e.coldstuff.SubmitVote(originID, vote.BlockID, vote.View, vote.SigData)
	return nil
}

// onBlockRequest handles requests from other nodes for blocks we have.
// We always respond to these requests if we have the block in question.
func (e *Engine) onBlockRequest(originID flow.Identifier, req *messages.ClusterBlockRequest) error {

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

	return nil
}

// onBlockResponse handles responses to queries for particular blocks we have made.
func (e *Engine) onBlockResponse(originID flow.Identifier, res *messages.ClusterBlockResponse) error {

	// process the block response as we would a regular proposal
	err := e.onBlockProposal(originID, res.Proposal)
	if err != nil {
		return fmt.Errorf("could not process block response: %w", err)
	}

	return nil
}

// onBlockCommit handles incoming block commits by passing them to the core
// consensus algorithm.
//
// NOTE: This is only necessary for ColdStuff and can be removed when we switch
// to HotStuff.
func (e *Engine) onBlockCommit(originID flow.Identifier, commit *coldstuffmodel.Commit) error {
	e.coldstuff.SubmitCommit(commit)
	return nil
}

// processPendingProposal handles proposals where the parent is missing.
func (e *Engine) processPendingProposal(originID flow.Identifier, proposal *messages.ClusterBlockProposal) error {

	pendingBlock := &clustermodel.PendingBlock{
		OriginID: originID,
		Header:   proposal.Header,
		Payload:  proposal.Payload,
	}

	// cache the block
	e.cache.Add(pendingBlock)

	// request the missing block - typically this is the pending block's direct
	// parent, but in general we walk the block's pending ancestors until we find
	// the oldest one that is missing its parent.
	missingID := proposal.Header.ParentID
	for {
		nextBlock, ok := e.cache.ByID(missingID)
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
		Sample(2).
		NodeIDs()
	recipients = append(recipients, originID)

	err := e.con.Submit(req, recipients...)
	if err != nil {
		return fmt.Errorf("could not send block request: %w", err)
	}

	// NOTE: at this point, if we can't find the parent, we should probably think about a way
	// to blacklist him, as this can be exploited by sending us lots of children without parent;
	// a second mitigation strategy is to put a strict limit on children we cache, and possibly a
	// limit on children we cache coming from a single other node

	return nil
}
