// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"errors"
	"fmt"
	"math/rand"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// Engine is the consensus engine, responsible for handling communication for
// the embedded consensus algorithm.
type Engine struct {
	unit     *engine.Unit   // used to control startup/shutdown
	log      zerolog.Logger // used to log relevant actions with context
	me       module.Local
	state    protocol.State
	headers  storage.Headers
	payloads storage.Payloads
	hotstuff hotstuff.HotStuff
	con      network.Conduit
	cache    map[flow.Identifier][]cacheItem
}

type cacheItem struct {
	OriginID flow.Identifier
	Proposal *messages.BlockProposal
}

// New creates a new consensus propagation engine.
func New(log zerolog.Logger, net module.Network, me module.Local, state protocol.State, headers storage.Headers, payloads storage.Payloads, hotstuff hotstuff.HotStuff) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:     engine.NewUnit(),
		log:      log.With().Str("engine", "consensus").Logger(),
		me:       me,
		state:    state,
		headers:  headers,
		payloads: payloads,
		hotstuff: hotstuff,
		cache:    make(map[flow.Identifier][]cacheItem),
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(engine.ProtocolConsensus, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the consensus, we consider it ready right away.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
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

// SendVote will send a vote to the desired node.
func (e *Engine) SendVote(vote *types.Vote, recipientID flow.Identifier) error {

	// check that we are sending our own vote
	if vote.Signature.SignerID != e.me.NodeID() {
		return fmt.Errorf("can only send votes signed locally")
	}

	// NOTE: the signer ID will be conveyed through the message origin ID;
	// we can thus leave it out of the message to conserve bandwidth

	// NOTE: the view is redundant because we can deduce it from the block ID
	// on the receiver end in the CCL; however, we can optimize processing by
	// including it, so that we can check the signature before having the block

	// convert the vote to a vote message
	msg := messages.BlockVote{
		BlockID:   vote.BlockID,
		View:      vote.View,
		Signature: vote.Signature.Raw,
	}

	// send the vote the desired recipient
	err := e.con.Submit(&msg, recipientID)
	if err != nil {
		return fmt.Errorf("could not send vote: %w", err)
	}

	return nil
}

// BroadcastProposal will propagate a block proposal to all non-local consensus nodes.
func (e *Engine) BroadcastProposal(proposal *types.Proposal) error {

	// first, check that we are the proposer of the block
	if proposal.Block.ProposerID != e.me.NodeID() {
		return fmt.Errorf("can only propagate locally generated proposals")
	}

	// NOTE: a number of fields are redundant and can be omitted in the flow model
	// (- height can be deduced from the parent) -> not used for now
	// - parent view can be deduced from the parent

	// convert the proposal into a flat header model
	header := flow.Header{
		ChainID:       proposal.Block.ChainID,
		ParentID:      proposal.Block.QC.BlockID,
		ProposerID:    proposal.Block.ProposerID,
		View:          proposal.Block.View,
		Height:        proposal.Block.Height,
		PayloadHash:   proposal.Block.PayloadHash,
		Timestamp:     proposal.Block.Timestamp,
		ParentSigs:    proposal.Block.QC.AggregatedSignature.Raw,
		ParentSigners: proposal.Block.QC.AggregatedSignature.SignerIDs,
		ProposerSig:   proposal.Signature,
	}

	// check that the header ID corresponds to the proposal value
	if proposal.Block.BlockID != header.ID() {
		return fmt.Errorf("mismatch between proposal & reconstructed header block ID")
	}

	// retrieve the payload for the block
	payload, err := e.payloads.ByPayloadHash(proposal.Block.PayloadHash)
	if err != nil {
		return fmt.Errorf("could not retrieve payload for proposal: %w", err)
	}

	// check that the payload matches the payload hash
	if proposal.Block.PayloadHash != payload.Hash() {
		return fmt.Errorf("mismatch between proposal & reconstructed payload hash")
	}

	// retrieve all consensus nodes without our ID
	recipients, err := e.state.AtBlockID(proposal.Block.QC.BlockID).Identities(
		filter.HasRole(flow.RoleConsensus),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	)
	if err != nil {
		return fmt.Errorf("could not get consensus recipients: %w", err)
	}

	// NOTE: some fields are not needed for the message
	// - proposer ID is conveyed over the network message
	// - the payload hash is deduced from the payload
	msg := messages.BlockProposal{
		ChainID:       header.ChainID,
		ParentID:      header.ParentID,
		View:          header.View,
		Timestamp:     header.Timestamp,
		ParentSigs:    header.ParentSigs,
		ParentSigners: header.ParentSigners,
		ProposerSig:   header.ProposerSig,
		Payload:       payload,
	}

	// broadcast the proposal to consensus nodes
	err = e.con.Submit(&msg, recipients.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not send proposal message: %w", err)
	}

	return nil
}

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *messages.BlockProposal:
		return e.onBlockProposal(originID, ev)
	case *messages.BlockVote:
		return e.onBlockVote(originID, ev)
	case *messages.BlockRequest:
		return e.onBlockRequest(originID, ev)
	case *messages.BlockResponse:
		return e.onBlockResponse(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// onBlockProposal handles incoming block proposals.
func (e *Engine) onBlockProposal(originID flow.Identifier, proposal *messages.BlockProposal) error {

	// retrieve the parent for the block for parent view; if not found, cache for later
	parent, err := e.headers.ByBlockID(proposal.ParentID)
	if errors.Is(err, storage.ErrNotFound) {
		return e.processPendingProposal(originID, proposal)
	}
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	// store all of the block contents
	err = e.payloads.Store(proposal.Payload)
	if err != nil {
		return fmt.Errorf("could not store block payload: %w", err)
	}

	// reconstruct the block proposal to know block ID
	header := flow.Header{
		ChainID:       proposal.ChainID,
		ParentID:      proposal.ParentID,
		ProposerID:    originID,
		View:          proposal.View,
		Height:        parent.Height + 1,
		PayloadHash:   proposal.Payload.Hash(),
		Timestamp:     proposal.Timestamp,
		ParentSigs:    proposal.ParentSigs,
		ParentSigners: proposal.ParentSigners,
		ProposerSig:   proposal.ProposerSig,
	}

	// insert the header into the database
	err = e.headers.Store(&header)
	if err != nil {
		return fmt.Errorf("could not store header: %w", err)
	}

	// see if the block is a valid extension of the protocol state
	blockID := header.ID()
	err = e.state.Mutate().Extend(blockID)
	if err != nil {
		return fmt.Errorf("could not extend protocol state: %w", err)
	}

	// reconstruct the hotstuff models
	hsSig := types.AggregatedSignature{
		Raw:       header.ParentSigs,
		SignerIDs: header.ParentSigners,
	}
	hsQC := types.QuorumCertificate{
		View:                parent.View,
		BlockID:             header.ParentID,
		AggregatedSignature: &hsSig,
	}
	hsBlock := types.Block{
		ChainID:     header.ChainID,
		BlockID:     blockID,
		View:        header.View,
		ProposerID:  header.ProposerID,
		QC:          &hsQC,
		PayloadHash: header.PayloadHash,
		Timestamp:   header.Timestamp,
	}
	hsProposal := types.Proposal{
		Block:     &hsBlock,
		Signature: header.ProposerSig,
	}

	// submit the model to hotstuff for processing
	e.hotstuff.SubmitProposal(&hsProposal)

	// check for any descendants of the block to process
	children, ok := e.cache[blockID]
	if !ok {
		return nil
	}

	// then try to process children only this once
	var result *multierror.Error
	for _, child := range children {
		err := e.onBlockProposal(child.OriginID, child.Proposal)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	// remove the children from cache
	delete(e.cache, blockID)

	return result.ErrorOrNil()
}

// onBlockVote handles incoming block votes.
func (e *Engine) onBlockVote(originID flow.Identifier, vote *messages.BlockVote) error {

	// rebuild the signature using the origin ID and the transmitted signature
	hsSig := types.SingleSignature{
		Raw:      vote.Signature,
		SignerID: originID,
	}

	// rebuild the vote in the type prefered by hotstuff
	// NOTE: the view number is somewhat redundant, as we can locally check the view
	// of the referenced block ID; however, we might not have the block yet, so sending
	// the view along will allow us to validate the signature immediately and cache votes
	// appropriately within hotstuff
	hsVote := types.Vote{
		BlockID:   vote.BlockID,
		View:      vote.View,
		Signature: &hsSig,
	}

	// forward the vote to hotstuff for processing
	e.hotstuff.SubmitVote(&hsVote)

	return nil
}

// onBlockRequest is how we handle when blocks are requested from us.
func (e *Engine) onBlockRequest(originID flow.Identifier, request *messages.BlockRequest) error {

	// try to retrieve the block header from storage
	header, err := e.headers.ByBlockID(request.BlockID)
	if err != nil {
		return fmt.Errorf("could not find requested block: %w", err)
	}

	// try to retrieve the block payload from storage
	payload, err := e.payloads.ByPayloadHash(header.PayloadHash)
	if err != nil {
		return fmt.Errorf("could not find requested payload: %w", err)
	}

	// NOTE: there is a bunch of redundant block proposal construction code here;
	// this is already refactored in another PR and can be replaced once that is
	// merged and I rebase this PR

	// construct the block proposal
	proposal := messages.BlockProposal{
		ChainID:       header.ChainID,
		ParentID:      header.ParentID,
		View:          header.View,
		Timestamp:     header.Timestamp,
		ParentSigs:    header.ParentSigs,
		ParentSigners: header.ParentSigners,
		ProposerSig:   header.ProposerSig,
		Payload:       payload,
	}

	// construct the block response
	response := messages.BlockResponse{
		OriginID: header.ProposerID,
		Proposal: &proposal,
		Nonce:    request.Nonce,
	}
	err = e.con.Submit(&response, originID)
	if err != nil {
		return fmt.Errorf("could not send block response: %w", err)
	}

	return nil
}

// onBlockResponse is how we process responses to our block requests; we could simply use
// the same function as new proposals (onBlockProposal), but separating it allows us more
// flexibility; for one, we can explicitly convey the original origin ID. We could also do
// additional robustness stuff by matching requests & responses.
func (e *Engine) onBlockResponse(originID flow.Identifier, response *messages.BlockResponse) error {

	// for now, we simply process the block response like a normal proposal
	err := e.onBlockProposal(response.OriginID, response.Proposal)
	if err != nil {
		return fmt.Errorf("could not process block response: %w", err)
	}

	return nil
}

// processPendingProposal will deal with proposals where the parent is missing.
func (e *Engine) processPendingProposal(originID flow.Identifier, proposal *messages.BlockProposal) error {

	// first, we cache the proposal with its origin ID, so we can process it once possible
	item := cacheItem{
		OriginID: originID,
		Proposal: proposal,
	}
	e.cache[proposal.ParentID] = append(e.cache[proposal.ParentID], item)

	// send the block request
	request := messages.BlockRequest{
		BlockID: proposal.ParentID,
		Nonce:   rand.Uint64(),
	}
	err := e.con.Submit(&request, originID)
	if err != nil {
		return fmt.Errorf("could not send block request: %w", err)
	}

	// NOTE: at this point, if he doesn't send us the parent, we should probably think about a way
	// to blacklist him, as this can be exploited by sending us lots of children without parent;
	// a second mitigation strategy is to put a strict limit on children we cache, and possibly a
	// limit on children we cache coming from a single other node

	return nil
}
