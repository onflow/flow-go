package testutil

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

func UncheckedState(db *badger.DB, identities flow.IdentityList) (*protocol.State, error) {

	genesis := flow.Genesis(identities)

	err := db.Update(func(txn *badger.Txn) error {
		return procedure.Bootstrap(genesis)(txn)
	})
	if err != nil {
		return nil, fmt.Errorf("could not bootstrap: %w", err)
	}

	state, err := protocol.NewState(db)
	if err != nil {
		return nil, fmt.Errorf("could not initialize state: %w", err)
	}

	return state, nil
}
