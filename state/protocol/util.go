package protocol

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/signature"
)

// IsNodeAuthorizedAt returns whether the node with the given ID is a valid
// un-ejected network participant as of the given state snapshot.
func IsNodeAuthorizedAt(snapshot Snapshot, id flow.Identifier) (bool, error) {
	return CheckNodeStatusAt(
		snapshot,
		id,
		filter.HasWeight(true),
		filter.Not(filter.Ejected),
	)
}

// IsNodeAuthorizedWithRoleAt returns whether the node with the given ID is a valid
// un-ejected network participant with the specified role as of the given state snapshot.
// Expected errors during normal operations:
//  * storage.ErrNotFound if snapshot references an unknown block
// All other errors are unexpected and potential symptoms of internal state corruption.
func IsNodeAuthorizedWithRoleAt(snapshot Snapshot, id flow.Identifier, role flow.Role) (bool, error) {
	return CheckNodeStatusAt(
		snapshot,
		id,
		filter.HasWeight(true),
		filter.Not(filter.Ejected),
		filter.HasRole(role),
	)
}

// CheckNodeStatusAt returns whether the node with the given ID is a valid identity at the given
// state snapshot, and satisfies all checks.
// Expected errors during normal operations:
//  * storage.ErrNotFound if snapshot references an unknown block
// All other errors are unexpected and potential symptoms of internal state corruption.
func CheckNodeStatusAt(snapshot Snapshot, id flow.Identifier, checks ...flow.IdentityFilter) (bool, error) {
	identity, err := snapshot.Identity(id)
	if IsIdentityNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("could not retrieve node identity (id=%x): %w)", id, err)
	}

	for _, check := range checks {
		if !check(identity) {
			return false, nil
		}
	}

	return true, nil
}

// IsSporkRootSnapshot returns whether the given snapshot is the state snapshot
// representing the initial state for a spork.
func IsSporkRootSnapshot(snapshot Snapshot) (bool, error) {
	segment, err := snapshot.SealingSegment()
	if err != nil {
		return false, fmt.Errorf("could not get snapshot head: %w", err)
	}
	if len(segment.Blocks) > 1 {
		// spork root snapshots uniquely have only one block in the sealing segment
		return false, nil
	}
	return true, nil
}

// FindGuarantors decodes the signer indices from the guarantee, and finds the guarantor identifiers from protocol state
func FindGuarantors(state State, guarantee *flow.CollectionGuarantee) ([]flow.Identifier, error) {
	snapshot := state.AtBlockID(guarantee.ReferenceBlockID)
	epochs := snapshot.Epochs()
	epoch := epochs.Current()
	cluster, err := epoch.ClusterByChainID(guarantee.ChainID)

	if err != nil {
		// protocol state must have validated the block that contains the guarantee, so the cluster
		// must be found, otherwise, it's an internal error
		return nil, fmt.Errorf(
			"internal error retrieving collector clusters for guarantee (ReferenceBlockID: %v, ChainID: %v): %w",
			guarantee.ReferenceBlockID, guarantee.ChainID, err)
	}

	guarantorIDs, err := signature.DecodeSignerIndicesToIdentifiers(cluster.Members().NodeIDs(), guarantee.SignerIndices)
	if err != nil {
		return nil, fmt.Errorf("could not decode guarantor indices: %w", err)
	}

	return guarantorIDs, nil
}
