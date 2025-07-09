package backend

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendNetwork struct {
	state                protocol.State
	chainID              flow.ChainID
	headers              storage.Headers
	snapshotHistoryLimit int
}

// GetNetworkParameters returns the network parameters for the current network.
func (b *backendNetwork) GetNetworkParameters(_ context.Context) accessmodel.NetworkParameters {
	return accessmodel.NetworkParameters{
		ChainID: b.chainID,
	}
}

// GetLatestProtocolStateSnapshot returns the latest finalized snapshot.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
func (b *backendNetwork) GetLatestProtocolStateSnapshot(ctx context.Context) ([]byte, error) {
	snapshot := b.state.Final()
	data, err := convert.SnapshotToBytes(snapshot)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, protocol.ErrSealingSegmentBelowRootBlock, protocol.NewUnfinalizedSealingSegmentErrorf(""))
		return nil, fmt.Errorf("snapshots might not be possible for every block: %w", err)
	}

	return data, nil
}

// GetProtocolStateSnapshotByBlockID returns serializable Snapshot for a block, by blockID.
// The requested block must be finalized, otherwise an error is returned.
//
// Dedicated sentinel errors providing details to clients about failed requests:
//   - access.DataNotFoundError - No block with the given ID was found
//   - access.InvalidRequestError - Block ID is for an orphaned block and will never have a valid snapshot
//   - access.PreconditionFailedError - A block was found, but it is not finalized and is above the finalized height.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
func (b *backendNetwork) GetProtocolStateSnapshotByBlockID(ctx context.Context, blockID flow.Identifier) ([]byte, error) {
	snapshot := b.state.AtBlockID(blockID)
	snapshotHeadByBlockId, err := snapshot.Head()
	if err != nil {
		// storage.ErrNotFound is specifically NOT allowed since the snapshot's reference block must exist
		// within the snapshot. we can ignore the actual error since it is rewritten below
		_ = access.RequireErrorIs(ctx, err, state.ErrUnknownSnapshotReference)
		return nil, access.NewDataNotFoundError("snapshot", fmt.Errorf("failed to retrieve snapshot: block not found"))
	}

	// Because there is no index from block ID to finalized height, we separately look up the finalized
	// block ID by the height of the queried block, then compare the queried ID to the finalized ID.
	// If they match, then the queried block must be finalized.
	blockIDFinalizedAtHeight, err := b.headers.BlockIDByHeight(snapshotHeadByBlockId.Height)
	if err != nil {
		// assert that the error is storage.ErrNotFound. we can ignore the actual error since it is rewritten below
		_ = access.RequireErrorIs(ctx, err, storage.ErrNotFound)

		// The block exists, but no block has been finalized at its height. Therefore, this block
		// may be finalized in the future, and the client can retry.
		return nil, access.NewPreconditionFailedError(
			fmt.Errorf("failed to retrieve snapshot: block still pending finalization",
				snapshotHeadByBlockId.Height))
	}

	if blockIDFinalizedAtHeight != blockID {
		// A different block than what was queried has been finalized at this height.
		// Therefore, the queried block will never be finalized.
		return nil, access.NewInvalidRequestError(fmt.Errorf("failed to retrieve snapshot: block orphaned"))
	}

	data, err := convert.SnapshotToBytes(snapshot)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, protocol.ErrSealingSegmentBelowRootBlock, protocol.NewUnfinalizedSealingSegmentErrorf(""))
		return nil, fmt.Errorf("snapshots might not be possible for every block: %w", err)
	}
	return data, nil
}

// GetProtocolStateSnapshotByHeight returns serializable Snapshot by block height.
// The block must be finalized (otherwise the by-height query is ambiguous).
//
// Dedicated sentinel errors providing details to clients about failed requests:
//   - access.DataNotFoundError - No finalized block with the given height was found.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
func (b *backendNetwork) GetProtocolStateSnapshotByHeight(ctx context.Context, blockHeight uint64) ([]byte, error) {
	snapshot := b.state.AtHeight(blockHeight)
	_, err := snapshot.Head()
	if err != nil {
		// storage.ErrNotFound is specifically NOT allowed since the snapshot's reference block must exist
		// within the snapshot. we can ignore the actual error since it is rewritten below
		_ = access.RequireErrorIs(ctx, err, state.ErrUnknownSnapshotReference)
		return nil, access.NewDataNotFoundError("snapshot", fmt.Errorf("failed to retrieve a valid snapshot: block not found"))
	}

	data, err := convert.SnapshotToBytes(snapshot)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, protocol.ErrSealingSegmentBelowRootBlock, protocol.NewUnfinalizedSealingSegmentErrorf(""))
		return nil, fmt.Errorf("snapshots might not be possible for every block: %w", err)
	}
	return data, nil
}
