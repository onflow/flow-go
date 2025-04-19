package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func UnsafeInsertGuarantee(w storage.Writer, collID flow.Identifier, guarantee *flow.CollectionGuarantee) error {
	return UpsertByKey(w, MakePrefix(codeGuarantee, collID), guarantee)
}

func RetrieveGuarantee(r storage.Reader, collID flow.Identifier, guarantee *flow.CollectionGuarantee) error {
	return RetrieveByKey(r, MakePrefix(codeGuarantee, collID), guarantee)
}

func UnsafeIndexPayloadGuarantees(w storage.Writer, blockID flow.Identifier, guarIDs []flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codePayloadGuarantees, blockID), guarIDs)
}

func LookupPayloadGuarantees(r storage.Reader, blockID flow.Identifier, guarIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codePayloadGuarantees, blockID), guarIDs)
}
