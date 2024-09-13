package operation

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
)

// InsertSafetyData inserts safety data into the database.
func InsertSafetyData(chainID flow.ChainID, safetyData *hotstuff.SafetyData) func(pebble.Writer) error {
	return insert(makePrefix(codeSafetyData, chainID), safetyData)
}

// UpdateSafetyData updates safety data in the database.
func UpdateSafetyData(chainID flow.ChainID, safetyData *hotstuff.SafetyData) func(pebble.Writer) error {
	return InsertSafetyData(chainID, safetyData)
}

// RetrieveSafetyData retrieves safety data from the database.
func RetrieveSafetyData(chainID flow.ChainID, safetyData *hotstuff.SafetyData) func(pebble.Reader) error {
	return retrieve(makePrefix(codeSafetyData, chainID), safetyData)
}

// InsertLivenessData inserts liveness data into the database.
func InsertLivenessData(chainID flow.ChainID, livenessData *hotstuff.LivenessData) func(pebble.Writer) error {
	return insert(makePrefix(codeLivenessData, chainID), livenessData)
}

// UpdateLivenessData updates liveness data in the database.
func UpdateLivenessData(chainID flow.ChainID, livenessData *hotstuff.LivenessData) func(pebble.Writer) error {
	return InsertLivenessData(chainID, livenessData)
}

// RetrieveLivenessData retrieves liveness data from the database.
func RetrieveLivenessData(chainID flow.ChainID, livenessData *hotstuff.LivenessData) func(pebble.Reader) error {
	return retrieve(makePrefix(codeLivenessData, chainID), livenessData)
}
