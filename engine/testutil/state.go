package testutil

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/cache"
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

func UncheckedState(db *badger.DB, commit flow.StateCommitment, participants flow.IdentityList) (*protocol.State, error) {

	genesis := flow.Genesis(participants)

	err := db.Update(func(txn *badger.Txn) error {
		return procedure.Bootstrap(commit, genesis)(txn)
	})
	if err != nil {
		return nil, fmt.Errorf("could not bootstrap: %w", err)
	}

	pcache, err := cache.NewPayloadCache(db)
	if err != nil {
		return nil, fmt.Errorf("could not load payload cache: %w", err)
	}

	state, err := protocol.NewState(db, pcache)
	if err != nil {
		return nil, fmt.Errorf("could not initialize state: %w", err)
	}

	return state, nil
}
