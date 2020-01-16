// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"
	"math"
	"sort"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
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
	var ids flow.IdentityList
	err := s.state.db.View(func(tx *badger.Txn) error {

		// check if height is max uint64 to get latest finalized state
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
		if head.Number < boundary {
			boundary = head.Number
		}

		// get finalized stakes within the boundary
		deltas, err := computeFinalizedDeltas(tx, boundary, filters)
		if err != nil {
			return fmt.Errorf("could not compute finalized stakes: %w", err)
		}

		// if there are unfinalized blocks, retrieve stakes there
		if head.Number > boundary {

			// get the final block we want to reach
			var finalID flow.Identifier
			err = operation.RetrieveBlockID(boundary, &finalID)(tx)
			if err != nil {
				return fmt.Errorf("could not get final hash: %w", err)
			}

			// track back from head block to latest finalized block
			for head.ID() != finalID {

				// get the identities for pending block
				var ids flow.IdentityList
				err = operation.RetrieveIdentities(head.ID(), &ids)(tx)
				if err != nil {
					return fmt.Errorf("could not add deltas: %w", err)
				}

				// manually add the deltas for valid ids
				for _, id := range ids {
					for _, filter := range filters {
						if !filter(id) {
							continue
						}
					}
					deltas[id.NodeID] += int64(id.Stake)
				}

				// set the head to the parent
				err = operation.RetrieveHeader(head.ParentID, &head)(tx)
				if err != nil {
					return fmt.Errorf("could not retrieve parent: %w", err)
				}
			}

		}

		// get role & address for each non-zero stake
		for nodeID, delta := range deltas {

			// discard nodes where the remaining stake is zero
			if delta == 0 {
				continue
			}

			// create the identity
			id := flow.Identity{
				NodeID: nodeID,
				Stake:  uint64(delta),
			}

			// retrieve the identity role
			err = operation.RetrieveRole(id.NodeID, &id.Role)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve role (%x): %w", nodeID, err)
			}

			// retrieve the identity address
			err = operation.RetrieveAddress(id.NodeID, &id.Address)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve address (%x): %w", nodeID, err)
			}

			ids = append(ids, id)
		}

		sort.Slice(ids, func(i int, j int) bool {
			return identity.ByNodeIDAsc(ids[i], ids[j])
		})

		return nil
	})

	return ids, err
}

func (s *Snapshot) Identity(nodeID flow.Identifier) (flow.Identity, error) {

	// get the ids
	ids, err := s.Identities(identity.HasNodeID(nodeID))
	if err != nil {
		return flow.Identity{}, fmt.Errorf("could not get identities: %w", err)
	}

	// return error if he doesn't exist
	if len(ids) == 0 {
		return flow.Identity{}, fmt.Errorf("identity not staked (%x)", nodeID)
	}

	return ids[0], nil
}

// Clusters sorts the list of node identities after filtering into the given
// number of clusters.
//
// This is guaranteed to be deterministic for an identical set of identities,
// regardless of the order.
func (s *Snapshot) Clusters() (*flow.ClusterList, error) {

	// get the node identities
	identities, err := s.Identities(identity.HasRole(flow.RoleCollection))
	if err != nil {
		return nil, fmt.Errorf("could not get identities: %w", err)
	}

	// order the identities by node ID
	sort.Slice(identities, func(i, j int) bool {
		return identity.ByNodeIDAsc(identities[i], identities[j])
	})

	// create the desired number of clusters and assign nodes
	clusters := flow.NewClusterList(s.state.clusters)
	for i, identity := range identities {
		index := uint(i) % s.state.clusters
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
			err := operation.RetrieveBlockID(s.number, &s.blockID)(tx)
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

func computeFinalizedDeltas(tx *badger.Txn, boundary uint64, filters []flow.IdentityFilter) (map[flow.Identifier]int64, error) {

	// define start and end prefixes for the range scan
	deltas := make(map[flow.Identifier]int64)
	err := operation.TraverseDeltas(0, boundary, filters, func(number uint64, role flow.Role, nodeID flow.Identifier, delta int64) error {
		deltas[nodeID] += delta
		return nil
	})(tx)

	return deltas, err
}
