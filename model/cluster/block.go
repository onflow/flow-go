// Package cluster contains models related to collection node cluster
// consensus.
package cluster

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

func Genesis() *Block {
	header := &flow.Header{
		View:      0,
		ChainID:   "",
		Timestamp: flow.GenesisTime(),
		ParentID:  flow.ZeroID,
	}

	payload := EmptyPayload()

	header.PayloadHash = payload.Hash()

	block := &Block{
		Header:  header,
		Payload: payload,
	}

	return block
}

// Block represents a block in collection node cluster consensus. It contains
// a standard block header with a payload containing only a single collection.
type Block struct {
	Header  *flow.Header
	Payload *Payload
}

// SetPayload sets the payload and payload hash.
func (b *Block) SetPayload(payload *Payload) {
	b.Payload = payload
	b.Header.PayloadHash = payload.Hash()
}

// ID returns the ID of the underlying block header.
func (b *Block) ID() flow.Identifier {
	return b.Header.ID()
}

// Checksum returns the checksum of the underlying block header.
func (b *Block) Checksum() flow.Identifier {
	return b.Header.Checksum()
}

// PendingBlock is a wrapper type representing a block that cannot yet be
// processed. The block header, payload, and sender ID are stored together
// while waiting for the block to become processable.
type PendingBlock struct {
	OriginID flow.Identifier
	Header   *flow.Header
	Payload  *Payload
}
