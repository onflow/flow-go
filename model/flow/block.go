// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

func Genesis(identities IdentityList) *Block {

	// create the raw content for the genesis block
	payload := Payload{
		Identities: identities,
		Guarantees: nil,
		Seals:      nil,
	}

	// create the header
	header := Header{
		ChainID:     DefaultChainID,
		ParentID:    ZeroID,
		Height:      0,
		PayloadHash: payload.Hash(),
		Timestamp:   GenesisTime(),
		View:        0,
	}

	// combine to block
	genesis := Block{
		Header:  &header,
		Payload: &payload,
	}

	return &genesis
}

// Block (currently) includes the header, the payload hashes as well as the
// payload contents.
type Block struct {
	Header  *Header
	Payload *Payload
}

// SetPayload sets the payload and updates the payload hash.
func (b *Block) SetPayload(payload Payload) {
	b.Payload = &payload
	b.Header.PayloadHash = b.Payload.Hash()
}

// Valid will check whether the block is valid bottom-up.
func (b Block) Valid() bool {
	return b.Header.PayloadHash == b.Payload.Hash()
}

// ID returns the ID of the header.
func (b Block) ID() Identifier {
	return b.Header.ID()
}

// Checksum returns the checksum of the header.
func (b Block) Checksum() Identifier {
	return b.Header.Checksum()
}

// PendingBlock is a wrapper type representing a block that cannot yet be
// processed. The block header, payload, and sender ID are stored together
// while waiting for the block to become processable.
type PendingBlock struct {
	OriginID Identifier
	Header   *Header
	Payload  *Payload
}
