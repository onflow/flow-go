// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/flow/order"
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
func (s *Snapshot) Identities(filters ...flow.IdentityFilter) (flow.IdentityList, error) {

	// execute the transaction that retrieves everything
	var identities flow.IdentityList
	err := s.state.db.View(func(tx *badger.Txn) error {

		// get the top header at the requested snapshot
		var head flow.Header
		err := s.head(&head)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve head: %w", err)
		}

		// get the latest finalized height
		var boundary uint64
		err = operation.RetrieveBoundary(&boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		// if the target number is before finalized state, set it as new limit
		if head.Height < boundary {
			boundary = head.Height
		}

		// get finalized stakes within the boundary
		deltas, err := computeFinalizedDeltas(tx, boundary, filters)
		if err != nil {
			return fmt.Errorf("could not compute finalized stakes: %w", err)
		}

		// if there are unfinalized blocks, retrieve stakes there
		if head.Height > boundary {

			// get the final block we want to reach
			var finalID flow.Identifier
			err = operation.RetrieveNumber(boundary, &finalID)(tx)
			if err != nil {
				return fmt.Errorf("could not get final hash: %w", err)
			}

			// track back from parent block to latest finalized block
			parentID := head.ParentID
			for parentID != finalID {

				// get the stake deltas
				// TODO: separate this from identities
				var identities []*flow.Identity
				err = procedure.RetrieveIdentities(parentID, &identities)(tx)
				if err != nil {
					return fmt.Errorf("could not add deltas: %w", err)
				}

				// manually add the deltas for valid ids
				for _, identity := range identities {
					for _, filter := range filters {
						if !filter(identity) {
							continue
						}
					}
					deltas[identity.NodeID] += int64(identity.Stake)
				}

				// get the next parent and forward pointer
				var parent flow.Header
				err = operation.RetrieveHeader(parentID, &parent)(tx)
				if err != nil {
					return fmt.Errorf("could not retrieve parent (%s): %w", parentID, err)
				}
				parentID = parent.ParentID
			}

		}

		// get identity for each non-zero stake
		for nodeID, delta := range deltas {

			// retrieve the identity
			var identity flow.Identity
			err = operation.RetrieveIdentity(nodeID, &identity)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve identity (%x): %w", nodeID, err)
			}

			// set the stake for the node
			identity.Stake = uint64(delta)

			identities = append(identities, &identity)
		}

		// apply filters again to filter on stuff that wasn't available while
		// running through the delta index in the key-value store
		identities = identities.Filter(filters...)

		sort.Slice(identities, func(i int, j int) bool {
			return order.ByNodeIDAsc(identities[i], identities[j])
		})

		return nil
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

func (s *Snapshot) Seal() (flow.Seal, error) {

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
		return flow.Seal{}, fmt.Errorf("could not retrieve seal: %w", err)
	}

	return seal, nil
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

	// order the identities by node ID
	sort.Slice(identities, func(i, j int) bool {
		return order.ByNodeIDAsc(identities[i], identities[j])
	})

	// create the desired number of clusters and assign nodes
	clusters := flow.NewClusterList(nClusters)
	for i, identity := range identities {
		index := uint(i) % nClusters
		clusters.Add(index, identity)
	}

	return clusters, nil
}

func (s *Snapshot) head(head *flow.Header) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// set the number to boundary if it's at max uint64
		if s.number == math.MaxUint64 {
			err := operation.RetrieveBoundary(&s.number)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve boundary: %w", err)
			}
		}

		// check if hash is nil and try to get it from height
		if s.blockID == flow.ZeroID {
			err := operation.RetrieveNumber(s.number, &s.blockID)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve hash (%d): %w", s.number, err)
			}
		}

		// get the height for our desired target hash
		err := operation.RetrieveHeader(s.blockID, head)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header (%x): %w", s.blockID, err)
		}

		return nil
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

	if len(indices)*4 > crypto.KmacMaxParamsLen {
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

	// create the key used for the KMAC by concatenating all indices
	key := make([]byte, 4*len(indices))
	for i, index := range indices {
		binary.LittleEndian.PutUint32(key[4*i:4*i+4], index)
	}

	// create a KMAC instance with our key and 32 bytes output size
	kmac, err := crypto.NewKMAC_128(key, nil, 32)
	if err != nil {
		return nil, fmt.Errorf("could not create kmac: %w", err)
	}

	seed := kmac.ComputeHash(header.ParentRandomBeaconSig)

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

func computeFinalizedDeltas(tx *badger.Txn, boundary uint64, filters []flow.IdentityFilter) (map[flow.Identifier]int64, error) {

	// define start and end prefixes for the range scan
	deltas := make(map[flow.Identifier]int64)
	err := operation.TraverseDeltas(0, boundary, filters, func(nodeID flow.Identifier, delta int64) error {
		deltas[nodeID] += delta
		return nil
	})(tx)

	return deltas, err
}
