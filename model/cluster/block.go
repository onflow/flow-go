// Package cluster contains models related to collection node cluster
// consensus.
package cluster

import (
	"github.com/onflow/flow-go/model/flow"
)

func Genesis() *Block {
	headerBody := flow.HeaderBody{
		View:      0,
		ChainID:   "cluster",
		Timestamp: flow.GenesisTime,
		ParentID:  flow.ZeroID,
	}

	//TODO(Uliana: malleability immutable): decide if we need separate constuctor or do not use constructor here at all
	//nolint:structwrite
	return &Block{
		Header:  headerBody,
		Payload: *NewEmptyPayload(flow.ZeroID),
	}
}

// Block represents a block in collection node cluster consensus. It contains
// a standard block header with a payload containing only a single collection.
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
// a trusted cluster Block using NewBlock constructor.
type UntrustedBlock Block

// NewBlock creates a new block in collection node cluster consensus.
// Construction cluster Block allowed only within the constructor.
//
// All errors indicate a valid ExecutionReceipt cannot be constructed from the input.
func NewBlock(untrusted UntrustedBlock) (*Block, error) {
	return &Block{
		Header:  untrusted.Header,
		Payload: untrusted.Payload,
	}, nil
}

// ID returns a collision-resistant hash of the cluster.Block struct.
func (b *Block) ID() flow.Identifier {
	return b.ToHeader().ID()
}

// ToHeader converts the block into a compact [flow.Header] representation,
// where the payload is compressed to a hash reference.
func (b *Block) ToHeader() *flow.Header {
	return &flow.Header{
		HeaderBody:  b.Header,
		PayloadHash: b.Payload.Hash(),
	}
}

// BlockProposal represents a signed proposed block in collection node cluster consensus.
type BlockProposal struct {
	Block           Block
	ProposerSigData []byte
}
