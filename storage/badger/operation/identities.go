// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertIdentity(nodeID flow.Identifier, identity *flow.Identity) func(*badger.Txn) error {
	return insert(makePrefix(codeIdentity, nodeID), identity)
}

func RetrieveIdentity(nodeID flow.Identifier, identity *flow.Identity) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIdentity, nodeID), identity)
}

func IndexPayloadIdentities(blockID flow.Identifier, nodeIDs []flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codePayloadIdentities, blockID), nodeIDs)
}

func LookupPayloadIdentities(blockID flow.Identifier, nodeIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codePayloadIdentities, blockID), nodeIDs)
}
