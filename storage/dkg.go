package storage

import (
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
)

// SafeBeaconKeys is a safe way to access beacon keys.
type SafeBeaconKeys interface {

	// RetrieveMyBeaconPrivateKey retrieves my beacon private key for the given
	// epoch, only if my key has been confirmed valid and safe for use.
	//
	// Returns:
	//   - (key, true, nil) if the key is present and confirmed valid
	//   - (nil, false, nil) if the key has been marked invalid or unavailable
	//     -> no beacon key will ever be available for the epoch in this case
	//   - (nil, false, [storage.ErrNotFound]) if the DKG has not ended
	//   - (nil, false, error) for any unexpected exception
	RetrieveMyBeaconPrivateKey(epochCounter uint64) (key crypto.PrivateKey, safe bool, err error)
}

// DKGStateReader is a read-only interface for low-level reading of the Random Beacon Recoverable State Machine.
type DKGStateReader interface {
	SafeBeaconKeys

	// GetDKGState retrieves the current state of the state machine for the given epoch.
	// If an error is returned, the state is undefined meaning that state machine is in initial state
	// Error returns:
	//   - [storage.ErrNotFound] - if there is no state stored for given epoch, meaning the state machine is in initial state.
	GetDKGState(epochCounter uint64) (flow.DKGState, error)

	// IsDKGStarted checks whether the DKG has been started for the given epoch.
	// No errors expected during normal operation.
	IsDKGStarted(epochCounter uint64) (bool, error)

	// UnsafeRetrieveMyBeaconPrivateKey retrieves the random beacon private key for an epoch.
	//
	// CAUTION: these keys are stored before they are validated against the
	// canonical key vector and may not be valid for use in signing. Use SafeBeaconKeys
	// to guarantee only keys safe for signing are returned
	// Error returns:
	//   - [storage.ErrNotFound] - if there is no key stored for given epoch.
	UnsafeRetrieveMyBeaconPrivateKey(epochCounter uint64) (crypto.PrivateKey, error)
}

// DKGState is the storage interface for storing all artifacts and state related to the DKG process,
// including the latest state of a running or completed DKG, and computed beacon keys. DKGState
// supports all state transitions that can occur for an individual node during the happy path
// epoch switchover of the network as a whole. Recovery from the epoch fallback mode is supported
// by the EpochRecoveryMyBeaconKey interface.
type DKGState interface {
	DKGStateReader

	// SetDKGState performs a state transition for the Random Beacon Recoverable State Machine.
	// Some state transitions may not be possible using this method. For instance, we might not be able to enter [flow.DKGStateCompleted]
	// state directly from [flow.DKGStateStarted], even if such transition is valid. The reason for this is that some states require additional
	// data to be processed by the state machine before the transition can be made. For such cases there are dedicated methods that should be used, ex.
	// InsertMyBeaconPrivateKey and UpsertMyBeaconPrivateKey, which allow to store the needed data and perform the transition in one atomic operation.
	// Error returns:
	//   - [storage.InvalidDKGStateTransitionError] - if the requested state transition is invalid.
	SetDKGState(epochCounter uint64, newState flow.DKGState) error

	// InsertMyBeaconPrivateKey stores the random beacon private key for an epoch and transitions the
	// state machine into the [flow.DKGStateCompleted] state.
	//
	// CAUTION: these keys are stored before they are validated against the
	// canonical key vector and may not be valid for use in signing. Use [SafeBeaconKeys]
	// to guarantee only keys safe for signing are returned
	// Error returns:
	//   - [storage.ErrAlreadyExists] - if there is already a key stored for given epoch.
	//   - [storage.InvalidDKGStateTransitionError] - if the requested state transition is invalid.
	InsertMyBeaconPrivateKey(epochCounter uint64, key crypto.PrivateKey) error

// CommitMyBeaconPrivateKey commits the previously inserted random beacon private key for an epoch. Effectively, this method
// transitions the state machine into the [flow.RandomBeaconKeyCommitted] state if the current state is [flow.DKGStateCompleted].
// The caller needs to supply the [flow.EpochCommit] as evidence that the stored key is valid for the specified epoch. Repeated
// calls for the same epoch are accepted (idempotent operation),if and only if the provided EpochCommit confirms the already
// committed key.
	// No errors are expected during normal operations.
	CommitMyBeaconPrivateKey(epochCounter uint64, commit *flow.EpochCommit) error
}

// EpochRecoveryMyBeaconKey is a specific interface that allows to overwrite the beacon private key for
// a future epoch, provided that the state machine has not yet reached the [flow.RandomBeaconKeyCommitted]
// state for the specified epoch.
// This interface is used *ONLY* in the epoch recovery process and only by the consensus participants. On the happy path,
// each consensus committee member takes part in the DKG, and after successfully finishing the DKG protocol it obtains a
// random beacon private key, which is stored in the database along with DKG state [flow.DKGStateCompleted]. If for any
// reason DKG fails, then the private key will be nil and DKG end state will be equal to [flow.DKGStateFailure].
// This module allows to overwrite the random beacon private key in case of EFM recovery or other configuration issues.
type EpochRecoveryMyBeaconKey interface {
	DKGStateReader

	// UpsertMyBeaconPrivateKey overwrites the random beacon private key for the epoch that recovers the protocol
	// from Epoch Fallback Mode. The resulting state of this method call is [flow.RandomBeaconKeyCommitted].
	// State transitions are allowed if and only if the current state is not equal to [flow.RandomBeaconKeyCommitted]. 
	// Repeated calls for the same epoch are idempotent, if and only if the provided EpochCommit confirms the already
	// committed key (error otherwise).
	// No errors are expected during normal operations.
	UpsertMyBeaconPrivateKey(epochCounter uint64, key crypto.PrivateKey, commit *flow.EpochCommit) error
}
