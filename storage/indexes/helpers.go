package indexes

import (
	"fmt"
	"math"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// validateStoreHeight validates that the block height is consecutive to the latest indexed height.
// Returns an error if the block height is not exactly one greater than the latest indexed height.
//
// Expected error returns during normal operation:
//   - [storage.ErrAlreadyExists]: if blockHeight is already indexed
func validateStoreHeight(blockHeight, latestHeight uint64) error {
	expectedHeight := latestHeight + 1
	if blockHeight < expectedHeight {
		return storage.ErrAlreadyExists
	}
	if blockHeight > expectedHeight {
		return fmt.Errorf("must index consecutive heights: expected %d, got %d", expectedHeight, blockHeight)
	}
	return nil
}

// validateCursorHeight validates the block height for the cursor is within the valid range (firstHeight, latestHeight)
//
// Expected error returns during normal operations:
//   - [storage.ErrHeightNotIndexed] if the block height is outside of the indexed range
func validateCursorHeight(blockHeight uint64, firstHeight uint64, latestHeight uint64) error {
	if blockHeight > latestHeight {
		return fmt.Errorf("cursor height %d is greater than latest indexed height %d: %w",
			blockHeight, latestHeight, storage.ErrHeightNotIndexed)
	}

	if blockHeight < firstHeight {
		return fmt.Errorf("cursor height %d is before first indexed height %d: %w",
			blockHeight, firstHeight, storage.ErrHeightNotIndexed)
	}
	return nil
}

// validateLimit validates the limit parameter for the index is within the valid exclusive range (0, math.MaxUint32)
//
// Any error indicates the limit is invalid.
func validateLimit(limit uint32) error {
	if limit == 0 {
		return fmt.Errorf("limit must be greater than 0")
	}
	if limit == math.MaxUint32 {
		return fmt.Errorf("limit must be less than %d", math.MaxUint32)
	}
	return nil
}

// readHeight reads a height value from the database.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if the height is not found
func readHeight(reader storage.Reader, key []byte) (uint64, error) {
	var height uint64
	if err := operation.RetrieveByKey(reader, key, &height); err != nil {
		return 0, err
	}
	return height, nil
}
