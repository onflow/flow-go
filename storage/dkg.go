package storage

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
)

type BeaconPrivateKeys interface {
	InsertMyBeaconPrivateKey(epochCounter uint64, key *encodable.RandomBeaconPrivKey) error
	RetrieveMyBeaconPrivateKey(epochCounter uint64) (*encodable.RandomBeaconPrivKey, error)
}

// DKGState is the storage interface for storing all artifacts and state
// related to the DKG process, including the latest state of a running or
// completed DKG, and computed beacon keys.
type DKGState interface {

	// SetDKGStarted sets the flag indicating the DKG has started for the given epoch.
	SetDKGStarted(epochCounter uint64) error

	// GetDKGStarted checks whether the DKG has been started for the given epoch.
	GetDKGStarted(epochCounter uint64) (bool, error)

	// SetDKGEndState stores that the DKG has ended, and its end state.
	SetDKGEndState(epochCounter uint64, endState flow.DKGEndState) error

	// InsertMyBeaconPrivateKey stores the random beacon private key for an epoch.
	//
	// CAUTION: these keys are stored before they are validated against the
	// canonical key vector and may not be valid for use in signing. Use SafeBeaconKeys
	// to guarantee only keys safe for signing are returned
	InsertMyBeaconPrivateKey(epochCounter uint64, key crypto.PrivateKey) error

	// RetrieveMyBeaconPrivateKey retrieves the random beacon private key for an epoch.
	//
	// CAUTION: these keys are stored before they are validated against the
	// canonical key vector and may not be valid for use in signing. Use SafeBeaconKeys
	// to guarantee only keys safe for signing are returned
	RetrieveMyBeaconPrivateKey(epochCounter uint64) (crypto.PrivateKey, error)
}

// SafeBeaconKeys is a safe way to access beacon keys.
type SafeBeaconKeys interface {

	// RetrieveMyBeaconPrivateKey retrieves my beacon private key for the given
	// epoch, only if my key has been confirmed valid and safe for use.
	//
	// Returns:
	// * (key, true, nil) if the key is present and confirmed valid
	// * (nil, false, nil) if the key has been marked invalid (by SetDKGEnded)
	// * (nil, false, error) for any other condition, or exception
	RetrieveMyBeaconPrivateKey(epochCounter uint64) (key crypto.PrivateKey, safe bool, err error)
}
