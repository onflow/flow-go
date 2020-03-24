// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertIdentity(identity *flow.Identity) func(*badger.Txn) error {
	return insert(makePrefix(codeIdentity, identity.NodeID), identity)
}

func CheckIdentity(nodeID flow.Identifier, exists *bool) func(*badger.Txn) error {
	return check(makePrefix(codeIdentity, nodeID), exists)
}

func RetrieveIdentity(nodeID flow.Identifier, identity *flow.Identity) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIdentity, nodeID), identity)
}

func IndexIdentityPayload(height uint64, blockID flow.Identifier, parentID flow.Identifier, nodeIDs []flow.Identifier) func(*badger.Txn) error {
	return insert(toPayloadIndex(codeIndexIdentity, height, blockID, parentID), nodeIDs)
}

func LookupIdentityPayload(height uint64, blockID flow.Identifier, parentID flow.Identifier, nodeIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(toPayloadIndex(codeIndexIdentity, height, blockID, parentID), nodeIDs)
}
