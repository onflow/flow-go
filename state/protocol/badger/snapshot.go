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
	"github.com/dapperlabs/flow-go/storage/badger/operation"
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
	if s.err != nil {
		return nil, s.err
	}

	// NOTE: we always use the genesis identities for now
	var genesisID flow.Identifier
	err := s.state.db.View(operation.LookupBlockHeight(0, &genesisID))
	if err != nil {
		return nil, fmt.Errorf("could not look up genesis block: %w", err)
	}

	// retrieve identities from storage
	identities, err := s.state.payloads.IdentitiesFor(genesisID)
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
	if s.err != nil {
		return nil, s.err
	}

	// filter identities at snapshot for node ID
	identities, err := s.Identities(filter.HasNodeID(nodeID))
	if err != nil {
		return nil, fmt.Errorf("could not get identities: %w", err)
	}

	// check if node ID is part of identities
	if len(identities) == 0 {
		return nil, fmt.Errorf("identity not found (%x)", nodeID)
	}

	return identities[0], nil
}

func (s *Snapshot) Commit() (flow.StateCommitment, error) {
	if s.err != nil {
		return nil, s.err
	}

	// get the ID of the sealed block
	seal, err := s.state.seals.ByBlockID(s.blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get look up sealed commit: %w", err)
	}

	return seal.FinalState, nil
}

// Clusters sorts the list of node identities after filtering into the given
// number of clusters.
//
// This is guaranteed to be deterministic for an identical set of identities,
// regardless of the order.
func (s *Snapshot) Clusters() (*flow.ClusterList, error) {
	if s.err != nil {
		return nil, s.err
	}

	// get the node identities
	identities, err := s.Identities(filter.HasRole(flow.RoleCollection))
	if err != nil {
		return nil, fmt.Errorf("could not get identities: %w", err)
	}

	return protocol.Clusters(s.state.clusters, identities), nil
}

func (s *Snapshot) Head() (*flow.Header, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.state.headers.ByBlockID(s.blockID)
}

func (s *Snapshot) Seed(indices ...uint32) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}

	if len(indices)*4 > hash.KmacMaxParamsLen {
		return nil, fmt.Errorf("unsupported number of indices")
	}

	// get the current state snapshot head
	var childrenIDs []flow.Identifier
	err := s.state.db.View(procedure.LookupBlockChildren(s.blockID, &childrenIDs))
	if err != nil {
		return nil, fmt.Errorf("could not look up children: %w", err)
	}

	// check we have at least one child
	if len(childrenIDs) == 0 {
		return nil, fmt.Errorf("block doesn't have children yet")
	}

	// get the header of the first child (they all have the same threshold sig)
	head, err := s.state.headers.ByBlockID(childrenIDs[0])
	if err != nil {
		return nil, fmt.Errorf("could not get head: %w", err)
	}

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
	if s.err != nil {
		return nil, s.err
	}
	return s.pending(s.blockID)
}

func (s *Snapshot) pending(blockID flow.Identifier) ([]flow.Identifier, error) {

	var pendingIDs []flow.Identifier
	err := s.state.db.View(procedure.LookupBlockChildren(blockID, &pendingIDs))
	if err != nil {
		return nil, fmt.Errorf("could not get pending children: %w", err)
	}

	for _, pendingID := range pendingIDs {
		additionalIDs, err := s.pending(pendingID)
		if err != nil {
			return nil, fmt.Errorf("could not get pending grandchildren: %w", err)
		}
		pendingIDs = append(pendingIDs, additionalIDs...)
	}
	return pendingIDs, nil
}
