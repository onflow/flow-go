// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"
	"sort"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/seed"
)

// SetupEpoch represents an epoch that has been setup, but not committed.
// Only the EpochSetup event for the epoch has been emitted as of the point
// at which the epoch was queried.
type SetupEpoch struct {
	setupEvent *flow.EpochSetup
}

func (es *SetupEpoch) Counter() (uint64, error) {
	return es.setupEvent.Counter, nil
}

func (es *SetupEpoch) FinalView() (uint64, error) {
	return es.setupEvent.FinalView, nil
}

func (es *SetupEpoch) InitialIdentities() (flow.IdentityList, error) {

	identities := es.setupEvent.Participants.Filter(filter.Any)
	// apply a deterministic sort to the participants
	sort.Slice(identities, func(i int, j int) bool {
		return order.ByNodeIDAsc(identities[i], identities[j])
	})

	return identities, nil
}

func (es *SetupEpoch) Clustering() (flow.ClusterList, error) {

	collectorFilter := filter.And(filter.HasStake(true), filter.HasRole(flow.RoleCollection))
	clustering, err := flow.NewClusterList(es.setupEvent.Assignments, es.setupEvent.Participants.Filter(collectorFilter))
	if err != nil {
		return nil, fmt.Errorf("failed to generate ClusterList from collector identities: %w", err)
	}
	return clustering, nil
}

func (es *SetupEpoch) Cluster(index uint) (protocol.Cluster, error) {
	return nil, fmt.Errorf("EpochCommit event not yet received in fork")
}

func (es *SetupEpoch) DKG() (protocol.DKG, error) {
	return nil, fmt.Errorf("EpochCommit event not yet received in fork")
}

func (es *SetupEpoch) Seed(indices ...uint32) ([]byte, error) {
	return seed.FromRandomSource(indices, es.setupEvent.RandomSource)
}

func NewSetupEpoch(setupEvent *flow.EpochSetup) *SetupEpoch {
	return &SetupEpoch{
		setupEvent: setupEvent,
	}
}

// ****************************************

// CommittedEpoch represents an epoch that has been committed.
// Both the EpochSetup and EpochCommitted events for the epoch have been emitted
// as of the point at which the epoch was queried.
type CommittedEpoch struct {
	SetupEpoch
	commitEvent *flow.EpochCommit
}

func (es *CommittedEpoch) Cluster(index uint) (protocol.Cluster, error) {

	qcs := es.commitEvent.ClusterQCs
	if uint(len(qcs)) <= index {
		return nil, fmt.Errorf("no cluster with index %d", index)
	}
	rootQC := qcs[index]

	clustering, err := es.Clustering()
	if err != nil {
		return nil, fmt.Errorf("failed to generate clustering: %w", err)
	}

	members, ok := clustering.ByIndex(index)
	if !ok {
		return nil, fmt.Errorf("failed to get members of cluster %d: %w", index, err)
	}
	epochCounter := es.setupEvent.Counter

	inf := Cluster{
		index:     index,
		counter:   epochCounter,
		members:   members,
		rootBlock: cluster.CanonicalRootBlock(epochCounter, members),
		rootQC:    rootQC,
	}

	return &inf, nil
}

func (es *CommittedEpoch) DKG() (protocol.DKG, error) {
	return &DKG{commitEvent: es.commitEvent}, nil
}

func NewCommittedEpoch(setupEvent *flow.EpochSetup, commitEvent *flow.EpochCommit) *CommittedEpoch {
	return &CommittedEpoch{
		SetupEpoch: SetupEpoch{
			setupEvent: setupEvent,
		},
		commitEvent: commitEvent,
	}
}

// ****************************************

// InvalidEpoch represents an epoch that does not exist.
// Neither the EpochSetup nor EpochCommitted events for the epoch have been
// emitted as of the point at which the epoch was queried.
type InvalidEpoch struct {
	err error
}

func (u *InvalidEpoch) Counter() (uint64, error) {
	return 0, u.err
}

func (u *InvalidEpoch) FinalView() (uint64, error) {
	return 0, u.err
}

func (u *InvalidEpoch) InitialIdentities() (flow.IdentityList, error) {
	return nil, u.err
}

func (u *InvalidEpoch) Clustering() (flow.ClusterList, error) {
	return nil, u.err
}

func (u *InvalidEpoch) Cluster(uint) (protocol.Cluster, error) {
	return nil, u.err
}

func (u *InvalidEpoch) DKG() (protocol.DKG, error) {
	return nil, u.err
}

func (u *InvalidEpoch) Seed(...uint32) ([]byte, error) {
	return nil, u.err
}

func NewInvalidEpoch(err error) *InvalidEpoch {
	return &InvalidEpoch{err: err}
}
