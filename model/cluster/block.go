// Package cluster contains models related to collection node cluster
// consensus.
package cluster

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// UnsignedBlock represents a block in collection node cluster consensus. It contains
// a standard block header with a payload containing only a single collection.
//
// Zero values for certain HeaderBody fields are allowed only for root blocks, which must be constructed
// using the NewRootBlock constructor. All non-root blocks must be constructed
// using NewBlock to ensure validation of the block fields.
//
//structwrite:immutable - mutations allowed only within the constructor
type UnsignedBlock = flow.GenericBlock[Payload]

// UntrustedUnsignedBlock is an untrusted input-only representation of a cluster UnsignedBlock,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedUnsignedBlock should be validated and converted into
// a trusted cluster UnsignedBlock using the NewBlock constructor (or NewRootBlock
// for the root block).
type UntrustedUnsignedBlock UnsignedBlock

// NewBlock creates a new block in collection node cluster consensus.
// This constructor enforces validation rules to ensure the block is well-formed.
// It must be used to construct all non-root blocks.
//
// All errors indicate that a valid UnsignedBlock cannot be constructed from the input.
func NewBlock(untrusted UntrustedUnsignedBlock) (*UnsignedBlock, error) {
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

	return &UnsignedBlock{
		HeaderBody: *headerBody,
		Payload:    *payload,
	}, nil
}

// NewRootBlock creates a root block in collection node cluster consensus.
//
// This constructor must be used **only** for constructing the root block,
// which is the only case where zero values are allowed.
func NewRootBlock(untrusted UntrustedUnsignedBlock) (*UnsignedBlock, error) {
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

	return &UnsignedBlock{
		HeaderBody: *rootHeaderBody,
		Payload:    *rootPayload,
	}, nil
}

// Proposal represents a signed proposed block in collection node cluster consensus.
type Proposal struct {
	Block           UnsignedBlock
	ProposerSigData []byte
}

// ProposalHeader converts the proposal into a compact [ProposalHeader] representation,
// where the payload is compressed to a hash reference.
func (p *Proposal) ProposalHeader() *flow.ProposalHeader {
	return &flow.ProposalHeader{Header: p.Block.ToHeader(), ProposerSigData: p.ProposerSigData}
}
