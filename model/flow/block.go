// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

func Genesis(chainID ChainID) *Block {

	// create the raw content for the genesis block
	payload := Payload{
		Guarantees: nil,
		Seals:      nil,
	}

	// create the header
	header := Header{
		ChainID:     chainID,
		ParentID:    ZeroID,
		Height:      0,
		PayloadHash: payload.Hash(),
		Timestamp:   GenesisTime,
		View:        0,
	}

	// combine to block
	genesis := Block{
		Header:  &header,
		Payload: &payload,
	}

	return &genesis
}

//// Block (currently) includes the header, the payload hashes as well as the
//// payload contents.
//type Block struct {
//	Header  *Header
//	Payload *Payload
//}

type Block = GenericBlock[*Payload]

//func ToGeneric(block *Block) *GenericBlock[*Payload] {
//	return &GenericBlock[P]{}
//}

// BlockStatus represents the status of a block.
type BlockStatus int

const (
	// BlockStatusUnknown indicates that the block status is not known.
	BlockStatusUnknown BlockStatus = iota
	// BlockStatusFinalized is the status of a finalized block.
	BlockStatusFinalized
	// BlockStatusSealed is the status of a sealed block.
	BlockStatusSealed
)

// String returns the string representation of a transaction status.
func (s BlockStatus) String() string {
	return [...]string{"BLOCK_UNKNOWN", "BLOCK_FINALIZED", "BLOCK_SEALED"}[s]
}

// GenericPayload is a type constraint matching either main chain (consensus) or cluster payloads.
// NOTE: this is experimental while we explore introducing generics to the codebase.
type GenericPayload interface {
	Hash() Identifier
}

// GenericBlock is a generic type which can be used to represent both consensus and cluster blocks.
// This enables code which only operates on payload-generic aspects of the block to be
// implemented once and applied to both consensus and cluster blocks.
// NOTE: this is experimental while we explore introducing generics to the codebase.
type GenericBlock[P GenericPayload] struct {
	Header  *Header
	Payload P
}

// SetPayload sets the payload and updates the payload hash.
func (b *GenericBlock[P]) SetPayload(payload P) {
	b.Payload = payload
	b.Header.PayloadHash = payload.Hash()
}

// Valid will check whether the block is valid bottom-up.
func (b *GenericBlock[P]) Valid() bool {
	return b.Header.PayloadHash == b.Payload.Hash()
}

// ID returns the ID of the header.
func (b *GenericBlock[P]) ID() Identifier {
	return b.Header.ID()
}

// Checksum returns the checksum of the header.
func (b *GenericBlock[P]) Checksum() Identifier {
	return b.Header.Checksum()
}
