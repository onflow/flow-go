package fvm

import "github.com/dapperlabs/flow-go/model/flow"

type Blocks interface {
	// ByHeight returns the block at the given height. It is only available
	// for finalized blocks.
	ByHeight(height uint64) (*flow.Block, error)
}
