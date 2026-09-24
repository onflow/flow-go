package common

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// GetLastFinalizedAndExecutedBlock returns the block ID, height and state commitment of the
// highest block that is both finalized and executed.
//
// It mirrors the anchor semantics of the ledger-backed `GetHighestFinalizedExecuted` in
// `engine/execution/state`: the executed block pointer is retrieved from the protocol
// database and capped by the finalized head, so the returned height is always at most the
// finalized height. The state commitment of the resulting block must be present in the
// commits store, which by construction means the block has been executed.
//
// No error returns are expected during normal operation.
func GetLastFinalizedAndExecutedBlock(
	state protocol.State,
	db storage.DB,
	headers storage.Headers,
	commits storage.Commits,
) (flow.Identifier, uint64, flow.StateCommitment, error) {
	finalized, err := state.Final().Head()
	if err != nil {
		return flow.ZeroID, 0, flow.DummyStateCommitment, fmt.Errorf("cannot get finalized head: %w", err)
	}

	var executedBlockID flow.Identifier
	err = operation.RetrieveExecutedBlock(db.Reader(), &executedBlockID)
	if err != nil {
		return flow.ZeroID, 0, flow.DummyStateCommitment, fmt.Errorf("cannot retrieve executed block: %w", err)
	}

	executedHeader, err := headers.ByBlockID(executedBlockID)
	if err != nil {
		return flow.ZeroID, 0, flow.DummyStateCommitment, fmt.Errorf("cannot retrieve executed header %v: %w", executedBlockID, err)
	}

	// the highest finalized and executed height is the min of the two
	highest := min(finalized.Height, executedHeader.Height)

	blockID, err := headers.BlockIDByHeight(highest)
	if err != nil {
		return flow.ZeroID, 0, flow.DummyStateCommitment, fmt.Errorf("cannot get block ID by height %d: %w", highest, err)
	}

	commit, err := commits.ByBlockID(blockID)
	if err != nil {
		return flow.ZeroID, 0, flow.DummyStateCommitment, fmt.Errorf("cannot get state commitment for block %v (height %d): %w", blockID, highest, err)
	}

	return blockID, highest, commit, nil
}
