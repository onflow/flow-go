package invalid

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
)

// Epoch represents an epoch that does not exist or could not be retrieved.
type Epoch struct {
	err error
}

// NewEpoch returns a new invalid epoch, containing an error describing why the
// epoch could not be retrieved. The following are expected errors when constructing
// an invalid Epoch:
//   - protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
//     This happens when the previous epoch is queried within the first epoch of a spork.
//   - protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
//     This happens when the next epoch is queried within the EpochStaking phase of any epoch.
//   - state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
//   - generic error in case of unexpected critical internal corruption or bugs
func NewEpoch(err error) *Epoch {
	if errors.Is(err, protocol.ErrNoPreviousEpoch) {
		return &Epoch{err: err}
	}
	if errors.Is(err, protocol.ErrNextEpochNotSetup) {
		return &Epoch{err: err}
	}
	if errors.Is(err, state.ErrUnknownSnapshotReference) {
		return &Epoch{err: err}
	}
	return &Epoch{err: fmt.Errorf("critical unexpected error querying epoch: %w", err)}
}

// NewEpochf is NewEpoch with ergonomic error formatting.
func NewEpochf(msg string, args ...interface{}) *Epoch {
	return NewEpoch(fmt.Errorf(msg, args...))
}

func (u *Epoch) Counter() (uint64, error) {
	return 0, u.err
}

func (u *Epoch) FirstView() (uint64, error) {
	return 0, u.err
}

func (u *Epoch) FinalView() (uint64, error) {
	return 0, u.err
}

func (u *Epoch) DKGPhase1FinalView() (uint64, error) {
	return 0, u.err
}

func (u *Epoch) DKGPhase2FinalView() (uint64, error) {
	return 0, u.err
}

func (u *Epoch) DKGPhase3FinalView() (uint64, error) {
	return 0, u.err
}

func (u *Epoch) InitialIdentities() (flow.IdentitySkeletonList, error) {
	return nil, u.err
}

func (u *Epoch) Clustering() (flow.ClusterList, error) {
	return nil, u.err
}

func (u *Epoch) Cluster(uint) (protocol.Cluster, error) {
	return nil, u.err
}

func (u *Epoch) ClusterByChainID(chainID flow.ChainID) (protocol.Cluster, error) {
	return nil, u.err
}

func (u *Epoch) DKG() (protocol.DKG, error) {
	return nil, u.err
}

func (u *Epoch) RandomSource() ([]byte, error) {
	return nil, u.err
}

func (u *Epoch) FirstHeight() (uint64, error) {
	return 0, u.err
}

func (u *Epoch) FinalHeight() (uint64, error) {
	return 0, u.err
}

// Epochs is an epoch query for an invalid snapshot.
type Epochs struct {
	err error
}

func (u *Snapshot) Epochs() protocol.EpochQuery {
	return &Epochs{err: u.err}
}

func (u *Epochs) Current() protocol.Epoch {
	return NewEpoch(u.err)
}

func (u *Epochs) Next() protocol.Epoch {
	return NewEpoch(u.err)
}

func (u *Epochs) Previous() protocol.Epoch {
	return NewEpoch(u.err)
}
