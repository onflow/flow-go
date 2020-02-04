package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertGuarantee(guarantee *flow.CollectionGuarantee) func(*badger.Txn) error {
	return insert(makePrefix(codeGuarantee, guarantee.CollectionID), guarantee)
}

func CheckGuarantee(collID flow.Identifier, exists *bool) func(*badger.Txn) error {
	return check(makePrefix(codeGuarantee, collID), exists)
}

func RetrieveGuarantee(collID flow.Identifier, guarantee *flow.CollectionGuarantee) func(*badger.Txn) error {
	return retrieve(makePrefix(codeGuarantee, collID), guarantee)
}

func IndexGuarantee(payloadHash flow.Identifier, guaranteeID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexGuarantee, payloadHash, guaranteeID), guaranteeID)
}

func LookupGuarantees(payloadHash flow.Identifier, collIDs *[]flow.Identifier) func(*badger.Txn) error {
	return traverse(makePrefix(codeIndexGuarantee, payloadHash), lookup(collIDs))
}
