package flow

import (
	"fmt"
	"time"
)

func Genesis(chainID ChainID) *Block {
	// create genesis block
	genesis := Block{
		Header: HeaderFields{
			ChainID:   chainID,
			ParentID:  ZeroID,
			Height:    0,
			Timestamp: GenesisTime,
			View:      0,
		},
		Payload: &Payload{},
	}

	return &genesis
}

// Block (currently) includes the header fields and payload contents.
type Block struct {
	Header  HeaderFields
	Payload *Payload
}

// ToHeader hashes the payload of the block.
func (b *Block) ToHeader() *Header {
	return &Header{
		HeaderFields: b.Header,
		PayloadHash:  b.Payload.Hash(),
	}
}

// SetPayload sets the payload and updates the payload hash.
// TODO remove usages of this; include the payload in struct initialization, or as an argument to a builder or builder function
func (b *Block) SetPayload(payload Payload) {
	b.Payload = &payload
}

// ID returns the ID of the block, by first hashing the payload, and then the rest of the block.
func (b Block) ID() Identifier {
	return b.ToHeader().ID()
}

// Checksum returns the checksum of the header.
func (b Block) Checksum() Identifier {
	return b.ToHeader().Checksum()
}

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

// CertifiedBlock holds a certified block, which is a block and a QC that is pointing to
// the block. A QC is the aggregated form of votes from a supermajority of HotStuff and
// therefore proves validity of the block. A certified block satisfies:
// Block.View == QC.View and Block.BlockID == QC.BlockID
type CertifiedBlock struct {
	Proposal     *BlockProposal
	CertifyingQC *QuorumCertificate
}

// NewCertifiedBlock constructs a new certified block. It checks the consistency
// requirements and errors otherwise:
//
//	Block.View == QC.View and Block.BlockID == QC.BlockID
func NewCertifiedBlock(proposal *BlockProposal, qc *QuorumCertificate) (CertifiedBlock, error) {
	if proposal.Block.Header.View != qc.View {
		return CertifiedBlock{}, fmt.Errorf("block's view (%d) should equal the qc's view (%d)", proposal.Block.Header.View, qc.View)
	}
	if proposal.Block.ID() != qc.BlockID {
		return CertifiedBlock{}, fmt.Errorf("block's ID (%v) should equal the block referenced by the qc (%d)", proposal.Block.ID(), qc.BlockID)
	}
	return CertifiedBlock{Proposal: proposal, CertifyingQC: qc}, nil
}

// BlockID returns a unique identifier for the block (the ID signed to produce a block vote).
// To avoid repeated computation, we use value from the QC.
// CAUTION: This is not a cryptographic commitment for the CertifiedBlock model.
func (b *CertifiedBlock) BlockID() Identifier {
	return b.CertifyingQC.BlockID
}

// View returns view where the block was produced.
func (b *CertifiedBlock) View() uint64 {
	return b.CertifyingQC.View
}

// Height returns height of the block.
func (b *CertifiedBlock) Height() uint64 {
	return b.Proposal.Block.Header.Height
}

// BlockDigest holds lightweight block information which includes only the block's id, height and timestamp
type BlockDigest struct {
	BlockID   Identifier
	Height    uint64
	Timestamp time.Time
}

// NewBlockDigest constructs a new block digest.
func NewBlockDigest(
	blockID Identifier,
	height uint64,
	timestamp time.Time,
) *BlockDigest {
	return &BlockDigest{
		BlockID:   blockID,
		Height:    height,
		Timestamp: timestamp,
	}
}
