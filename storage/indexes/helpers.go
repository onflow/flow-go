package indexes

import (
	"fmt"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

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
