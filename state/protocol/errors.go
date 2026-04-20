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
	ErrNextEpochNotCommitted = fmt.Errorf("next epoch has not yet been committed")

	// ErrNextEpochAlreadyCommitted is a sentinel error returned when code tries
	// to retrieve an uncommitted TentativeEpoch during the EpochCommitted phase
	ErrNextEpochAlreadyCommitted = fmt.Errorf("retrieving tentative epoch data when epoch is already committed")

	// ErrUnknownEpochBoundary is a sentinel returned when a query is made for an
	// epoch boundary which is unknown to this node.
	//
	// There are 2 cases where an epoch boundary can be unknown.
	// Consider an epoch boundary between epoch N and epoch M=N+1.
	// Let:
	//   - n be the final block in epoch N
	//   - m be the first block in epoch M
	//   - r be this node's lowest known block
	//   - f be this node's latest finalized block
	//
	// CASE 1: `r.Height > n.Height`
	// The boundary occurred before this node's lowest known block
	// Note that this includes the case where `r == m` (we know the first block
	// of epoch M but not the final block of epoch N).
	//
	// CASE 2: `f.Height < m.Height`
	// The boundary has not been finalized yet. Note that we may have finalized
	// n but not m.
	ErrUnknownEpochBoundary = fmt.Errorf("unknown epoch boundary for current chain state")

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

func NewInvalidBlockTimestamp(msg string, args ...any) error {
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
func NewInvalidServiceEventErrorf(msg string, args ...any) error {
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

func NewUnfinalizedSealingSegmentErrorf(msg string, args ...any) error {
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
