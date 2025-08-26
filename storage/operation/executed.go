package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// UpdateExecutedBlock updates the storage index for the latest executed block using the
// `codeExecutedBlock` prefix. This method records the block ID of the most recently executed
// block, regardless of whether it is on the canonical chain or a fork.
//
// ## Usage Context
//   - The stored "last executed block" may reference a block on a fork that is later orphaned.
//   - This is acceptable and expected: the value is used for reporting execution metrics and
//     for optimizing the loading of unexecuted blocks on node startup.
//   - On startup, the system will traverse from the last executed and last sealed blocks to
//     find a common ancestor, ensuring correctness even if the stored block is on a fork.
//   - If the stored block is not on the canonical chain, the node may re-execute some blocks
//     unnecessarily, but this is safe and does not affect correctness.
//
// ## Limitations & Edge Cases
// - The value is not guaranteed to be on the finalized or canonical chain.
// - Forks of arbitrary length may occur; the stored block may be on any such fork.
//
// ## Correct Usage
// - Use for metrics (e.g., reporting latest executed block height).
// - Use for optimizing block execution on startup (as a performance hint).
//
// ## Incorrect Usage
// - Do not use as a source of truth for canonical chain state.
// - Do not use for security-critical logic or fork resolution.
//
// See project documentation and `engine/execution/ingestion/loader/unexecuted_loader.go` for details on startup traversal logic.
func UpdateExecutedBlock(w storage.Writer, blockID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeExecutedBlock), blockID)
}

func RetrieveExecutedBlock(r storage.Reader, blockID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeExecutedBlock), blockID)
}
