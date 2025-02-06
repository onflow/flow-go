package models

import (
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

// Build creates a BlockDigest instance with data from the provided flow.BlockDigest.
func (b *BlockDigest) Build(block *flow.BlockDigest) {
	b.BlockId = block.BlockID.String()
	b.Height = util.FromUint(block.Height)
	b.Timestamp = block.Timestamp
}
