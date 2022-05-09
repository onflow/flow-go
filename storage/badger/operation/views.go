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

// UpdateSafetyData updates safety data in the database.
func UpdateSafetyData(chainID flow.ChainID, safetyData *hotstuff.SafetyData) func(*badger.Txn) error {
	return update(makePrefix(codeSafetyData, chainID), safetyData)
}

// RetrieveSafetyData retrieves safety data from the database.
func RetrieveSafetyData(chainID flow.ChainID, safetyData *hotstuff.SafetyData) func(*badger.Txn) error {
	return retrieve(makePrefix(codeSafetyData, chainID), safetyData)
}

// InsertLivenessData inserts liveness data into the database.
func InsertLivenessData(chainID flow.ChainID, livenessData *hotstuff.LivenessData) func(*badger.Txn) error {
	return insert(makePrefix(codeLivenessData, chainID), livenessData)
}

// UpdateLivenessData updates liveness data in the database.
func UpdateLivenessData(chainID flow.ChainID, livenessData *hotstuff.LivenessData) func(*badger.Txn) error {
	return update(makePrefix(codeLivenessData, chainID), livenessData)
}

// RetrieveLivenessData retrieves liveness data from the database.
func RetrieveLivenessData(chainID flow.ChainID, livenessData *hotstuff.LivenessData) func(*badger.Txn) error {
	return retrieve(makePrefix(codeLivenessData, chainID), livenessData)
}
