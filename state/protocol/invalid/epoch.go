package invalid

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// Epoch represents an epoch that does not exist or could not be retrieved.
type Epoch struct {
	err error
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

func (u *Epoch) InitialIdentities() (flow.IdentityList, error) {
	return nil, u.err
}

func (u *Epoch) Clustering() (flow.ClusterList, error) {
	return nil, u.err
}

func (u *Epoch) Cluster(uint) (protocol.Cluster, error) {
	return nil, u.err
}

func (u *Epoch) DKG() (protocol.DKG, error) {
	return nil, u.err
}

func (u *Epoch) RandomSource() ([]byte, error) {
	return nil, u.err
}

func NewEpoch(err error) *Epoch {
	return &Epoch{err: err}
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
