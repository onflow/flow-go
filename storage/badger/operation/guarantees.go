package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertGuarantee(collID flow.Identifier, guarantee *flow.CollectionGuarantee) func(*badger.Txn) error {
	return insert(makePrefix(codeGuarantee, collID), guarantee)
}

func RetrieveGuarantee(collID flow.Identifier, guarantee *flow.CollectionGuarantee) func(*badger.Txn) error {
	return retrieve(makePrefix(codeGuarantee, collID), guarantee)
}

func IndexPayloadGuarantees(blockID flow.Identifier, guarIDs []flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codePayloadGuarantees, blockID), guarIDs)
}

func LookupPayloadGuarantees(blockID flow.Identifier, guarIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codePayloadGuarantees, blockID), guarIDs)
}
