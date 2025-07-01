// Package cluster contains models related to collection node cluster
// consensus.
package cluster

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

func Genesis() *Block {
	headerBody := flow.NewRootHeaderBody(flow.UntrustedHeaderBody{
		View:      0,
		ChainID:   "cluster",
		Timestamp: flow.GenesisTime,
		ParentID:  flow.ZeroID,
	})

	block := NewBlock(*headerBody, *NewEmptyPayload(flow.ZeroID))
	return &block
}

// Block represents a block in collection node cluster consensus. It contains
// a standard block header with a payload containing only a single collection.
type Block struct {
	Header  flow.HeaderBody
	Payload Payload
}

// NewBlock creates a new block in collection node cluster consensus.
//
// Parameters:
// - headerBody: the header fields to use for the block
// - payload: the payload to associate with the block
func NewBlock(
	headerBody flow.HeaderBody,
	payload Payload,
) Block {
	return Block{
		Header:  headerBody,
		Payload: payload,
	}
}

// ID returns a collision-resistant hash of the cluster.Block struct.
func (b *Block) ID() flow.Identifier {
	return b.ToHeader().ID()
}

// ToHeader converts the block into a compact [flow.Header] representation,
// where the payload is compressed to a hash reference.
func (b *Block) ToHeader() *flow.Header {
	if b.Header.ContainsParentQC() {
		header, err := flow.NewHeader(flow.UntrustedHeader{
			HeaderBody:  b.Header,
			PayloadHash: b.Payload.Hash(),
		})
		if err != nil {
			panic(fmt.Errorf("could not build header from block: %w", err))
		}
		return header
	}

	header := flow.NewRootHeader(flow.UntrustedHeader{
		HeaderBody:  b.Header,
		PayloadHash: b.Payload.Hash(),
	})

	return header
}

// BlockProposal represents a signed proposed block in collection node cluster consensus.
type BlockProposal struct {
	Block           Block
	ProposerSigData []byte
}
