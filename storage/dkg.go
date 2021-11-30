package storage

import (
	"github.com/onflow/flow-go/model/encodable"
)

type BeaconPrivateKeys interface {
	InsertMyBeaconPrivateKey(epochCounter uint64, key *encodable.RandomBeaconPrivKey) error
	RetrieveMyBeaconPrivateKey(epochCounter uint64) (*encodable.RandomBeaconPrivKey, error)
}

// DKGState is the storage interface for the state of in-progress and completed
// DKG instances.
type DKGState interface {

	// SetDKGStarted sets the flag indicating the DKG has started for the given epoch.
	SetDKGStarted(epochCounter uint64) error

	// GetDKGStarted checks whether the DKG has been started for the given epoch.
	GetDKGStarted(epochCounter uint64) (bool, error)
}
