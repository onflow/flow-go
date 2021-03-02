package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/dkg"
)

func InsertMyDKGPrivateInfo(epochCounter uint64, info *dkg.DKGParticipantPriv) func(*badger.Txn) error {
	return insert(makePrefix(codeDKGPrivateInfo, epochCounter), info)
}

func RetrieveMyDKGPrivateInfo(epochCounter uint64, info *dkg.DKGParticipantPriv) func(*badger.Txn) error {
	return retrieve(makePrefix(codeDKGPrivateInfo, epochCounter), info)
}
