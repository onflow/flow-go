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
		BlockID:      ZeroID,
		ParentCommit: nil,
		StateCommit:  trie.Hash(trie.EmptySlice),
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
	Number      uint64
	Timestamp   time.Time
	ParentID    Identifier
	PayloadHash Identifier
	ProposerID  Identifier
	Signatures  []crypto.Signature
}

// Body returns the immutable part of the block header.
func (h Header) Body() interface{} {
	return struct {
		Number      uint64
		Timestamp   time.Time
		ParentID    Identifier
		PayloadHash Identifier
		ProposerID  Identifier
	}{
		Number:      h.Number,
		Timestamp:   h.Timestamp,
		ParentID:    h.ParentID,
		PayloadHash: h.PayloadHash,
		ProposerID:  h.ProposerID,
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
