package model

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
