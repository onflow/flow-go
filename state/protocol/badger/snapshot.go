// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/flow/order"
	"github.com/dapperlabs/flow-go/module/signature"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Snapshot represents a read-only immutable snapshot of the protocol state.
type Snapshot struct {
	err     error
	state   *State
	blockID flow.Identifier
}

// Identities retrieves all active ids at the given snapshot and
// applies the given filters.
func (s *Snapshot) Identities(selector flow.IdentityFilter) (flow.IdentityList, error) {

	// retrieve identities from storage
	identities, err := s.state.identities.ByBlockID(s.blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get identities for block: %w", err)
	}

	// apply the filter to the identities
	identities = identities.Filter(selector)

	// apply a deterministic sort to the identities
	sort.Slice(identities, func(i int, j int) bool {
		return order.ByNodeIDAsc(identities[i], identities[j])
	})

	return identities, err
}

func (s *Snapshot) Identity(nodeID flow.Identifier) (*flow.Identity, error) {

	// filter identities at snapshot for node ID
	identities, err := s.Identities(filter.HasNodeID(nodeID))
	if err != nil {
		return nil, fmt.Errorf("could not get identities: %w", err)
	}

	// check if node ID is part of identities
	if len(identities) == 0 {
		return nil, fmt.Errorf("identity not staked (%x)", nodeID)
	}

	return identities[0], nil
}

func (s *Snapshot) Commit() (flow.StateCommitment, error) {
	return s.state.commits.ByBlockID(s.blockID)
}

// Clusters sorts the list of node identities after filtering into the given
// number of clusters.
//
// This is guaranteed to be deterministic for an identical set of identities,
// regardless of the order.
func (s *Snapshot) Clusters() (*flow.ClusterList, error) {

	nClusters := s.state.clusters

	// get the node identities
	identities, err := s.Identities(filter.HasRole(flow.RoleCollection))
	if err != nil {
		return nil, fmt.Errorf("could not get identities: %w", err)
	}

	return protocol.Clusters(nClusters, identities), nil
}

func (s *Snapshot) Head() (*flow.Header, error) {
	return s.state.headers.ByBlockID(s.blockID)
}

func (s *Snapshot) Seed(indices ...uint32) ([]byte, error) {

	if len(indices)*4 > hash.KmacMaxParamsLen {
		return nil, fmt.Errorf("unsupported number of indices")
	}

	// get the current state snapshot head
	head, err := s.state.headers.ByBlockID(s.blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get head: %w", err)
	}

	// TODO: apparently, we need to get a child of the block here in order to
	// use the parent signatures contained within it; this is non-trivial, so
	// I will defer that to a different task

	// create the key used for the KMAC by concatenating all indices
	key := make([]byte, 4*len(indices))
	for i, index := range indices {
		binary.LittleEndian.PutUint32(key[4*i:4*i+4], index)
	}

	// create a KMAC instance with our key and 32 bytes output size
	kmac, err := hash.NewKMAC_128(key, nil, 32)
	if err != nil {
		return nil, fmt.Errorf("could not create kmac: %w", err)
	}

	// split the parent voter sig into staking & beacon parts
	combiner := signature.NewCombiner()
	sigs, err := combiner.Split(head.ParentVoterSig)
	if err != nil {
		return nil, fmt.Errorf("could not split block signature: %w", err)
	}
	if len(sigs) != 2 {
		return nil, fmt.Errorf("invalid block signature split")
	}

	// generate the seed by hashing the random beacon threshold signature
	beaconSig := sigs[1]
	seed := kmac.ComputeHash(beaconSig)

	return seed, nil
}

func (s *Snapshot) Pending() ([]flow.Identifier, error) {
	var pendingIDs []flow.Identifier
	err := s.state.db.View(procedure.LookupBlockChildren(s.blockID, &pendingIDs))
	return pendingIDs, err
}
