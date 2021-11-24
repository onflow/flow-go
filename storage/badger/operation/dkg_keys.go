package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/encodable"
)

func InsertMyBeaconPrivateKey(epochCounter uint64, info *encodable.RandomBeaconPrivKey) func(*badger.Txn) error {
	return insert(makePrefix(codeDKGPrivateInfo, epochCounter), info)
}

func RetrieveMyBeaconPrivateKey(epochCounter uint64, info *encodable.RandomBeaconPrivKey) func(*badger.Txn) error {
	return retrieve(makePrefix(codeDKGPrivateInfo, epochCounter), info)
}
