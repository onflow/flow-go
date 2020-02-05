// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"time"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/storage/ledger/trie"
)

func Genesis(ids IdentityList) *Block {

	// create the first seal with zero references
	seal := Seal{
		BlockID:       ZeroID,
		PreviousState: nil,
		FinalState:    trie.GetDefaultHashForHeight(255),
	}

	// create the raw content for the genesis block
	payload := Payload{
		Identities: ids,
		Guarantees: nil,
		Seals:      []*Seal{&seal},
	}

	// create the header
	header := Header{
		Number:      0,
		Timestamp:   time.Unix(1575244800, 0).UTC(),
		ParentID:    ZeroID,
		PayloadHash: payload.Hash(),
		ProposerID:  ZeroID,
	}

	// combine to block
	genesis := Block{
		Header:  header,
		Payload: payload,
	}

	return &genesis
}

// Block (currently) includes the header, the payload hashes as well as the
// payload contents.
type Block struct {
	Header
	Payload
}

// Valid will check whether the block is valid bottom-up.
func (b Block) Valid() bool {
	return b.PayloadHash == b.Payload.Hash()
}

// Header contains all meta-data for a block, as well as a hash representing
// the combined payload of the entire block. It is what consensus nodes agree
// on after validating the contents against the payload hash.
type Header struct {
	// Number is the block number, a monotonically incrementing counter of
	// finalized blocks.
	// TODO should we rename this Height?
	Number uint64
	// View is the view number at which this block was proposed.
	View uint64

	// ChainID is a chain-specific value to prevent replay attacks.
	ChainID string
	// Timestamp is the time at which this block was proposed. The proposing
	// node can choose any time, so this should not be trusted as accurate.
	Timestamp time.Time

	// ParentID is the ID of this block's parent.
	ParentID Identifier
	// PayloadHash is a hash of the payload of this block.
	PayloadHash Identifier

	// ParentView is the view number of the parent of this block.
	ParentView uint64
	// ParentSig is an aggregated signature for the parent block.
	ParentSig AggregatedSignature
	// ProposerID is the ID of the node that proposed this block.
	ProposerID Identifier

	// ProposerSig is the signature of the proposer over the header body.
	// NOTE: This is omitted from the ID.
	ProposerSig PartialSignature
}

// Body returns the immutable part of the block header.
func (h Header) Body() interface{} {
	return struct {
		Number      uint64
		View        uint64
		ChainID     string
		Timestamp   time.Time
		ParentID    Identifier
		PayloadHash Identifier
		ProposerID  Identifier
		ParentView  uint64
		ParentSig   []crypto.Signature
	}{
		Number:      h.Number,
		ChainID:     h.ChainID,
		View:        h.View,
		Timestamp:   h.Timestamp,
		ParentID:    h.ParentID,
		PayloadHash: h.PayloadHash,
		ProposerID:  h.ProposerID,
		ParentView:  h.ParentView,
		ParentSig:   h.ParentSig,
	}
}

// ID returns a unique ID to singularly identify the header and its block
// within the flow system.
func (h Header) ID() Identifier {
	return MakeID(h.Body())
}

// Checksum returns the checksum of the header.
func (h Header) Checksum() Identifier {
	return MakeID(h)
}

// Payload is the actual content of each block.
type Payload struct {
	Identities IdentityList
	Guarantees []*CollectionGuarantee
	Seals      []*Seal
}

// Hash returns the root hash of the payload.
func (p Payload) Hash() Identifier {
	idHash := MerkleRoot(GetIDs(p.Identities)...)
	collHash := MerkleRoot(GetIDs(p.Guarantees)...)
	sealHash := MerkleRoot(GetIDs(p.Seals)...)
	return ConcatSum(idHash, collHash, sealHash)
}
