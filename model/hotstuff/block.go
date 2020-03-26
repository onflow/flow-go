package hotstuff

import (
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Block is the HotStuff algorithm's concept of a block, which - in the bigger picture - corresponds
// to the block header.
type Block struct {
	BlockID     flow.Identifier
	View        uint64
	ProposerID  flow.Identifier
	QC          *QuorumCertificate
	PayloadHash flow.Identifier
	Timestamp   time.Time
}

// BlockFromFlow converts a flow header to a hotstuff block.
func BlockFromFlow(header *flow.Header, parentView uint64) *Block {

	sig := AggregatedSignature{
		StakingSignatures:     header.ParentStakingSigs,
		RandomBeaconSignature: header.ParentRandomBeaconSig,
		SignerIDs:             header.ParentSigners,
	}

	qc := QuorumCertificate{
		BlockID:             header.ParentID,
		View:                parentView,
		AggregatedSignature: &sig,
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
