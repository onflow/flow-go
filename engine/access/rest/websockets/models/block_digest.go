package models

import (
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

func (b *BlockDigest) Build(block *flow.BlockDigest) {
	b.BlockId = block.ID().String()
	b.Height = util.FromUint(block.Height)
	b.Timestamp = block.Timestamp
}
