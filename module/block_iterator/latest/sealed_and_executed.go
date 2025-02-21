package latest

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage/badger/procedure"
)

type LatestSealedAndExecuted struct {
	root            *flow.Header
	state           protocol.State
	executedBlockDB *badger.DB
}

func NewLatestSealedAndExecuted(
	root *flow.Header,
	state protocol.State,
	executedBlockDB *badger.DB,
) *LatestSealedAndExecuted {
	return &LatestSealedAndExecuted{
		root:            root,
		state:           state,
		executedBlockDB: executedBlockDB,
	}
}

// BelowLatest returns the header at the given threshold below the latest sealed and executed block.
func (l *LatestSealedAndExecuted) BelowLatest(threshold uint64) (*flow.Header, error) {

	minHeight := l.root.Height + threshold
	latest, err := l.Latest()
	if err != nil {
		return nil, err
	}

	if minHeight > latest.Height {
		return l.root, nil
	}

	height := latest.Height - threshold
	return l.state.AtHeight(height).Head()
}

// Latest returns the latest sealed and executed block.
func (l *LatestSealedAndExecuted) Latest() (*flow.Header, error) {
	height, err := LatestSealedAndExecutedHeight(l.state, l.executedBlockDB)
	if err != nil {
		return nil, err
	}

	header, err := l.state.AtHeight(height).Head()
	if err != nil {
		return nil, err
	}

	return header, nil
}

// LatestSealedAndExecutedHeight returns the height of the latest sealed and executed block.
func LatestSealedAndExecutedHeight(state protocol.State, db *badger.DB) (uint64, error) {
	lastSealed, err := state.Sealed().Head()
	if err != nil {
		return 0, err
	}

	var blockID flow.Identifier
	var lastExecuted uint64
	err = db.View(procedure.GetLastExecutedBlock(&lastExecuted, &blockID))
	if err != nil {
		return 0, err
	}

	// the last sealed executed is min(last_sealed, last_executed)
	if lastExecuted < lastSealed.Height {
		return lastExecuted, nil
	}
	return lastSealed.Height, nil
}
