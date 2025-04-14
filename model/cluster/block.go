// Package cluster contains models related to collection node cluster
// consensus.
package cluster

import (
	"github.com/onflow/flow-go/model/flow"
)

func Genesis() *Block {
	// TODO: Uliana: refactor after PR #7100 will be fixed
	header := &flow.Header{
		View:      0,
		ChainID:   "cluster",
		Timestamp: flow.GenesisTime,
		ParentID:  flow.ZeroID,
	}

	payload := EmptyPayload(flow.ZeroID)
	headerFields := header.HeaderFields()

	block := &Block{
		Header: &headerFields,
	}
	block.SetPayload(payload)

	return block
}

// Block represents a block in collection node cluster consensus. It contains
// a standard block header with a payload containing only a single collection.
type Block struct {
	Header  *flow.HeaderFields
	Payload *Payload
}

// ToHeader hashes the payload of the block.

func (b *Block) ToHeader() *flow.Header {
	return &flow.Header{
		ChainID:            b.Header.ChainID,
		ParentID:           b.Header.ParentID,
		Height:             b.Header.Height,
		Timestamp:          b.Header.Timestamp,
		View:               b.Header.View,
		ParentView:         b.Header.ParentView,
		ParentVoterIndices: b.Header.ParentVoterIndices,
		ParentVoterSigData: b.Header.ParentVoterSigData,
		ProposerID:         b.Header.ProposerID,
		LastViewTC:         b.Header.LastViewTC,
		PayloadHash:        b.Payload.Hash(),
	}
}

// BlockProposal represents a signed proposed block in collection node cluster consensus.
// TODO(malleability, #7100) clean up types
type BlockProposal struct {
	Block           *Block
	ProposerSigData []byte
}

// ID returns the ID of the underlying block header.
func (b *Block) ID() flow.Identifier {
	return flow.MakeID(b)
}

// SetPayload sets the payload and payload hash.
func (b *Block) SetPayload(payload Payload) {
	b.Payload = &payload
}
