// Package cluster contains models related to collection node cluster
// consensus.
package cluster

import (
	"io"

	"github.com/onflow/go-ethereum/rlp"

	"github.com/onflow/flow-go/model/flow"
)

func Genesis() *Block {
	header := &flow.Header{
		HeaderBody: flow.HeaderBody{
			View:      0,
			ChainID:   "cluster",
			Timestamp: flow.GenesisTime,
			ParentID:  flow.ZeroID,
		},
	}

	return NewBlock(header.HeaderBody, EmptyPayload(flow.ZeroID))
}

// Block represents a block in collection node cluster consensus. It contains
// a standard block header with a payload containing only a single collection.
type Block struct {
	Header  *flow.HeaderBody
	Payload *Payload
}

// NewBlock creates a new block in collection node cluster consensus.
//
// Parameters:
// - headerBody: the header fields to use for the block
// - payload: the payload to associate with the block
func NewBlock(
	headerBody flow.HeaderBody,
	payload Payload,
) *Block {
	return &Block{
		Header:  &headerBody,
		Payload: &payload,
	}
}

// ID returns a collision-resistant hash of the cluster.Block struct.
func (b *Block) ID() flow.Identifier {
	return flow.MakeID(b)
}

// EncodeRLP defines custom encoding for the cluster.Block to calculate its ID.
// The hash of the block is not just the hash of all the fields, It's a two-step process.
// If we just hash of all the fields, we lose the ability to have like a compressed data structure like the header.
// We first hash the payload fields, and then with that hash of the payload fields, we hash the header body fields and include the hash of the payload.
// This convention ensures that both the header and the block produce the same hash.
// The Timestamp is converted from time.Time to Unix time (uint64), which is necessary
// because time.Time is not RLP-encodable due to its private fields.
func (b *Block) EncodeRLP(w io.Writer) error {
	payloadHash := b.Payload.Hash()

	// the order of the fields is kept according to the flow.Header Fingerprint()
	encodingCanonicalForm := struct {
		ChainID            flow.ChainID
		ParentID           flow.Identifier
		Height             uint64
		PayloadHash        flow.Identifier
		Timestamp          uint64
		View               uint64
		ParentView         uint64
		ParentVoterIndices []byte
		ParentVoterSigData []byte
		ProposerID         flow.Identifier
		LastViewTCID       flow.Identifier
	}{
		ChainID:            b.Header.ChainID,
		ParentID:           b.Header.ParentID,
		Height:             b.Header.Height,
		PayloadHash:        payloadHash,
		Timestamp:          uint64(b.Header.Timestamp.UnixNano()),
		View:               b.Header.View,
		ParentView:         b.Header.ParentView,
		ParentVoterIndices: b.Header.ParentVoterIndices,
		ParentVoterSigData: b.Header.ParentVoterSigData,
		ProposerID:         b.Header.ProposerID,
		LastViewTCID:       b.Header.LastViewTC.ID(),
	}

	return rlp.Encode(w, encodingCanonicalForm)
}

// ToHeader converts the block into a compact [flow.Header] representation,
// where the payload is compressed to a hash reference.
func (b *Block) ToHeader() *flow.Header {
	return &flow.Header{
		HeaderBody:  *b.Header,
		PayloadHash: b.Payload.Hash(),
	}
}

// BlockProposal represents a signed proposed block in collection node cluster consensus.
type BlockProposal struct {
	Block           *Block
	ProposerSigData []byte
}
