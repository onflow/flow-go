package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// UpdateExecutedBlock updates the pointer to the Execution Node's OWN highest executed block. We
// overwrite the block ID of the most recently executed block, regardless of whether this block may
// later be orphaned or is already orphaned.
//
// ## Usage Context
//   - The stored "last executed block" may reference a block on a fork that is later orphaned.
//   - This is acceptable and expected: the index is intended for reporting execution metrics and
//     for optimizing the loading of unexecuted blocks on node startup.
//   - On startup, the Execution Node may use the latest executed block as a hint on where to
//     restart the execution. It MUST traverse from the last executed in the direction of decreasing
//     height. It will eventually reach a block with a finalized seal. From this block, the Execution
//     Node should restart its execution and cover _all_ descendants (that are not orphaned). Thereby,
//     we guarantee that even if the stored block is on a fork, we eventually also cover blocks
//     are finalized or and the most recent still unfinalized blocks.
//   - If the block referenced as "highest executed block" is not on the canonical chain, the Execution
//     Node may (re-)execute some blocks unnecessarily, but this does not affect correctness.
//
// ## Limitations & Edge Cases
// - The value is not guaranteed to be on the finalized chain.
// - Forks of arbitrary length may occur; the stored block may be on any such fork.
//
// ## Correct Usage
// - Use for metrics (e.g., reporting latest executed block height).
// - Use for optimizing block execution on startup (as a performance hint).
//
// ## Incorrect Usage
// - Do not use as a source of truth for canonical chain state.
// - Do not disregard blocks with lower heights as not needing execution.
//
// See project documentation in `engine/execution/ingestion/loader/unexecuted_loader.go` for details on startup traversal logic.
func UpdateExecutedBlock(w storage.Writer, blockID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeExecutedBlock), blockID)
}

func RetrieveExecutedBlock(r storage.Reader, blockID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeExecutedBlock), blockID)
}
