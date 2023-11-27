package protocol

import (
	"errors"
	"fmt"
	"math"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/storage"
)

// IsNodeAuthorizedAt returns whether the node with the given ID is a valid
// un-ejected network participant as of the given state snapshot.
func IsNodeAuthorizedAt(snapshot Snapshot, id flow.Identifier) (bool, error) {
	return CheckNodeStatusAt(
		snapshot,
		id,
		filter.HasInitialWeight[flow.Identity](true),
		filter.IsValidCurrentEpochParticipant,
	)
}

// IsNodeAuthorizedWithRoleAt returns whether the node with the given ID is a valid
// un-ejected network participant with the specified role as of the given state snapshot.
// Expected errors during normal operations:
//   - state.ErrUnknownSnapshotReference if snapshot references an unknown block
//
// All other errors are unexpected and potential symptoms of internal state corruption.
func IsNodeAuthorizedWithRoleAt(snapshot Snapshot, id flow.Identifier, role flow.Role) (bool, error) {
	return CheckNodeStatusAt(
		snapshot,
		id,
		filter.HasInitialWeight[flow.Identity](true),
		filter.IsValidCurrentEpochParticipant,
		filter.HasRole[flow.Identity](role),
	)
}

// CheckNodeStatusAt returns whether the node with the given ID is a valid identity at the given
// state snapshot, and satisfies all checks.
// Expected errors during normal operations:
//   - state.ErrUnknownSnapshotReference if snapshot references an unknown block
//
// All other errors are unexpected and potential symptoms of internal state corruption.
func CheckNodeStatusAt(snapshot Snapshot, id flow.Identifier, checks ...flow.IdentityFilter[flow.Identity]) (bool, error) {
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
	sporkRootBlockHeight := snapshot.Params().SporkRootBlockHeight()
	head, err := snapshot.Head()
	if err != nil {
		return false, fmt.Errorf("could not get snapshot head: %w", err)
	}
	return head.Height == sporkRootBlockHeight, nil
}

// PreviousEpochExists returns whether the previous epoch exists w.r.t. the given
// state snapshot.
// No errors are expected during normal operation.
func PreviousEpochExists(snap Snapshot) (bool, error) {
	_, err := snap.Epochs().Previous().Counter()
	if errors.Is(err, ErrNoPreviousEpoch) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("unexpected error checking previous epoch exists: %w", err)
	}
	return true, nil
}

// FindGuarantors decodes the signer indices from the guarantee, and finds the guarantor identifiers from protocol state
// Expected Error returns during normal operations:
//   - signature.InvalidSignerIndicesError if `signerIndices` does not encode a valid set of collection guarantors
//   - state.ErrUnknownSnapshotReference if guarantee references an unknown block
//   - protocol.ErrNextEpochNotCommitted if epoch has not been committed yet
//   - protocol.ErrClusterNotFound if cluster is not found by the given chainID
func FindGuarantors(state State, guarantee *flow.CollectionGuarantee) ([]flow.Identifier, error) {
	snapshot := state.AtBlockID(guarantee.ReferenceBlockID)
	epochs := snapshot.Epochs()
	epoch := epochs.Current()
	cluster, err := epoch.ClusterByChainID(guarantee.ChainID)

	if err != nil {
		return nil, fmt.Errorf(
			"fail to retrieve collector clusters for guarantee (ReferenceBlockID: %v, ChainID: %v): %w",
			guarantee.ReferenceBlockID, guarantee.ChainID, err)
	}

	guarantorIDs, err := signature.DecodeSignerIndicesToIdentifiers(cluster.Members().NodeIDs(), guarantee.SignerIndices)
	if err != nil {
		return nil, fmt.Errorf("could not decode signer indices for guarantee %v: %w", guarantee.ID(), err)
	}

	return guarantorIDs, nil
}

// OrderedSeals returns the seals in the input payload in ascending height order.
// The Flow protocol has a variety of validity rules for `payload`. While we do not verify
// payload validity in this function, the implementation is optimized for valid payloads,
// where the heights of the sealed blocks form a continuous integer sequence (no gaps).
// Per convention ['Vacuous Truth'], an empty set of seals is considered to be
// ordered. Hence, if `payload.Seals` is empty, we return (nil, nil).
// Expected Error returns during normal operations:
//   - ErrMultipleSealsForSameHeight in case there are seals repeatedly sealing block at the same height
//   - ErrDiscontinuousSeals in case there are height-gaps in the sealed blocks
//   - storage.ErrNotFound if any of the seals references an unknown block
func OrderedSeals(blockSeals []*flow.Seal, headers storage.Headers) ([]*flow.Seal, error) {
	numSeals := uint64(len(blockSeals))
	if numSeals == 0 {
		return nil, nil
	}
	heights := make([]uint64, numSeals)
	minHeight := uint64(math.MaxUint64)
	for i, seal := range blockSeals {
		header, err := headers.ByBlockID(seal.BlockID)
		if err != nil {
			return nil, fmt.Errorf("could not get block (id=%x) for seal: %w", seal.BlockID, err) // storage.ErrNotFound or exception
		}
		heights[i] = header.Height
		if header.Height < minHeight {
			minHeight = header.Height
		}
	}
	// As seals in a valid payload must have consecutive heights, we can populate
	// the ordered output by shifting by minHeight.
	seals := make([]*flow.Seal, numSeals)
	for i, seal := range blockSeals {
		idx := heights[i] - minHeight
		// (0) Per construction, `minHeight` is the smallest value in the `heights` slice. Hence, `idx ≥ 0`
		// (1) But if there are gaps in the heights of the sealed blocks (byzantine inputs),
		//    `idx` may reach/exceed `numSeals`. In this case, we respond with a `ErrDiscontinuousSeals`.
		//     Thereby this function can tolerate all inputs without producing an 'out of range' panic.
		// (2) In case of duplicates _and_ gaps, we might overwrite elements in `seals` while leaving
		//     other elements as `nil`. We avoid the edge case of leaving `nil` elements in our output,
		//     and instead, reject the input with an `ErrMultipleSealsForSameHeight`
		if idx >= numSeals {
			return nil, fmt.Errorf("sealed blocks' heights (unordered) %v : %w", heights, ErrDiscontinuousSeals)
		}
		if seals[idx] != nil {
			return nil, fmt.Errorf("duplicate seal for block height %d: %w", heights[i], ErrMultipleSealsForSameHeight)
		}
		seals[idx] = seal
	}
	return seals, nil
}
