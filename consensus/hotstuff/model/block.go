package model

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
)

// Block is the HotStuff algorithm's concept of a block, which - in the bigger picture - corresponds
// to the block header.
type Block struct {
	View        uint64
	BlockID     flow.Identifier
	ProposerID  flow.Identifier
	QC          *flow.QuorumCertificate
	PayloadHash flow.Identifier
	Timestamp   time.Time
}

// BlockFromFlow converts a flow header to a hotstuff block.
func BlockFromFlow(header *flow.Header) *Block {
	block := Block{
		BlockID:     header.ID(),
		View:        header.View,
		QC:          header.QuorumCertificate(),
		ProposerID:  header.ProposerID,
		PayloadHash: header.PayloadHash,
		Timestamp:   header.Timestamp,
	}

	return &block
}

// GenesisBlockFromFlow returns a HotStuff block model representing a genesis
// block based on the given header.
func GenesisBlockFromFlow(header *flow.Header) *Block {
	genesis := &Block{
		BlockID:     header.ID(),
		View:        header.View,
		ProposerID:  header.ProposerID,
		QC:          nil,
		PayloadHash: header.PayloadHash,
		Timestamp:   header.Timestamp,
	}
	return genesis
}

// CertifiedBlock holds a certified block, which is a block and a QC that is pointing to
// the block. A QC is the aggregated form of votes from a supermajority of HotStuff and
// therefore proves validity of the block. A certified block satisfies:
// Block.View == QC.View and Block.BlockID == QC.BlockID
type CertifiedBlock struct {
	Block *Block
	QC    *flow.QuorumCertificate
}

// ID returns unique identifier for the block.
// To avoid repeated computation, we use value from the QC.
func (b *CertifiedBlock) ID() flow.Identifier {
	return b.QC.BlockID
}

// View returns view where the block was proposed.
func (b *CertifiedBlock) View() uint64 {
	return b.QC.View
}
