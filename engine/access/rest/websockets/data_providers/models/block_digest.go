package models

import (
	"time"

	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

// BlockDigest is a lightweight block information model.
type BlockDigest struct {
	BlockId   string    `json:"block_id"`
	Height    string    `json:"height"`
	Timestamp time.Time `json:"timestamp"`
}

// NewBlockDigest creates a block digest instance with data from the provided flow.BlockDigest.
func NewBlockDigest(block *flow.BlockDigest) BlockDigest {
	return BlockDigest{
		BlockId:   block.ID().String(),
		Height:    util.FromUint(block.Height),
		Timestamp: block.Timestamp,
	}
}
