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

func IndexIdentity(payloadHash flow.Identifier, index uint64, nodeID flow.Identifier) func(*badger.Txn) error {
	//BadgerDB iterates the keys in order - https://github.com/dgraph-io/badger/blob/master/iterator.go#L710
	//Hence adding index at the end should make us retrieve them in order
	return insert(makePrefix(codeIndexIdentity, payloadHash, index), nodeID)
}

func LookupIdentities(payloadHash flow.Identifier, nodeIDs *[]flow.Identifier) func(*badger.Txn) error {
	return traverse(makePrefix(codeIndexIdentity, payloadHash), lookup(nodeIDs))
}
