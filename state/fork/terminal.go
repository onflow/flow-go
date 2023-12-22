package fork

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// Terminal specifies the terminal condition for traversing a fork. It abstracts
// the precise termination condition for `TraverseBackward` and `TraverseForward`.
// Generally, when traversing a fork (in either direction), there are two distinct
// blocks:
//   - the `head` of the fork that should be traversed
//   - the `lowestBlock` in that fork, which should be included in the traversal
//
// The traversal walks `head <--> lowestBlock` (in either direction).
//
// There are a variety of ways to precisely specify `head` and `lowestBlock`:
//   - At least one block, `head` or `lowestBlock`, must be specified by its ID
//     to unambiguously identify the fork that should be traversed.
//   - The other block an either be specified by ID or height.
//   - If both `head` and `lowestBlock` are specified by their ID,
//     they must both be on the same fork.
//
// Essentially it converts the termination condition into two statements:
//   - (i) The height of the lowest block that should be visited.
//   - (ii) A consistency check `ConfirmTerminalReached` for the lowest visited block.
//     This check is predominantly useful in case both `head` and `lowestBlock`
//     are specified by their ID. It allows to enforce that `head` and `lowestBlock`
//     are both on the same fork and error otherwise.
//     However, the precise implementation of `ConfirmTerminalReached` is left to
//     the Terminal. Other, more elaborate conditions are possible.
type Terminal interface {
	// LowestHeightToVisit computes the height of the lowest block that should be visited
	LowestHeightToVisit(headers storage.Headers) (uint64, error)

	// ConfirmTerminalReached is a self-consistency check that the lowest visited block is
	// in fact the expected terminal.
	ConfirmTerminalReached(headers storage.Headers, lowestVisitedBlock *flow.Header) error
}
