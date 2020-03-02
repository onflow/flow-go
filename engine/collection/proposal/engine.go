// Package proposal implements an engine for proposing and guaranteeing
// collections and submitting them to consensus nodes.
package proposal

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/cluster"
	model "github.com/dapperlabs/flow-go/model/coldstuff"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// Engine is the collection proposal engine, which packages pending
// transactions into collections and sends them to consensus nodes.
type Engine struct {
	unit        *engine.Unit
	log         zerolog.Logger
	tracer      trace.Tracer
	con         network.Conduit
	me          module.Local
	state       protocol.State
	provider    network.Engine // provider engine to propagate guarantees
	pool        mempool.Transactions
	collections storage.Collections
	guarantees  storage.Guarantees
	headers     storage.Headers

	coldstuff module.ColdStuff
}

func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	state protocol.State,
	tracer trace.Tracer,
	provider network.Engine,
	pool mempool.Transactions,
	collections storage.Collections,
	guarantees storage.Guarantees,
	headers storage.Headers,
) (*Engine, error) {

	e := &Engine{
		unit:        engine.NewUnit(),
		log:         log.With().Str("engine", "proposal").Logger(),
		me:          me,
		state:       state,
		tracer:      tracer,
		provider:    provider,
		pool:        pool,
		collections: collections,
		guarantees:  guarantees,
		headers:     headers,
	}

	con, err := net.Register(engine.CollectionProposal, e)
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
	case *model.Commit:
		return e.onBlockCommit(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// SendVote will send a vote to the desired node.
func (e *Engine) SendVote(blockID flow.Identifier, view uint64, sig crypto.Signature, _ crypto.Signature, recipientID flow.Identifier) error {

	// build the vote message
	vote := &messages.ClusterBlockVote{
		BlockID:   blockID,
		View:      view,
		Signature: sig,
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
	// NOTE: relies on the fact that cluster payload hash is the ID of its collection
	collectionID := header.PayloadHash
	collection, err := e.collections.LightByID(collectionID)
	if err != nil {
		return fmt.Errorf("could not get payload for block: %w", err)
	}
	payload := cluster.Payload{Collection: *collection}

	// retrieve all collection nodes in our cluster
	// TODO filter by cluster
	recipients, err := e.state.Final().Identities(
		filter.HasRole(flow.RoleCollection),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	)
	if err != nil {
		return fmt.Errorf("could not get cluster members: %w", err)
	}

	// create the proposal message for the collection
	msg := &messages.ClusterBlockProposal{
		Header:  header,
		Payload: &payload,
	}

	err = e.con.Submit(msg, recipients.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not broadcast proposal: %w", err)
	}

	return nil
}

// BroadcastCommit broadcasts a commit message to all collection nodes in our
// cluster.
func (e *Engine) BroadcastCommit(commit *model.Commit) error {

	// retrieve all collection nodes in our cluster
	// TODO filter by cluster
	recipients, err := e.state.Final().Identities(
		filter.HasRole(flow.RoleCollection),
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

	// retrieve the parent block
	parent, err := e.headers.ByBlockID(proposal.Header.ParentID)
	// TODO handle block buffering https://github.com/dapperlabs/flow-go/issues/2408
	//if errors.Is(err, storage.ErrNotFound) {}
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	_ = parent

	// TODO handle missing transactions
	// TODO store block contents
	// TODO ensure block is valid extension of cluster state
	// TODO submit to coldstuff
	// TODO check for buffered descendants of the block

	return nil
}

// onBlockVote handles votes for blocks by passing them to the core consensus
// algorithm
func (e *Engine) onBlockVote(originID flow.Identifier, vote *messages.ClusterBlockVote) error {
	e.coldstuff.SubmitVote(originID, vote.BlockID, vote.View, vote.Signature, nil)
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
	collection, err := e.collections.LightByID(header.PayloadHash)
	if err != nil {
		return fmt.Errorf("could not find requested block: %w", err)
	}

	payload := &cluster.Payload{Collection: *collection}

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
func (e *Engine) onBlockCommit(originID flow.Identifier, commit *model.Commit) error {
	e.coldstuff.SubmitCommit(commit)
	return nil
}

// createProposal creates a new proposal
func (e *Engine) createProposal() error {
	if e.pool.Size() == 0 {
		return ErrEmptyTxpool
	}

	transactions := e.pool.All()
	coll := flow.CollectionFromTransactions(transactions)

	err := e.collections.Store(&coll)
	if err != nil {
		return fmt.Errorf("could not save proposed collection: %w", err)
	}

	guarantee := coll.Guarantee()

	trace.StartCollectionGuaranteeSpan(e.tracer, guarantee, transactions).
		SetTag("node_type", "collection").
		SetTag("node_id", e.me.NodeID().String())

	err = e.guarantees.Store(&guarantee)
	if err != nil {
		return fmt.Errorf("could not save proposed collection guarantee %s: %w", guarantee.ID(), err)
	}

	// Collection guarantee is saved, we can now delete Txs from the mem pool
	for _, tx := range transactions {
		e.pool.Rem(tx.ID())
		e.tracer.FinishSpan(tx.ID())
	}

	err = e.provider.ProcessLocal(&messages.SubmitCollectionGuarantee{Guarantee: guarantee})
	if err != nil {
		return fmt.Errorf("could not submit collection guarantee: %w", err)
	}

	return nil
}
