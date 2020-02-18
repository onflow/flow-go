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
		ChainID:       "flow",
		ParentID:      ZeroID,
		ProposerID:    ZeroID,
		View:          0,
		Height:        0,
		PayloadHash:   payload.Hash(),
		Timestamp:     time.Unix(1575244800, 0).UTC(),
		ParentSigs:    nil,
		ParentSigners: nil,
		ProposerSig:   nil,
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
	// ChainID is a chain-specific value to prevent replay attacks.
	ChainID string
	// ParentID is the ID of this block's parent.
	ParentID Identifier
	// ProposerID is the ID of the node that proposed this block.
	ProposerID Identifier
	// View is the view number at which this block was proposed.
	View uint64
	// Height is the height of the block in the blockchain
	Height uint64
	// PayloadHash is a hash of the payload of this block.
	PayloadHash Identifier
	// Timestamp is the time at which this block was proposed. The proposing
	// node can choose any time, so this should not be trusted as accurate.
	Timestamp time.Time
	// ParentSig is an aggregated signature for the parent block.
	ParentSigs    []crypto.Signature
	ParentSigners []Identifier
	// ProposerSig is the signature of the proposer over the header body.
	ProposerSig crypto.Signature
}

// Body returns the immutable part of the block header.
func (h Header) Body() interface{} {
	return struct {
		ChainID       string
		ParentID      Identifier
		ProposerID    Identifier
		View          uint64
		Height        uint64
		PayloadHash   Identifier
		Timestamp     time.Time
		ParentSigs    []crypto.Signature
		ParentSigners []Identifier
	}{
		ChainID:       h.ChainID,
		ParentID:      h.ParentID,
		ProposerID:    h.ProposerID,
		View:          h.View,
		Height:        h.Height,
		PayloadHash:   h.PayloadHash,
		Timestamp:     h.Timestamp,
		ParentSigs:    h.ParentSigs,
		ParentSigners: h.ParentSigners,
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
