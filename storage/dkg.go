package storage

import (
	"github.com/onflow/flow-go/model/dkg"
)

// DKGKeys is the storage interface for storing random beacon private keys
// resulting from the DKG.
type DKGKeys interface {
	InsertMyDKGPrivateInfo(epochCounter uint64, key *dkg.DKGParticipantPriv) error
	RetrieveMyDKGPrivateInfo(epochCounter uint64) (*dkg.DKGParticipantPriv, error)
}

// DKGState is the storage interface for the state of in-progress and completed
// DKG instances.
type DKGState interface {

	// DKGStarted sets the flag indicating the DKG has started for the given epoch.
	DKGStarted(epochCounter uint64) error
}
