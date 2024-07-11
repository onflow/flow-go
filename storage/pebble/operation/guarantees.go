package operation

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
)

func InsertGuarantee(collID flow.Identifier, guarantee *flow.CollectionGuarantee) func(pebble.Writer) error {
	return insert(makePrefix(codeGuarantee, collID), guarantee)
}

func RetrieveGuarantee(collID flow.Identifier, guarantee *flow.CollectionGuarantee) func(pebble.Reader) error {
	return retrieve(makePrefix(codeGuarantee, collID), guarantee)
}

func IndexPayloadGuarantees(blockID flow.Identifier, guarIDs []flow.Identifier) func(pebble.Writer) error {
	return insert(makePrefix(codePayloadGuarantees, blockID), guarIDs)
}

func LookupPayloadGuarantees(blockID flow.Identifier, guarIDs *[]flow.Identifier) func(pebble.Reader) error {
	return retrieve(makePrefix(codePayloadGuarantees, blockID), guarIDs)
}
