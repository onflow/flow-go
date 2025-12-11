package common

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// ResolveHeightError wraps storage.ErrNotFound errors returned during height-based queries with
// additional context about why the data was not found.
//
// Specifically, this function determines whether the queried height falls outside the node's
// accessible range and provides context-sensitive error messages based on spork and node root block
// heights.
//
// Will return the original error, possibly wrapped with additional context.
// CAUTION: this function might return irrecoverable errors or generic fatal from the lower protocol layers
func ResolveHeightError(
	stateParams protocol.Params,
	height uint64,
	err error,
) error {
	if !errors.Is(err, storage.ErrNotFound) {
		return err
	}

	sporkRootBlockHeight := stateParams.SporkRootBlockHeight()
	nodeRootBlockHeader := stateParams.SealedRoot().Height

	if height < sporkRootBlockHeight {
		return fmt.Errorf("block height %d is less than the spork root block height %d. Try to use a historic node: %w",
			height,
			sporkRootBlockHeight,
			err,
		)
	}

	if height < nodeRootBlockHeader {
		return fmt.Errorf("block height %d is less than the node's root block height %d. Try to use a different Access node: %w",
			height,
			nodeRootBlockHeader,
			err,
		)
	}

	return err
}
