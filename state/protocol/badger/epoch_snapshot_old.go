// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"
	"sort"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/flow/order"
	"github.com/dapperlabs/flow-go/state"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// EpochSnapshotOld represents a read-only immutable snapshot of the protocol state at the
// epoch it is constructed with. It allows efficient access to data associated directly
// with epochs, such as identities, clusters and DKG information. An epoch snapshot can
// lazily convert to a block snapshot in order to make data associated directly with blocks
// accessible through its API.
type EpochSnapshotOld struct {
	err     error
	state   *State
	counter uint64
}

// Identities returns the set of identities for the epoch associated with the current epoch
// snapshot, filtered with the given selector. It uses a deterministic sorting olgorithm based on
// the node IDs, which means that order is deterministic between calls as long as the same selector
// is used.
func (es *EpochSnapshotOld) Identities(selector flow.IdentityFilter) (flow.IdentityList, error) {
	if es.err != nil {
		return nil, es.err
	}

	// retrieve the identities for the epoch
	var setup flow.EpochSetup
	err := es.state.db.View(operation.RetrieveEpochSetup(es.counter, &setup))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch setup event: %w", err)
	}

	// TODO: We currently don't slash any nodes. However, once we receive
	// slashing events, we need a smart way to progressively store a growing
	// list of stake modifications per epoch, which should be applied here.

	// apply the filter to the identities
	identities := setup.Participants.Filter(selector)

	// apply a deterministic sort to the participants
	sort.Slice(identities, func(i int, j int) bool {
		return order.ByNodeIDAsc(identities[i], identities[j])
	})

	return identities, nil
}

// Identity retrieves the identity with the given node ID from the identities for the epoch
// associated with the current epoch snapshot.
func (es *EpochSnapshotOld) Identity(nodeID flow.Identifier) (*flow.Identity, error) {
	if es.err != nil {
		return nil, es.err
	}

	// filter identities at snapshot for node ID
	identities, err := es.Identities(filter.HasNodeID(nodeID))
	if err != nil {
		return nil, fmt.Errorf("could not get identities: %w", err)
	}

	// check if node ID is part of identities
	if len(identities) == 0 {
		return nil, protocol.IdentityNotFoundErr{
			NodeID: nodeID,
		}
	}

	return identities[0], nil
}

// Commit will convert the epoch snapshot into a block snapshot for the latest
// finalized block on the given epoch in order to retrieve the commit associated with
// that block.
func (es *EpochSnapshotOld) Commit() (flow.StateCommitment, error) {
	return es.BlockSnapshot().Commit()
}

// Clusters retrieves the cluster assignments for the epoch associated with the current
// snapshot and retrieves a list of clusters with the respective collection nodes assigned
// to the respective clusters.
func (es *EpochSnapshotOld) Clusters() (flow.ClusterList, error) {
	if es.err != nil {
		return nil, es.err
	}

	var setup flow.EpochSetup
	err := es.state.db.View(operation.RetrieveEpochSetup(es.counter, &setup))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch setup: %w", err)
	}

	// create the list of clusters
	clusters, err := flow.NewClusterList(
		setup.Assignments,
		setup.Participants.Filter(filter.HasRole(flow.RoleCollection)),
	)
	return clusters, err
}

// ClusterRootBlock returns the canonical root block for the given cluster, for the
// epoch associated with the current snapshot.
func (es *EpochSnapshotOld) ClusterRootBlock(cluster flow.IdentityList) (*cluster.Block, error) {

	counter, err := es.Epoch()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch: %w", err)
	}
	clusters, err := es.Clusters()
	if err != nil {
		return nil, fmt.Errorf("could not get clusters: %w", err)
	}

	// verify the cluster exists
	_, exists := clusters.IndexOf(cluster)
	if !exists {
		return nil, fmt.Errorf("cluster does not exist in current epoch")
	}

	return protocol.CanonicalClusterRootBlock(counter, cluster), nil
}

// ClusterRootQC returns the quorum certificate for the root block of the given
// cluster, for the epoch associated with the current snapshot.
func (es *EpochSnapshotOld) ClusterRootQC(cluster flow.IdentityList) (*flow.QuorumCertificate, error) {

	clusters, err := es.Clusters()
	if err != nil {
		return nil, fmt.Errorf("could not get clusters: %w", err)
	}

	index, exists := clusters.IndexOf(cluster)
	if !exists {
		return nil, fmt.Errorf("cluster does not exist in current epoch")
	}

	var commit flow.EpochCommit
	err = es.state.db.View(operation.RetrieveEpochCommit(es.counter, &commit))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch commit: %w", err)
	}

	return commit.ClusterQCs[index], nil
}

// Head converts the epoch snapshot into a block snapshot in order to retrieve the header
// associated with the latest finalized block of the epoch.
func (es *EpochSnapshotOld) Head() (*flow.Header, error) {
	return es.BlockSnapshot().Head()
}

// Seed converts the epoch snapshot into a block snapshot in order to retrieve the
// random seed associated with the latest finalized block of the epoch.
func (es *EpochSnapshotOld) Seed(indices ...uint32) ([]byte, error) {
	return es.BlockSnapshot().Seed(indices...)
}

// Pending converts the epoch snapshot into a block snapshot in order to retrieve
// the blocks pending finalization associated with the latest finalized block of the epoch.
func (es *EpochSnapshotOld) Pending() ([]flow.Identifier, error) {
	return es.BlockSnapshot().Pending()
}

// Epoch returns the counter of the epoch associated with the snapshot.
func (es *EpochSnapshotOld) Epoch() (uint64, error) {
	if es.err != nil {
		return 0, es.err
	}

	return es.counter, nil
}

// DKG returns a DKG accessor to access DKG information for the epoch associated with the
// snapshot in a granular manner.
func (es *EpochSnapshotOld) DKG() protocol.DKG {
	return &DKG{snapshot: es}
}


// Identities returns the set of identities for the epoch associated with the current epoch
// snapshot, filtered with the given selector. It uses a deterministic sorting olgorithm based on
// the node IDs, which means that order is deterministic between calls as long as the same selector
// is used.
func (es *EpochSnapshotOld) EpochSeed(indices ...uint32)  ([]byte, error) {
	if es.err != nil {
		return nil, es.err
	}

	// retrieve the identities for the epoch
	var setup flow.EpochSetup
	err := es.state.db.View(operation.RetrieveEpochSetup(es.counter, &setup))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch identities: %w", err)
	}

	setup.

	return identities, nil
}

// Seed returns the random seed at the given indices for the current block snapshot.
func (bs *EpochSnapshotOld) Seed2(indices ...uint32) ([]byte, error) {
	if bs.err != nil {
		return nil, bs.err
	}

	// get the current state snapshot head
	var childrenIDs []flow.Identifier
	err := bs.state.db.View(procedure.LookupBlockChildren(bs.blockID, &childrenIDs))
	if err != nil {
		return nil, fmt.Errorf("could not look up children: %w", err)
	}

	// check we have at least one child
	if len(childrenIDs) == 0 {
		return nil, state.NewNoValidChildBlockError("block doesn't have children yet")
	}

	// find the first child that has been validated
	var validChildID flow.Identifier
	for _, childID := range childrenIDs {
		var valid bool
		err = bs.state.db.View(operation.RetrieveBlockValidity(childID, &valid))
		// skip blocks whose validity hasn't been checked yet
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not get child validity: %w", err)
		}
		if valid {
			validChildID = childID
			break
		}
	}

	if validChildID == flow.ZeroID {
		return nil, state.NewNoValidChildBlockError("block has no valid children")
	}

	// get the header of the first child (they all have the same threshold sig)
	head, err := bs.state.headers.ByBlockID(validChildID)
	if err != nil {
		return nil, fmt.Errorf("could not get head: %w", err)
	}

	seed, err := protocol.SeedFromParentSignature(indices, head.ParentVoterSig)
	if err != nil {
		return nil, fmt.Errorf("could not create seed from header's signature: %w", err)
	}

	return seed, nil
}


// BlockSnapshot converts the epoch snapshot into a block snapshot. Snapshots can
// be created by providing a block ID or an epoch counter. Depending on the accessed
// data, either one of them can be more efficient. We thus implement the function on
// the type that does it more efficiently and lazily convert between the two as needed.
// NOTE: Conversion can fail for epochs that don't have any blocks associated with them
// yet. This effectively makes some information inaccessible for epoch snapshots for
// a future epoch.
func (es *EpochSnapshotOld) BlockSnapshot() *BlockSnapshot {

	// If we already have an error, don't bother converting.
	if es.err != nil {
		return &BlockSnapshot{err: es.err}
	}

	// We map epoch to the height of the latest finalized block within it, so this
	// will only fail if no finalized blocks exist yet in the active epoch.
	var height uint64
	err := es.state.db.View(operation.RetrieveEpochHeight(es.counter, &height))
	if err != nil {
		return &BlockSnapshot{err: fmt.Errorf("could not retrieve epoch height: %w", err)}
	}

	// Now, we can simply retrieve the block ID by its height.
	var blockID flow.Identifier
	err = es.state.db.View(operation.LookupBlockHeight(height, &blockID))
	if err != nil {
		return &BlockSnapshot{err: fmt.Errorf("could not look up block height: %w", err)}
	}

	return &BlockSnapshot{
		blockID: blockID,
		state:   es.state,
	}
}
