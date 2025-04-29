package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
)

// InsertSafetyData inserts safety data into the database.
func InsertSafetyData(chainID flow.ChainID, safetyData *hotstuff.SafetyData) func(*badger.Txn) error {
	return insert(makePrefix(codeSafetyData, chainID), safetyData)
}

// InsertLivenessData inserts liveness data into the database.
func InsertLivenessData(chainID flow.ChainID, livenessData *hotstuff.LivenessData) func(*badger.Txn) error {
	return insert(makePrefix(codeLivenessData, chainID), livenessData)
}
