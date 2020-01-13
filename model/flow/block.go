// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"time"

	"github.com/dapperlabs/flow-go/crypto"
)

func Genesis(ids IdentityList) *Block {

	// create the raw content for the genesis block
	content := Content{
		Identities: ids,
		Guarantees: nil,
	}

	// create the payload hash summaries
	payload := content.Payload()

	// create the header
	header := Header{
		Number:      0,
		Timestamp:   time.Unix(1575244800, 0),
		ParentID:    ZeroID,
		PayloadHash: payload.Root(),
	}

	// combine to block
	genesis := Block{
		Header:  header,
		Payload: payload,
		Content: content,
	}

	return &genesis
}

// Block (currently) includes the header, the payload hashes as well as the
// payload contents.
type Block struct {
	Header
	Payload
	Content
}

// Valid will check whether the block is valid bottom-up.
func (b Block) Valid() bool {

	// check that the content hashes are correct
	if b.Payload != b.Content.Payload() {
		return false
	}

	// check that the payload hash is correct
	if b.PayloadHash != b.Payload.Root() {
		return false
	}

	return true
}

// Header contains all meta-data for a block, as well as a hash representing
// the combined payload of the entire block. It is what consensus nodes agree
// on after validating the contents against the payload hash.
type Header struct {
	Number      uint64
	Timestamp   time.Time
	ParentID    Identifier
	PayloadHash Identifier
	Signatures  []crypto.Signature
}

// Body returns the immutable part of the block header.
func (h Header) Body() interface{} {
	return struct {
		Number      uint64
		Timestamp   time.Time
		ParentID    Identifier
		PayloadHash Identifier
	}{
		Number:      h.Number,
		Timestamp:   h.Timestamp,
		ParentID:    h.ParentID,
		PayloadHash: h.PayloadHash,
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
	IdentitiesHash  Identifier
	CollectionsHash Identifier
}

// Root returns the hash of the payload.
func (p Payload) Root() Identifier {
	return ConcatSum(
		p.IdentitiesHash,
		p.CollectionsHash,
	)
}

// Content is the actual content of each block.
type Content struct {
	Identities IdentityList
	Guarantees []*CollectionGuarantee
}

// Payload creates the payload for the given content.
func (c Content) Payload() Payload {
	p := Payload{
		IdentitiesHash:  MerkleRoot(GetIDs(c.Identities)...),
		CollectionsHash: MerkleRoot(GetIDs(c.Guarantees)...),
	}
	return p
}
