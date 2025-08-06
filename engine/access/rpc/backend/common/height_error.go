package common

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// ResolveHeightError processes errors returned during height-based queries.
// If the error is due to a block not being found, this function determines whether the queried
// height falls outside the node's accessible range and provides context-sensitive error messages
// based on spork and node root block heights.
//
// Expected errors during normal operation:
// - storage.ErrNotFound - Indicates that the queried block does not exist in the local database.
func ResolveHeightError(
	stateParams protocol.Params,
	height uint64,
	genericErr error,
) error {
	if !errors.Is(genericErr, storage.ErrNotFound) {
		return genericErr
	}

	sporkRootBlockHeight := stateParams.SporkRootBlockHeight()
	nodeRootBlockHeader := stateParams.SealedRoot().Height

	if height < sporkRootBlockHeight {
		return fmt.Errorf("block height %d is less than the spork root block height %d. Try to use a historic node: %w",
			height,
			sporkRootBlockHeight,
			genericErr,
		)
	} else if height < nodeRootBlockHeader {
		return fmt.Errorf("block height %d is less than the node's root block height %d. Try to use a different Access node: %w",
			height,
			nodeRootBlockHeader,
			genericErr,
		)
	} else {
		return genericErr
	}
}
