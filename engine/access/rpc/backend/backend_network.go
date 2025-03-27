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
// No errors are expected during normal operation.
// All errors can be considered benign. Exceptions are handled explicitly within the backend and are
// not propagated.
func (b *backendNetwork) GetLatestProtocolStateSnapshot(ctx context.Context) ([]byte, error) {
	snapshot := b.state.Final()
	data, err := convert.SnapshotToBytes(snapshot)
	if err != nil {
		return nil, access.RequireNoError(ctx, fmt.Errorf("failed to convert snapshot to bytes: %w", err))
	}

	return data, nil
}

// GetProtocolStateSnapshotByBlockID returns serializable Snapshot for a block, by blockID.
// The requested block must be finalized, otherwise an error is returned.
//
// Expected errors during normal operation:
//   - access.DataNotFoundError - No block with the given ID was found
//   - access.InvalidRequestError - Block ID is for an orphaned block and will never have a valid snapshot
//   - access.PreconditionFailedError - A block was found, but it is not finalized and is above the finalized height.
//
// All errors can be considered benign. Exceptions are handled explicitly within the backend and are
// not propagated.
//
// The block may or may not be finalized in the future; the client can retry later.
func (b *backendNetwork) GetProtocolStateSnapshotByBlockID(ctx context.Context, blockID flow.Identifier) ([]byte, error) {
	snapshot := b.state.AtBlockID(blockID)
	snapshotHeadByBlockId, err := snapshot.Head()
	if err != nil {
		// storage.ErrNotFound is specifically NOT allowed since the snapshot's reference block must exist
		// within the snapshot. we can ignore the actual error since it is rewritten below
		_ = access.RequireErrorIs(ctx, err, state.ErrUnknownSnapshotReference)
		return nil, access.NewDataNotFoundError("snapshot", fmt.Errorf("failed to retrieve a valid snapshot: block not found"))
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
			fmt.Errorf("failed to retrieve snapshot for block with height %d: block not finalized and is above finalized height",
				snapshotHeadByBlockId.Height))
	}

	if blockIDFinalizedAtHeight != blockID {
		// A different block than what was queried has been finalized at this height.
		// Therefore, the queried block will never be finalized.
		return nil, access.NewInvalidRequestError(fmt.Errorf("failed to retrieve snapshot for block: block not finalized and is below finalized height"))
	}

	data, err := convert.SnapshotToBytes(snapshot)
	if err != nil {
		return nil, access.RequireNoError(ctx, fmt.Errorf("failed to convert snapshot to bytes: %w", err))
	}
	return data, nil
}

// GetProtocolStateSnapshotByHeight returns serializable Snapshot by block height.
// The block must be finalized (otherwise the by-height query is ambiguous).
//
// Expected errors during normal operation:
//   - access.DataNotFoundError - No finalized block with the given height was found.
//
// All errors can be considered benign. Exceptions are handled explicitly within the backend and are
// not propagated.
//
// The block height may or may not be finalized in the future; the client can retry later.
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
		return nil, access.RequireNoError(ctx, fmt.Errorf("failed to convert snapshot to bytes: %w", err))
	}
	return data, nil
}
