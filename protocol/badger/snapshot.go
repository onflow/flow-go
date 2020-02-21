// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"
	"math"
	"sort"

	"github.com/dgraph-io/badger/v2"

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
	sealed bool
}

// Identities retrieves all active ids at the given snapshot and
// applies the given filters.
func (s *Snapshot) Identities(filters ...flow.IdentityFilter) (flow.IdentityList, error) {

	// execute the transaction that retrieves everything
	var identities []*flow.Identity
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
		if head.View < boundary {
			boundary = head.View
		}

		// get finalized stakes within the boundary
		deltas, err := computeFinalizedDeltas(tx, boundary, filters)
		if err != nil {
			return fmt.Errorf("could not compute finalized stakes: %w", err)
		}

		// if there are unfinalized blocks, retrieve stakes there
		if head.View > boundary {

			// get the final block we want to reach
			var finalID flow.Identifier
			err = operation.RetrieveNumber(boundary, &finalID)(tx)
			if err != nil {
				return fmt.Errorf("could not get final hash: %w", err)
			}

			// track back from head block to latest finalized block
			for head.ID() != finalID {

				// get the stake deltas
				// TODO: separate this from identities
				var identities []*flow.Identity
				err = procedure.RetrieveIdentities(head.PayloadHash, &identities)(tx)
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

				// set the head to the parent
				err = operation.RetrieveHeader(head.ParentID, &head)(tx)
				if err != nil {
					return fmt.Errorf("could not retrieve parent: %w", err)
				}
			}

		}

		// get identity for each non-zero stake
		for nodeID, delta := range deltas {

			// discard nodes where the remaining stake is zero
			if delta == 0 {
				continue
			}

			// retrieve the identity
			var identity flow.Identity
			err = operation.RetrieveIdentity(nodeID, &identity)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve role (%x): %w", nodeID, err)
			}

			// set the stake for the node
			identity.Stake = uint64(delta)

			identities = append(identities, &identity)
		}

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

func (s *Snapshot) Commit() (flow.StateCommitment, error) {

	var commit flow.StateCommitment
	err := s.state.db.View(func(tx *badger.Txn) error {

		// get the head at the requested snapshot
		var header flow.Header
		err := s.head(&header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve head: %w", err)
		}

		// try getting the commit directly from the block
		var commit flow.StateCommitment
		err = operation.LookupCommit(header.ID(), &commit)(tx)
		if err != nil {
			return fmt.Errorf("could not get commit: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not retrieve commit: %w", err)
	}

	return commit, nil
}

// Clusters sorts the list of node identities after filtering into the given
// number of clusters.
//
// This is guaranteed to be deterministic for an identical set of identities,
// regardless of the order.
func (s *Snapshot) Clusters() (*flow.ClusterList, error) {

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
	clusters := flow.NewClusterList(s.state.clusters)
	for i, identity := range identities {
		index := uint(i) % s.state.clusters
		clusters.Add(index, identity)
	}

	return clusters, nil
}

func (s *Snapshot) head(head *flow.Header) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		if s.sealed {
			// set the number to boundary if it's at max uint64
			if s.number == math.MaxUint64 {
				err := operation.RetrieveSealedBoundary(&s.number)(tx)
				if err != nil {
					return fmt.Errorf("could not retrieve sealed boundary: %w", err)
				}
			}

			// check if hash is nil and try to get it from height
			if s.blockID == flow.ZeroID {
				err := operation.RetrieveSealHeight(s.number, &s.blockID)(tx)
				if err != nil {
					return fmt.Errorf("could not retrieve seal height (%d): %w", s.number, err)
				}
			}
		} else {
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
					return fmt.Errorf("could not retrieve number (%d): %w", s.number, err)
				}
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
