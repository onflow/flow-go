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
func BlockFromFlow(header *flow.Header, parentView uint64) *Block {

	qc := flow.QuorumCertificate{
		BlockID:   header.ParentID,
		View:      parentView,
		SignerIDs: header.ParentVoterIDs,
		SigData:   header.ParentVoterSigData,
	}

	block := Block{
		BlockID:     header.ID(),
		View:        header.View,
		QC:          &qc,
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
