package fvm

import "github.com/onflow/cadence/runtime"

// Blocks provides access to block headers
type Blocks interface {
	// returns current block header
	Current() (runtime.Block, error)
	// returns the height of the current block
	Height() uint64
	// ByHeight returns the block at the given height in the chain ending in `header` (or finalized
	// if `header` is nil). This enables querying un-finalized blocks by height with respect to the
	// chain defined by the block we are executing. It returns a runtime block,
	// a boolean which is set if block is found and an error if any fatal error happens
	ByHeight(height uint64) (runtime.Block, bool, error)
}
