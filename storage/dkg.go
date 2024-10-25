package storage

import (
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
)

// DKGState is the storage interface for storing all artifacts and state
// related to the DKG process, including the latest state of a running or
// completed DKG, and computed beacon keys.
type DKGState interface {

	// SetDKGStarted sets the flag indicating the DKG has started for the given epoch.
	// Error returns: storage.ErrAlreadyExists
	SetDKGStarted(epochCounter uint64) error

	// GetDKGStarted checks whether the DKG has been started for the given epoch.
	// No errors expected during normal operation.
	GetDKGStarted(epochCounter uint64) (bool, error)

	// SetDKGEndState stores that the DKG has ended, and its end state.
	// Error returns: storage.ErrAlreadyExists
	SetDKGEndState(epochCounter uint64, endState flow.DKGEndState) error

	// GetDKGEndState retrieves the end state for the given DKG.
	// Error returns: storage.ErrNotFound
	GetDKGEndState(epochCounter uint64) (flow.DKGEndState, error)

	// InsertMyBeaconPrivateKey stores the random beacon private key for an epoch.
	//
	// CAUTION: these keys are stored before they are validated against the
	// canonical key vector and may not be valid for use in signing. Use SafeBeaconKeys
	// to guarantee only keys safe for signing are returned
	// Error returns: storage.ErrAlreadyExists
	InsertMyBeaconPrivateKey(epochCounter uint64, key crypto.PrivateKey) error

	// RetrieveMyBeaconPrivateKey retrieves the random beacon private key for an epoch.
	//
	// CAUTION: these keys are stored before they are validated against the
	// canonical key vector and may not be valid for use in signing. Use SafeBeaconKeys
	// to guarantee only keys safe for signing are returned
	// Error returns: storage.ErrNotFound
	RetrieveMyBeaconPrivateKey(epochCounter uint64) (crypto.PrivateKey, error)
}

// SafeBeaconKeys is a safe way to access beacon keys.
type SafeBeaconKeys interface {

	// RetrieveMyBeaconPrivateKey retrieves my beacon private key for the given
	// epoch, only if my key has been confirmed valid and safe for use.
	//
	// Returns:
	//   - (key, true, nil) if the key is present and confirmed valid
	//   - (nil, false, nil) if the key has been marked invalid or unavailable
	//     -> no beacon key will ever be available for the epoch in this case
	//   - (nil, false, storage.ErrNotFound) if the DKG has not ended
	//   - (nil, false, error) for any unexpected exception
	RetrieveMyBeaconPrivateKey(epochCounter uint64) (key crypto.PrivateKey, safe bool, err error)
}

// EpochRecoveryMyBeaconKey is a specific interface that allows to overwrite the beacon private key for given epoch.
// This interface is used *ONLY* in the epoch recovery process and only by the consensus participants.
// Each consensus participant takes part in the DKG, after finishing the DKG protocol each replica obtains a random beacon
// private key which is stored in the database along with DKG end state which will be equal to flow.DKGEndStateSuccess.
// If for any reason DKG fails, then the private key will be nil and DKG end state will be equal to flow.DKGEndStateDKGFailure.
// It's not a problem by itself, but when the epoch recovery takes place, we need to query last valid beacon private key for
// the current replica and set it for recovered epoch, otherwise replicas won't be able to vote for blocks in the recovered epoch.
type EpochRecoveryMyBeaconKey interface {
	SafeBeaconKeys

	// OverwriteMyBeaconPrivateKey overwrites the random beacon private key for the epoch that recovers the protocol from
	// epoch fallback mode. Effectively, this function overwrites whatever might be available in the database with
	// given private key for current consensus participant.
	// No errors are expected during normal operations.
	OverwriteMyBeaconPrivateKey(epochCounter uint64, key crypto.PrivateKey) error
}
