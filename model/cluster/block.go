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
// Zero values are allowed only for root blocks, which must be constructed
// using the NewRootBlock constructor. All non-root blocks must be constructed
// using NewBlock to ensure validation of the block fields.
//
//structwrite:immutable - mutations allowed only within the constructor
type Block struct {
	Header  flow.HeaderBody
	Payload Payload
}

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
	untrustedHeaderBody := untrusted.Header
	if untrustedHeaderBody.ParentID == flow.ZeroID {
		return nil, fmt.Errorf("parent ID must not be zero")
	}
	if len(untrustedHeaderBody.ParentVoterIndices) == 0 {
		return nil, fmt.Errorf("parent voter indices must not be empty")
	}
	if len(untrustedHeaderBody.ParentVoterSigData) == 0 {
		return nil, fmt.Errorf("parent voter signature must not be empty")
	}
	if untrustedHeaderBody.ProposerID == flow.ZeroID {
		return nil, fmt.Errorf("proposer ID must not be zero")
	}

	// validate payload
	payload, err := NewPayload(UntrustedPayload(untrusted.Payload))
	if err != nil {
		return nil, fmt.Errorf("invalid cluster payload: %w", err)
	}

	return &Block{
		Header:  untrustedHeaderBody,
		Payload: *payload,
	}, nil
}

// NewRootBlock creates a root block in collection node cluster consensus.
//
// This constructor must be used **only** for constructing the root block,
// which is the only case where zero values are allowed.
func NewRootBlock(untrusted UntrustedBlock) *Block {
	return &Block{
		Header:  untrusted.Header,
		Payload: untrusted.Payload,
	}
}

// ID returns a collision-resistant hash of the cluster.Block struct.
func (b *Block) ID() flow.Identifier {
	return b.ToHeader().ID()
}

// ToHeader converts the block into a compact [flow.Header] representation,
// where the payload is compressed to a hash reference.
// The receiver Block must be well-formed (enforced by mutation protection on the type).
// This function may panic if invoked on a malformed Block.
func (b *Block) ToHeader() *flow.Header {
	if b.Header.IsRootHeaderBody() {
		return flow.NewRootHeader(flow.UntrustedHeader{
			HeaderBody:  b.Header,
			PayloadHash: b.Payload.Hash(),
		})
	}

	header, err := flow.NewHeader(flow.UntrustedHeader{
		HeaderBody:  b.Header,
		PayloadHash: b.Payload.Hash(),
	})
	if err != nil {
		panic(fmt.Errorf("could not build header from block: %w", err))
	}

	return header
}

// BlockProposal represents a signed proposed block in collection node cluster consensus.
type BlockProposal struct {
	Block           Block
	ProposerSigData []byte
}
