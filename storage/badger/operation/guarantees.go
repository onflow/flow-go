package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertGuarantee(guarantee *flow.CollectionGuarantee) func(*badger.Txn) error {
	return insert(makePrefix(codeGuarantee, guarantee.CollectionID), guarantee)
}

func CheckGuarantee(guaranteeID flow.Identifier, exists *bool) func(*badger.Txn) error {
	return check(makePrefix(codeGuarantee, guaranteeID), exists)
}

func RetrieveGuarantee(guaranteeID flow.Identifier, guarantee *flow.CollectionGuarantee) func(*badger.Txn) error {
	return retrieve(makePrefix(codeGuarantee, guaranteeID), guarantee)
}

func IndexGuaranteePayload(blockID flow.Identifier, guaranteeIDs []flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexGuarantee, blockID), guaranteeIDs)
}

func LookupGuaranteePayload(blockID flow.Identifier, guaranteeIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexGuarantee, blockID), guaranteeIDs)
}
