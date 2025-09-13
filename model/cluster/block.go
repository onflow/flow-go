// Package cluster contains models related to collection node cluster
// consensus.
package cluster

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// Block represents a block in collection node cluster consensus. It contains
// a standard block header with a payload containing only a single collection.
//
// Zero values for certain HeaderBody fields are allowed only for root blocks, which must be constructed
// using the NewRootBlock constructor. All non-root blocks must be constructed
// using NewBlock to ensure validation of the block fields.
//
//structwrite:immutable - mutations allowed only within the constructor
type Block = flow.GenericBlock[Payload]

// UntrustedBlock is an untrusted input-only representation of a cluster Block,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedBlock should be validated and converted into
// a trusted cluster Block using the NewBlock constructor (or NewRootBlock
// for the root block).
type UntrustedBlock Block

// NewBlock creates a new block in collection node cluster consensus.
// This constructor enforces validation rules to ensure the block is well-formed.
// It must be used to construct all non-root blocks.
//
// All errors indicate that a valid Block cannot be constructed from the input.
func NewBlock(untrusted UntrustedBlock) (*Block, error) {
	// validate header body
	headerBody, err := flow.NewHeaderBody(flow.UntrustedHeaderBody(untrusted.HeaderBody))
	if err != nil {
		return nil, fmt.Errorf("invalid header body: %w", err)
	}

	// validate payload
	payload, err := NewPayload(UntrustedPayload(untrusted.Payload))
	if err != nil {
		return nil, fmt.Errorf("invalid cluster payload: %w", err)
	}

	return &Block{
		HeaderBody: *headerBody,
		Payload:    *payload,
	}, nil
}

// NewRootBlock creates a root block in collection node cluster consensus.
//
// This constructor must be used **only** for constructing the root block,
// which is the only case where zero values are allowed.
func NewRootBlock(untrusted UntrustedBlock) (*Block, error) {
	rootHeaderBody, err := flow.NewRootHeaderBody(flow.UntrustedHeaderBody(untrusted.HeaderBody))
	if err != nil {
		return nil, fmt.Errorf("invalid root header body: %w", err)
	}

	if rootHeaderBody.ParentID != flow.ZeroID {
		return nil, fmt.Errorf("ParentID must be zero")
	}

	rootPayload, err := NewRootPayload(UntrustedPayload(untrusted.Payload))
	if err != nil {
		return nil, fmt.Errorf("invalid root cluster payload: %w", err)
	}

	return &Block{
		HeaderBody: *rootHeaderBody,
		Payload:    *rootPayload,
	}, nil
}

// Proposal represents a signed proposed block in collection node cluster consensus.
//
//structwrite:immutable - mutations allowed only within the constructor.
type Proposal struct {
	Block           Block
	ProposerSigData []byte
}

// UntrustedProposal is an untrusted input-only representation of a cluster.Proposal,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedProposal should be validated and converted into
// a trusted cluster Proposal using the NewProposal constructor (or NewRootProposal
// for the root proposal).
type UntrustedProposal Proposal

// NewProposal creates a new cluster Proposal.
// This constructor enforces validation rules to ensure the Proposal is well-formed.
//
// All errors indicate that a valid cluster.Proposal cannot be constructed from the input.
func NewProposal(untrusted UntrustedProposal) (*Proposal, error) {
	block, err := NewBlock(UntrustedBlock(untrusted.Block))
	if err != nil {
		return nil, fmt.Errorf("invalid block: %w", err)
	}
	if len(untrusted.ProposerSigData) == 0 {
		return nil, fmt.Errorf("proposer signature must not be empty")
	}

	return &Proposal{
		Block:           *block,
		ProposerSigData: untrusted.ProposerSigData,
	}, nil
}

// NewRootProposal creates a root cluster proposal.
// This constructor must be used **only** for constructing the root proposal,
// which is the only case where zero values are allowed.
func NewRootProposal(untrusted UntrustedProposal) (*Proposal, error) {
	block, err := NewRootBlock(UntrustedBlock(untrusted.Block))
	if err != nil {
		return nil, fmt.Errorf("invalid root block: %w", err)
	}
	if len(untrusted.ProposerSigData) > 0 {
		return nil, fmt.Errorf("proposer signature must be empty")
	}

	return &Proposal{
		Block:           *block,
		ProposerSigData: untrusted.ProposerSigData,
	}, nil

}

// ProposalHeader converts the proposal into a compact [ProposalHeader] representation,
// where the payload is compressed to a hash reference.
func (p *Proposal) ProposalHeader() *flow.ProposalHeader {
	return &flow.ProposalHeader{Header: p.Block.ToHeader(), ProposerSigData: p.ProposerSigData}
}

// BlockResponse is the same as flow.BlockResponse, but for cluster
// consensus. It contains a list of structurally validated cluster block proposals
// that should correspond to the request.
type BlockResponse flow.GenericBlockResponse[Proposal]
