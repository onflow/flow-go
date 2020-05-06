// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/dgraph-io/badger/v2"

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
	state   *State
	number  uint64
	blockID flow.Identifier
}

// Identities retrieves all active ids at the given snapshot and
// applies the given filters.
func (s *Snapshot) Identities(selector flow.IdentityFilter) (flow.IdentityList, error) {

	// retrieve identities from the database
	var identities flow.IdentityList
	err := s.state.db.View(operation.RetrieveIdentities(&identities))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve identities: %w", err)
	}

	// filter the identities
	identities = identities.Filter(selector)

	// identities should always be deterministically storted
	sort.Slice(identities, func(i int, j int) bool {
		return order.ByNodeIDAsc(identities[i], identities[j])
	})

	return identities, err
}

func (s *Snapshot) Identity(nodeID flow.Identifier) (*flow.Identity, error) {

	// get the ids
	identities, err := s.Identities(filter.HasNodeID(nodeID))
	if err != nil {
		return nil, fmt.Errorf("could not get identities: %w", err)
	}

	// return error if he doesn't exist
	if len(identities) == 0 {
		return nil, fmt.Errorf("identity not staked (%x)", nodeID)
	}

	return identities[0], nil
}

func (s *Snapshot) Seal() (*flow.Seal, error) {

	var seal flow.Seal
	err := s.state.db.View(func(tx *badger.Txn) error {

		// get the head at the requested snapshot
		var header flow.Header
		err := s.head(&header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve head: %w", err)
		}

		err = procedure.LookupSealByBlock(header.ID(), &seal)(tx)
		if err != nil {
			return fmt.Errorf("could not get seal by block: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not retrieve seal: %w", err)
	}

	return &seal, nil
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

func (s *Snapshot) head(head *flow.Header) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		err := procedure.RetrieveHeader(&s.number, &s.blockID, head)(tx)
		if err != nil {
			return fmt.Errorf("could not get snapshot header: %w", err)
		}

		// retrieve the finalized header to ensure we are only dealing with
		// blocks from the main consensus-node chain within the protocol state
		var final flow.Header
		err = procedure.RetrieveLatestFinalizedHeader(&final)(tx)
		if err != nil {
			return fmt.Errorf("could not get chain ID: %w", err)
		}

		if head.ChainID != final.ChainID {
			return fmt.Errorf("invalid chain id (got=%s expected=%s)", head.ChainID, final.ChainID)
		}

		return err
	}
}

func (s *Snapshot) Head() (*flow.Header, error) {
	var header flow.Header
	err := s.state.db.View(func(tx *badger.Txn) error {
		return s.head(&header)(tx)
	})
	return &header, err
}

func (s *Snapshot) Seed(indices ...uint32) ([]byte, error) {

	if len(indices)*4 > hash.KmacMaxParamsLen {
		return nil, fmt.Errorf("unsupported number of indices")
	}

	// get the current state snapshot head
	var header flow.Header
	err := s.state.db.View(func(tx *badger.Txn) error {
		return s.head(&header)(tx)
	})
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
	sigs, err := combiner.Split(header.ParentVoterSig)
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

func (s *Snapshot) Unfinalized() ([]flow.Identifier, error) {
	var unfinalizedBlockIDs []flow.Identifier
	err := s.state.db.View(func(tx *badger.Txn) error {
		var boundary uint64
		// retrieve the current finalized view
		err := operation.RetrieveBoundary(&boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		// retrieve the block ID of the last finalized block
		var headID flow.Identifier
		err = operation.RetrieveNumber(boundary, &headID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve head: %w", err)
		}

		// find all the unfinalized blocks that connect to the finalized block
		// the order guarantees that if a block requires certain blocks to connect to the
		// finalized block, those connecting blocks must appear before this block.
		operation.FindDescendants(boundary, headID, &unfinalizedBlockIDs)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("could not find unfinalized block IDs: %w", err)
	}

	return unfinalizedBlockIDs, nil
}
