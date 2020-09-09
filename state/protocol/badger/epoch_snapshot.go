// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/state/protocol"
)

// EpochSnapshot represents a read-only immutable snapshot of the protocol state at the
// epoch it is constructed with. It allows efficient access to data associated directly
// with epochs, such as identities, clusters and DKG information. An epoch snapshot can
// lazily convert to a block snapshot in order to make data associated directly with blocks
// accessible through its API.
type EpochSnapshot struct {
	state   *State
	counter uint64
	blockID flow.Identifier
}

func (es *EpochSnapshot) Counter() (uint64, error) {
	return es.counter, nil
}

func (es *EpochSnapshot) Head() (*flow.Header, error) {
	return es.state.headers.ByBlockID(es.blockID)
}

func (es *EpochSnapshot) FinalView() (uint64, error) {
	setup, err := es.state.setups.ByCounter(es.counter)
	if err != nil {
		return 0, fmt.Errorf("could not retrieve epoch setup event: %w", err)
	}
	return setup.FinalView, nil
}

func (es *EpochSnapshot) InitialIdentities(selector flow.IdentityFilter) (flow.IdentityList, error) {
	setup, err := es.state.setups.ByCounter(es.counter)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch setup event: %w", err)
	}
	return setup.Participants, nil
}

func (es *EpochSnapshot) Clustering() (flow.ClusterList, error) {
	setup, err := es.state.setups.ByCounter(es.counter)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch setup event: %w", err)
	}

	collectorFilter := filter.And(filter.HasStake(true), filter.HasRole(flow.RoleCollection))
	clustering, err := flow.NewClusterList(setup.Assignments, setup.Participants.Filter(collectorFilter))
	if err != nil {
		return nil, fmt.Errorf("failed to generate ClusterList from collector identities: %w", err)
	}
	return clustering, nil
}

func (es *EpochSnapshot) ClusterInformation(index uint32) (protocol.ClusterInformation, error) {
	panic("implement me")
}

func (es *EpochSnapshot) DKG() (protocol.DKG, error) {
	panic("implement me")
}

func (es *EpochSnapshot) EpochSetupSeed(indices ...uint32) ([]byte, error) {
	panic("implement me")
}

func (es *EpochSnapshot) Phase() (protocol.EpochState, error) {
	panic("implement me")
}

func NewEpochSnapshot() protocol.EpochSnapshot

// ****************************************

type UndefinedEpochSnapshot struct {
	err error
}

func (es *UndefinedEpochSnapshot) Head() (*flow.Header, error) {
	return nil, es.err
}

func (es *UndefinedEpochSnapshot) FinalView() (uint64, error) {
	return 0, es.err
}

func (es *UndefinedEpochSnapshot) Phase() (protocol.EpochState, error) {
	return -1, es.err
}

func (es *UndefinedEpochSnapshot) InitialIdentities(flow.IdentityFilter) (flow.IdentityList, error) {
	return nil, es.err
}
func (es *UndefinedEpochSnapshot) Counter() (uint64, error) {
	return 0, es.err
}
func (es *UndefinedEpochSnapshot) Clustering() (flow.ClusterList, error) {
	return nil, es.err
}
func (es *UndefinedEpochSnapshot) ClusterInformation(uint32) (protocol.ClusterInformation, error) {
	return nil, es.err
}
func (es *UndefinedEpochSnapshot) DKG() (protocol.DKG, error) {
	return nil, es.err
}
func (es *UndefinedEpochSnapshot) EpochSetupSeed(...uint32) ([]byte, error) {
	return nil, es.err
}
