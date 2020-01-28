// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"time"

	"github.com/dapperlabs/flow-go/crypto"
)

func Genesis(ids IdentityList) *Block {

	// create the raw content for the genesis block
	payload := Payload{
		Identities: ids,
		Guarantees: nil,
	}

	// create the header
	header := Header{
		Number:      0,
		Timestamp:   time.Unix(1575244800, 0),
		ParentID:    ZeroID,
		PayloadHash: payload.Hash(),
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
	// check that the payload hash is correct
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

// Payload represents the second level of the block payload merkle tree.
type Payload struct {
	Identities IdentityList
	Guarantees []*CollectionGuarantee
}

// Root returns the hash of the payload.
func (p Payload) Hash() Identifier {
	return ConcatSum(
		MerkleRoot(GetIDs(p.Identities)...),
		MerkleRoot(GetIDs(p.Guarantees)...),
	)
}
