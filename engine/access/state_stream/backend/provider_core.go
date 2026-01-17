package backend

import (
	"errors"
	"fmt"
	"time"

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// providerCore handles common provider operations
type providerCore struct {
	state           protocol.State
	snapshotBuilder *executionStateSnapshotBuilder
	criteria        optimistic_sync.Criteria
	blockHeight     uint64
}

// getSnapshotMetadata retrieves the execution state snapshot metadata for the current block height.
// It builds the snapshot using the snapshot builder and translates errors appropriately.
//
// Expected errors during normal operations:
//   - [subscription.ErrBlockNotReady]: If the block header is not found or execution result is not ready yet.
//   - [optimistic_sync.ErrBlockBeforeNodeHistory]: If the request is for data before the node's root block.
//   - [optimistic_sync.SnapshotNotFoundError]: Result is not available, not ready for querying, or does not descend from the latest sealed result.
//   - [optimistic_sync.CriteriaNotMetError]: Returned when the block is already sealed but no execution result can satisfy the provided criteria.
func (c *providerCore) getSnapshotMetadata() (*executionStateSnapshotMetadata, error) {
	metadata, err := c.snapshotBuilder.BuildSnapshot(c.blockHeight, c.criteria)
	if err != nil {
		switch {
		case optimistic_sync.IsExecutionResultNotReadyError(err):
			return nil, subscription.ErrBlockNotReady

		case errors.Is(err, storage.ErrNotFound):
			return nil, subscription.ErrBlockNotReady

		case errors.Is(err, optimistic_sync.SnapshotNotFoundError) ||
			errors.Is(err, optimistic_sync.ErrBlockBeforeNodeHistory) ||
			optimistic_sync.IsCriteriaNotMetError(err):
			return nil, err

		default:
			return nil, fmt.Errorf("unexpected error: %w", err)
		}
	}
	return metadata, nil
}

func (c *providerCore) isSporkRoot() bool {
	return c.blockHeight == c.state.Params().SporkRootBlock().Height
}

func (c *providerCore) sporkRootBlockInfo() (flow.Identifier, time.Time) {
	sporkRoot := c.state.Params().SporkRootBlock()
	return sporkRoot.ID(), time.UnixMilli(int64(sporkRoot.Timestamp)).UTC()
}

func (c *providerCore) incrementHeight(resultID flow.Identifier) {
	c.criteria.ParentExecutionResultID = resultID
	c.blockHeight++
}
