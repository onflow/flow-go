package model

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// Block is the HotStuff algorithm's concept of a block, which - in the bigger picture - corresponds
// to the block header.
type Block struct {
	View       uint64
	BlockID    flow.Identifier
	ProposerID flow.Identifier
	QC         *flow.QuorumCertificate
	Timestamp  uint64 // Unix milliseconds
}

// BlockFromFlow converts a flow header to a hotstuff block.
func BlockFromFlow(header *flow.Header) *Block {
	block := Block{
		BlockID:    header.Hash(),
		View:       header.View,
		QC:         header.ParentQC(),
		ProposerID: header.ProposerID,
		Timestamp:  header.Timestamp,
	}

	return &block
}

// GenesisBlockFromFlow returns a HotStuff block model representing a genesis
// block based on the given header.
func GenesisBlockFromFlow(header *flow.Header) *Block {
	genesis := &Block{
		BlockID:    header.Hash(),
		View:       header.View,
		ProposerID: header.ProposerID,
		QC:         nil,
		Timestamp:  header.Timestamp,
	}
	return genesis
}

// CertifiedBlock holds a certified block, which is a block and a QC that is pointing to
// the block. A QC is the aggregated form of votes from a supermajority of HotStuff and
// therefore proves validity of the block. A certified block satisfies:
// Block.View == QC.View and Block.BlockID == QC.BlockID
type CertifiedBlock struct {
	Block        *Block
	CertifyingQC *flow.QuorumCertificate
}

// NewCertifiedBlock constructs a new certified block. It checks the consistency
// requirements and returns an exception otherwise:
//
//	Block.View == QC.View and Block.BlockID == QC.BlockID
func NewCertifiedBlock(block *Block, qc *flow.QuorumCertificate) (CertifiedBlock, error) {
	if block.View != qc.View {
		return CertifiedBlock{}, fmt.Errorf("block's view (%d) should equal the qc's view (%d)", block.View, qc.View)
	}
	if block.BlockID != qc.BlockID {
		return CertifiedBlock{}, fmt.Errorf("block's ID (%v) should equal the block referenced by the qc (%d)", block.BlockID, qc.BlockID)
	}
	return CertifiedBlock{Block: block, CertifyingQC: qc}, nil
}

// BlockID returns a unique identifier for the block (the ID signed to produce a block vote).
// To avoid repeated computation, we use value from the QC.
// CAUTION: This is not a cryptographic commitment for the CertifiedBlock model.
func (b *CertifiedBlock) BlockID() flow.Identifier {
	return b.CertifyingQC.BlockID
}

// View returns view where the block was proposed.
func (b *CertifiedBlock) View() uint64 {
	return b.Block.View
}
