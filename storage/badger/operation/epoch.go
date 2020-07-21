package operation

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dgraph-io/badger/v2"
)

func InsertEpochCounter(counter uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeEpochCounter), counter)
}

func UpdateEpochCounter(counter uint64) func(*badger.Txn) error {
	return update(makePrefix(codeEpochCounter), counter)
}

func RetrieveEpochCounter(counter *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeEpochCounter), counter)
}

func InsertEpochSeed(counter uint64, seed []byte) func(*badger.Txn) error {
	return insert(makePrefix(codeEpochSeed, counter), seed)
}

func RetrieveEpochSeed(counter uint64, seed *[]byte) func(*badger.Txn) error {
	return insert(makePrefix(codeEpochSeed, counter), seed)
}

func InsertEpochEnd(counter uint64, view uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeEpochEnd, counter), view)
}

func RetrieveEpochEnd(counter uint64, view *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeEpochEnd, counter), view)
}

func InsertEpochClusters(counter uint64, clusters []flow.IdentityList) func(*badger.Txn) error {
	return insert(makePrefix(codeEpochClusters, counter), clusters)
}

func RetrieveEpochClusters(counter uint64, clusters *[]flow.IdentityList) func(*badger.Txn) error {
	return retrieve(makePrefix(codeEpochClusters, counter), clusters)
}

func InsertEpochIdentities(counter uint64, identities flow.IdentityList) func(*badger.Txn) error {
	return insert(makePrefix(codeEpochIdentities, counter), identities)
}

func RetrieveEpochIdentities(counter uint64, identities *flow.IdentityList) func(*badger.Txn) error {
	return retrieve(makePrefix(codeEpochIdentities, counter), identities)
}
