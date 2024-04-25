package protocol

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state"
)

var (
	// ErrNoPreviousEpoch is a sentinel error returned when a previous epoch is
	// queried from a snapshot within the first epoch after the root block.
	ErrNoPreviousEpoch = fmt.Errorf("no previous epoch exists")

	// ErrNextEpochNotSetup is a sentinel error returned when the next epoch
	// has not been set up yet.
	ErrNextEpochNotSetup = fmt.Errorf("next epoch has not yet been set up")

	// ErrNextEpochNotCommitted is a sentinel error returned when the next epoch
	// has not been committed and information is queried that is only accessible
	// in the EpochCommitted phase.
	ErrNextEpochNotCommitted = fmt.Errorf("queried info from EpochCommit event before it was emitted")

	// ErrEpochTransitionNotFinalized is a sentinel returned when a query is made
	// for an epoch boundary which has not been finalized by this node.
	// We say an epoch boundary between epoch N and M=N+1 is finalized when the first
	// block within epoch M is finalized.
	// There are two cases where this can occur:
	//   1. Epoch M is in the future and no block has been finalized within it yet.
	//   2. Epoch M is in the past, but its boundary occurred prior to the start of this node's local history.
	// TODO add badger/state test cases
	ErrEpochTransitionNotFinalized = fmt.Errorf("cannot query block at un-finalized epoch transition")

	// ErrSealingSegmentBelowRootBlock is a sentinel error returned for queries
	// for a sealing segment below the root block (local history cutoff).
	ErrSealingSegmentBelowRootBlock = fmt.Errorf("cannot construct sealing segment beyond locally known history")

	// ErrClusterNotFound is a sentinel error returns for queries for a cluster
	ErrClusterNotFound = fmt.Errorf("could not find cluster")

	// ErrMultipleSealsForSameHeight indicates that an (unordered) slice of seals
	// contains two or more seals for the same block height (possibilities include
	// duplicated seals or seals for different blocks at the same height).
	ErrMultipleSealsForSameHeight = fmt.Errorf("multiple seals for same block height")

	// ErrDiscontinuousSeals indicates that an (unordered) slice of seals skips at least one block height.
	ErrDiscontinuousSeals = fmt.Errorf("seals have discontinuity, i.e. they skip some block heights")
)

type IdentityNotFoundError struct {
	NodeID flow.Identifier
}

func (e IdentityNotFoundError) Error() string {
	return fmt.Sprintf("identity not found (%x)", e.NodeID)
}

func IsIdentityNotFound(err error) bool {
	var errIdentityNotFound IdentityNotFoundError
	return errors.As(err, &errIdentityNotFound)
}

type InvalidBlockTimestampError struct {
	error
}

func (e InvalidBlockTimestampError) Unwrap() error {
	return e.error
}

func (e InvalidBlockTimestampError) Error() string {
	return e.error.Error()
}

func IsInvalidBlockTimestampError(err error) bool {
	var errInvalidTimestampError InvalidBlockTimestampError
	return errors.As(err, &errInvalidTimestampError)
}

func NewInvalidBlockTimestamp(msg string, args ...interface{}) error {
	return InvalidBlockTimestampError{
		error: fmt.Errorf(msg, args...),
	}
}

// InvalidServiceEventError indicates an invalid service event was processed.
type InvalidServiceEventError struct {
	error
}

func (e InvalidServiceEventError) Unwrap() error {
	return e.error
}

func IsInvalidServiceEventError(err error) bool {
	var errInvalidServiceEventError InvalidServiceEventError
	return errors.As(err, &errInvalidServiceEventError)
}

// NewInvalidServiceEventErrorf returns an invalid service event error. Since all invalid
// service events indicate an invalid extension, the service event error is wrapped in
// the invalid extension error at construction.
func NewInvalidServiceEventErrorf(msg string, args ...interface{}) error {
	return state.NewInvalidExtensionErrorf(
		"cannot extend state with invalid service event: %w",
		InvalidServiceEventError{
			error: fmt.Errorf(msg, args...),
		},
	)
}

// UnfinalizedSealingSegmentError indicates that including unfinalized blocks
// in the sealing segment is illegal.
type UnfinalizedSealingSegmentError struct {
	error
}

func NewUnfinalizedSealingSegmentErrorf(msg string, args ...interface{}) error {
	return UnfinalizedSealingSegmentError{
		error: fmt.Errorf(msg, args...),
	}
}

func (e UnfinalizedSealingSegmentError) Unwrap() error {
	return e.error
}

func (e UnfinalizedSealingSegmentError) Error() string {
	return e.error.Error()
}

// IsUnfinalizedSealingSegmentError returns true if err is of type UnfinalizedSealingSegmentError
func IsUnfinalizedSealingSegmentError(err error) bool {
	var e UnfinalizedSealingSegmentError
	return errors.As(err, &e)
}
