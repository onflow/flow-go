package protocol_state

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type Mutator struct {
	protocolStateDB storage.ProtocolState
}

var _ protocol.StateMutator = (*Mutator)(nil)

func (m *Mutator) CreateUpdater(candidate *flow.Header) (protocol.StateUpdater, error) {
	parentState, err := m.protocolStateDB.ByBlockID(candidate.ParentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve protocol state for block (%v): %w", candidate.ParentID, err)
	}
	return newUpdater(candidate, parentState), nil
}

func (m *Mutator) CommitProtocolState(updater protocol.StateUpdater) func(tx *transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		updatedState, updatedStateID, hasChanges := updater.Build()
		if hasChanges {
			err := m.protocolStateDB.StoreTx(updatedStateID, updatedState)(tx)
			if err != nil {
				return fmt.Errorf("could not store protocol state (%v): %w", updatedStateID, err)
			}
		}

		err := m.protocolStateDB.Index(updater.Block().ID(), updatedStateID)(tx)
		if err != nil {
			return fmt.Errorf("could not index protocol state (%v) for block (%v): %w",
				updatedStateID, updater.Block().ID(), err)
		}
		return nil
	}
}
