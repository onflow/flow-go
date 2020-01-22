package operation

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertGuarantee(cg *flow.CollectionGuarantee) func(*badger.Txn) error {
	return insert(makePrefix(codeGuarantee, cg.ID()), cg)
}

func PersistGuarantee(cg *flow.CollectionGuarantee) func(*badger.Txn) error {
	return persist(makePrefix(codeGuarantee, cg.ID()), cg)
}

func RetrieveGuarantee(collID flow.Identifier, cg *flow.CollectionGuarantee) func(*badger.Txn) error {
	return retrieve(makePrefix(codeGuarantee, collID), cg)
}

func IndexGuarantee(blockID flow.Identifier, cg *flow.CollectionGuarantee) func(*badger.Txn) error {
	return persist(makePrefix(codeCollectionIndex, blockID, cg.ID()), cg.ID())
}

func RetrieveGuarantees(blockID flow.Identifier, guarantees *[]*flow.CollectionGuarantee) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		iter := func() (checkFunc, createFunc, handleFunc) {

			check := func(key []byte) bool {
				return true
			}

			var collID flow.Identifier
			create := func() interface{} {
				return &collID
			}

			handle := func() error {

				if collID == flow.ZeroID {
					return fmt.Errorf("collection ID was not decoded")
				}

				var guarantee flow.CollectionGuarantee
				err := RetrieveGuarantee(collID, &guarantee)(tx)
				if err != nil {
					return fmt.Errorf("failed to retrieve guarantee: %w", err)
				}

				*guarantees = append(*guarantees, &guarantee)

				return nil
			}

			return check, create, handle
		}

		return traverse(makePrefix(codeCollectionIndex, blockID), iter)(tx)
	}
}
