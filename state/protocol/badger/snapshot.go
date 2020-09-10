// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// protocolStateSnapshot represents a read-only immutable snapshot of the protocol state at the
// block it is constructed with. It allows efficient access to data associated directly
// with blocks at a given state (finalized, sealed), such as the related header, commit,
// seed or pending children. A block snapshot can lazily convert to an epoch snapshot in
// order to make data associated directly with epochs accessible through its API.
type protocolStateSnapshot struct {
	err     error
	state   *State
	blockID flow.Identifier
}

// Identities will convert the block snapshot into an epoch snapshot to retrieve the list of
// identities for the epoch active at the current block snapshot.
func (ps *protocolStateSnapshot) Identities(selector flow.IdentityFilter) (flow.IdentityList, error) {
	return bs.EpochSnapshot().Identities(selector)
}

// Identity will convert the block snapshot to an epoch snapshot to retrieve the identity with
// the given node ID for the epoch active at the current block snapshot.
func (ps *protocolStateSnapshot) Identity(nodeID flow.Identifier) (*flow.Identity, error) {
	return bs.EpochSnapshot().Identity(nodeID)
}

// Commit retrieves the latest execution state commitment at the current block snapshot. This
// commitment represents the execution state as currently finalized.
func (ps *protocolStateSnapshot) Commit() (flow.StateCommitment, error) {
	if bs.err != nil {
		return nil, bs.err
	}

	// get the ID of the sealed block
	seal, err := bs.state.seals.ByBlockID(bs.blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get look up sealed commit: %w", err)
	}

	return seal.FinalState, nil
}

// Clusters will convert the block snapshot to an epoch snapshot to retrieve the list of
// clusters for the epoch active at the current block snapshot.
func (ps *protocolStateSnapshot) Clusters() (flow.ClusterList, error) {
	return bs.EpochSnapshot().Clusters()
}

func (ps *protocolStateSnapshot) ClusterRootBlock(cluster flow.IdentityList) (*cluster.Block, error) {
	return bs.EpochSnapshot().ClusterRootBlock(cluster)
}

func (ps *protocolStateSnapshot) ClusterRootQC(cluster flow.IdentityList) (*flow.QuorumCertificate, error) {
	return bs.EpochSnapshot().ClusterRootQC(cluster)
}

// Head returns the header associated with the current block snapshot.
func (ps *protocolStateSnapshot) Head() (*flow.Header, error) {
	if bs.err != nil {
		return nil, bs.err
	}

	return bs.state.headers.ByBlockID(bs.blockID)
}

// Seed returns the random seed at the given indices for the current block snapshot.
func (ps *protocolStateSnapshot) Seed(indices ...uint32) ([]byte, error) {
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

// Pending returns a list of block IDs for blocks that are pending finalization at the
// given block snapshot.
func (ps *protocolStateSnapshot) Pending() ([]flow.Identifier, error) {
	if bs.err != nil {
		return nil, bs.err
	}
	return bs.pending(bs.blockID)
}

func (ps *protocolStateSnapshot) pending(blockID flow.Identifier) ([]flow.Identifier, error) {

	var pendingIDs []flow.Identifier
	err := bs.state.db.View(procedure.LookupBlockChildren(blockID, &pendingIDs))
	if err != nil {
		return nil, fmt.Errorf("could not get pending children: %w", err)
	}

	for _, pendingID := range pendingIDs {
		additionalIDs, err := bs.pending(pendingID)
		if err != nil {
			return nil, fmt.Errorf("could not get pending grandchildren: %w", err)
		}
		pendingIDs = append(pendingIDs, additionalIDs...)
	}
	return pendingIDs, nil
}

// Epoch converts the block snapshot into an epoch snapshot in order to return the
// epoch counter associated with the active epoch.
func (ps *protocolStateSnapshot) Epoch() (uint64, error) {
	return bs.EpochSnapshot().Epoch()
}

// DKG converts the block snapshot into an epoch snapshot in order to return the epoch
// DKG data associated with the active epoch.
func (ps *protocolStateSnapshot) DKG() protocol.DKG {
	return bs.EpochSnapshot().DKG()
}

// EpochSnapshot converts the block snapshot into an epoch snapshot. Snapshots can
// be created by providing a block ID or an epoch counter. Depending on the accessed
// data, either one of them can be more efficient. We thus implement the function on
// the type that does it more efficiently and lazily convert between the two as needed.
func (ps *protocolStateSnapshot) EpochSnapshot() *EpochSnapshot {

	// If we already have an error, don't bother converting.
	if bs.err != nil {
		return &EpochSnapshot{err: bs.err}
	}

	// NOTE: We will often access epoch information through block snapshots, so it
	// would make sense to introduce a simple caching layer here that maps view
	// ranges to epochs in order to bypass any database calls and even the storage
	// caching layer. We only need to load this once upon construction and update it
	// as we mutate the protocol state.

	// Retrieve the current header to get its view, as well as the current
	// epoch counter as a starting point.
	header, err := bs.state.headers.ByBlockID(bs.blockID)
	if err != nil {
		return &EpochSnapshot{err: fmt.Errorf("could not retrieve snapshot header: %w", err)}
	}
	var counter uint64
	err = bs.state.db.View(operation.RetrieveEpochCounter(&counter))
	if err != nil {
		return &EpochSnapshot{err: fmt.Errorf("could not retrieve epoch counter: %w", err)}
	}

	// If the header's view is after the current epoch's view, we are dealing
	// with a header for the next epoch (it could be pending). We should never
	// have pending headers from two epochs in the future, so it's safe to
	// return here.
	var setup flow.EpochSetup
	err = bs.state.db.View(operation.RetrieveEpochSetup(counter, &setup))
	if err != nil {
		return &EpochSnapshot{err: fmt.Errorf("could not retrieve epoch setup: %w", err)}
	}
	if header.View > setup.FinalView {
		return &EpochSnapshot{
			state:   bs.state,
			counter: counter + 1,
		}
	}

	// we can now iterate backwards through the epochs until we find the one the
	// header's view falls within
	var start uint64 // first view of the epoch, inclusive
	for {

		// get the start view of the epoch
		err = bs.state.db.View(operation.LookupEpochStart(counter, &start))
		if err != nil {
			return &EpochSnapshot{err: fmt.Errorf("could not look up epoch start (counter: %d): %w", counter, err)}
		}

		if header.View >= start {
			return &EpochSnapshot{
				state:   bs.state,
				counter: counter,
			}
		}

		counter--
	}
}
