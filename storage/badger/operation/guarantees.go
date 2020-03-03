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

func IndexGuaranteePayload(height uint64, blockID flow.Identifier, parentID flow.Identifier, guaranteeIDs []flow.Identifier) func(*badger.Txn) error {
	return insert(toPayloadIndex(codeIndexGuarantee, height, blockID, parentID), guaranteeIDs)
}

func LookupGuaranteePayload(height uint64, blockID flow.Identifier, parentID flow.Identifier, collIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(toPayloadIndex(codeIndexGuarantee, height, blockID, parentID), collIDs)
}

func VerifyGuaranteePayload(height uint64, blockID flow.Identifier, collIDs []flow.Identifier) func(*badger.Txn) error {
	return iterate(makePrefix(codeIndexGuarantee, height), makePrefix(codeIndexGuarantee, uint64(0)), verifypayload(blockID, collIDs))
}
