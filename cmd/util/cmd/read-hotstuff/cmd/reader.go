package cmd

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
	"github.com/onflow/flow-go/model/flow"
)

// HotstuffReader exposes only the read-only parts of the Hotstuff Persister component.
// This is used to read information about the HotStuff instance's current state from CLI tools.
// CAUTION: the write functions are hidden here, because it is NOT SAFE to use them outside
// the Hotstuff state machine.
type HotstuffReader interface {
	// GetLivenessData retrieves the latest persisted liveness data.
	GetLivenessData() *hotstuff.LivenessData
	// GetSafetyData retrieves the latest persisted safety data.
	GetSafetyData() *hotstuff.SafetyData
}

// NewHotstuffReader returns a new Persister, constrained to read-only operations.
func NewHotstuffReader(db *badger.DB, chainID flow.ChainID) HotstuffReader {
	persist, err := persister.New(db, chainID)
	if err != nil {
		panic(err)
	}
	return persist
}
