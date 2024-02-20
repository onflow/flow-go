// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"fmt"
	"time"
)

func Genesis(chainID ChainID) *Block {

	// create the raw content for the genesis block
	payload := Payload{}

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
	Block        *Block
	CertifyingQC *QuorumCertificate
}

// NewCertifiedBlock constructs a new certified block. It checks the consistency
// requirements and errors otherwise:
//
//	Block.View == QC.View and Block.BlockID == QC.BlockID
func NewCertifiedBlock(block *Block, qc *QuorumCertificate) (CertifiedBlock, error) {
	if block.Header.View != qc.View {
		return CertifiedBlock{}, fmt.Errorf("block's view (%d) should equal the qc's view (%d)", block.Header.View, qc.View)
	}
	if block.ID() != qc.BlockID {
		return CertifiedBlock{}, fmt.Errorf("block's ID (%v) should equal the block referenced by the qc (%d)", block.ID(), qc.BlockID)
	}
	return CertifiedBlock{Block: block, CertifyingQC: qc}, nil
}

// ID returns unique identifier for the block.
// To avoid repeated computation, we use value from the QC.
func (b *CertifiedBlock) ID() Identifier {
	return b.CertifyingQC.BlockID
}

// View returns view where the block was produced.
func (b *CertifiedBlock) View() uint64 {
	return b.CertifyingQC.View
}

// Height returns height of the block.
func (b *CertifiedBlock) Height() uint64 {
	return b.Block.Header.Height
}

// LightweightBlock holds lightweight block information which includes only block id, block height and block timestamp
type LightweightBlock struct {
	ID        Identifier
	Height    uint64
	Timestamp time.Time
}
