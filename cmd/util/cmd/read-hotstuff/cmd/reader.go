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
	GetLivenessData() (*hotstuff.LivenessData, error)
	// GetSafetyData retrieves the latest persisted safety data.
	GetSafetyData() (*hotstuff.SafetyData, error)
}

// NewHotstuffReader returns a new Persister, constrained to read-only operations.
func NewHotstuffReader(db *badger.DB, chainID flow.ChainID) HotstuffReader {
	return persister.New(db, chainID)
}
