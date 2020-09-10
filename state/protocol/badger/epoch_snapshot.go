// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"
	"sort"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/flow/order"
	"github.com/dapperlabs/flow-go/state/cluster"
	"github.com/dapperlabs/flow-go/state/protocol"
)

// EpochSetupSnapshot represents a read-only immutable snapshot of the protocol state
// that contains only an EpochSetup Event.
type EpochSetupSnapshot struct {
	header     *flow.Header
	setupEvent *flow.EpochSetup
}

func (es *EpochSetupSnapshot) Counter() (uint64, error) {
	return es.setupEvent.Counter, nil
}

func (es *EpochSetupSnapshot) Head() (*flow.Header, error) {
	return es.header, nil
}

func (es *EpochSetupSnapshot) FinalView() (uint64, error) {
	return es.setupEvent.FinalView, nil
}

func (es *EpochSetupSnapshot) InitialIdentities(selector flow.IdentityFilter) (flow.IdentityList, error) {
	identities := es.setupEvent.Participants.Filter(selector)

	// apply a deterministic sort to the participants
	sort.Slice(identities, func(i int, j int) bool {
		return order.ByNodeIDAsc(identities[i], identities[j])
	})

	return identities, nil
}

func (es *EpochSetupSnapshot) Clustering() (flow.ClusterList, error) {
	collectorFilter := filter.And(filter.HasStake(true), filter.HasRole(flow.RoleCollection))
	clustering, err := flow.NewClusterList(es.setupEvent.Assignments, es.setupEvent.Participants.Filter(collectorFilter))
	if err != nil {
		return nil, fmt.Errorf("failed to generate ClusterList from collector identities: %w", err)
	}
	return clustering, nil
}

func (es *EpochSetupSnapshot) ClusterInformation(index uint32) (protocol.ClusterInformation, error) {
	return nil, fmt.Errorf("EpochCommit event not yet received in fork")
}

func (es *EpochSetupSnapshot) DKG() (protocol.DKG, error) {
	return nil, fmt.Errorf("EpochCommit event not yet received in fork")
}

func (es *EpochSetupSnapshot) EpochSetupSeed(indices ...uint32) ([]byte, error) {
	return protocol.SeedFromSourceOfRandomness(indices, es.setupEvent.SourceOfRandomness)
}

func (es *EpochSetupSnapshot) Phase() (protocol.EpochPhase, error) {
	return protocol.EpochSetupPhase, nil
}

func NewEpochSetupSnapshot(header *flow.Header, setupEvent *flow.EpochSetup) *EpochSetupSnapshot {
	return &EpochSetupSnapshot{
		header:     header,
		setupEvent: setupEvent,
	}
}

// ****************************************

// EpochCommitSnapshot represents a read-only immutable snapshot of the protocol state
// that contains an EpochSetup _and_ EpochCommit Event.
type EpochCommitSnapshot struct {
	EpochSetupSnapshot
	commitEvent *flow.EpochCommit
}

func (es *EpochCommitSnapshot) ClusterInformation(index uint32) (protocol.ClusterInformation, error) {
	qcs := es.commitEvent.ClusterQCs
	if uint32(len(qcs)) <= index {
		return nil, fmt.Errorf("no cluster with index %d", index)
	}
	rootQC := qcs[index]

	clustering, err := es.Clustering()
	if err != nil {
		return nil, fmt.Errorf("failed to generate clustering: %w", err)
	}

	members, ok := clustering.ByIndex(uint(index))
	if !ok {
		return nil, fmt.Errorf("failed to get members of cluster %d: %w", index, err)
	}
	epochCounter := es.setupEvent.Counter

	inf := ClusterInformation{
		index:     index,
		counter:   epochCounter,
		members:   members,
		rootBlock: cluster.CanonicalClusterRootBlock(epochCounter, members),
		rootQC:    rootQC,
	}

	return &inf, nil
}

func (es *EpochCommitSnapshot) DKG() (protocol.DKG, error) {
	return &DKG{commitEvent: es.commitEvent}, nil
}

func (es *EpochCommitSnapshot) Phase() (protocol.EpochPhase, error) {
	return protocol.EpochCommittedPhase, nil
}

func NewEpochCommitSnapshot(header *flow.Header, setupEvent *flow.EpochSetup, commitEvent *flow.EpochCommit) *EpochCommitSnapshot {
	return &EpochCommitSnapshot{
		EpochSetupSnapshot: EpochSetupSnapshot{
			header:     header,
			setupEvent: setupEvent,
		},
		commitEvent: commitEvent,
	}
}

// ****************************************

type UndefinedEpochSnapshot struct {
	err error
}

func (u *UndefinedEpochSnapshot) Counter() (uint64, error) {
	return 0, u.err
}

func (u *UndefinedEpochSnapshot) Head() (*flow.Header, error) {
	return nil, u.err
}

func (u *UndefinedEpochSnapshot) FinalView() (uint64, error) {
	return 0, u.err
}

func (u *UndefinedEpochSnapshot) InitialIdentities(flow.IdentityFilter) (flow.IdentityList, error) {
	return nil, u.err
}

func (u *UndefinedEpochSnapshot) Clustering() (flow.ClusterList, error) {
	return nil, u.err
}

func (u *UndefinedEpochSnapshot) ClusterInformation(uint32) (protocol.ClusterInformation, error) {
	return nil, u.err
}

func (u *UndefinedEpochSnapshot) DKG() (protocol.DKG, error) {
	return nil, u.err
}

func (u *UndefinedEpochSnapshot) EpochSetupSeed(...uint32) ([]byte, error) {
	return nil, u.err
}

func (u *UndefinedEpochSnapshot) Phase() (protocol.EpochPhase, error) {
	return -1, u.err
}

func NewUndefinedEpochSnapshot(err error) *UndefinedEpochSnapshot {
	return &UndefinedEpochSnapshot{err: err}
}
