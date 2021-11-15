package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/dkg"
)

// InsertMyDKGPrivateInfo stores the random beacon private key for the given epoch.
//
// CAUTION: This method stores confidential information and should only be
// used in the context of the secrets database. This is enforced in the above
// layer (see storage.DKGKeys).
func InsertMyDKGPrivateInfo(epochCounter uint64, info *dkg.DKGParticipantPriv) func(*badger.Txn) error {
	return insert(makePrefix(codeDKGPrivateInfo, epochCounter), info)
}

// RetrieveMyDKGPrivateInfo retrieves the random beacon private key for the given epoch.
//
// CAUTION: This method stores confidential information and should only be
// used in the context of the secrets database. This is enforced in the above
// layer (see storage.DKGKeys).
func RetrieveMyDKGPrivateInfo(epochCounter uint64, info *dkg.DKGParticipantPriv) func(*badger.Txn) error {
	return retrieve(makePrefix(codeDKGPrivateInfo, epochCounter), info)
}
