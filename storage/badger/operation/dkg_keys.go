package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/encodable"
)

func InsertMyBeaconPrivateKey(epochCounter uint64, info *encodable.RandomBeaconPrivKey) func(*badger.Txn) error {
	return insert(makePrefix(codeBeaconPrivateKey, epochCounter), info)
}

func RetrieveMyBeaconPrivateKey(epochCounter uint64, info *encodable.RandomBeaconPrivKey) func(*badger.Txn) error {
	return retrieve(makePrefix(codeBeaconPrivateKey, epochCounter), info)
}
