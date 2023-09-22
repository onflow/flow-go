package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// RegisterIndex defines methods for the register index.
type RegisterIndex interface {
	// Get the register by the ID at a given block height or lower.
	//
	// If the register at the given height was not indexed, return the value from the highest height the register was indexed at if one exists.
	// Expected errors:
	// - storage.ErrHeightNotIndexed if the given height was not indexed yet or lower than the first indexed height.
	// - storage.ErrNotFound if the given height is indexed, but the register does not exist.
	Get(ID flow.RegisterID, height uint64) (flow.RegisterValue, error)

	// LatestHeight at which we indexed the registers. 
	// The value will continue to change as we index new registers.
	// Expected errors:
	// - storage.ErrNotFound if the latest height is not set.
	LatestHeight() (uint64, error)

	// FirstHeight at which we indexed the registers. 
	// The value should not be changed after the register index is created.
	// Expected errors:
	// - storage.ErrNotFound if the first height is not set.
	FirstHeight() (uint64, error)

	// Store batch of register entries at the provided block height.
	//
	// The provided height must either be the same as the latest indexed height or one higher,
	// otherwise and error is returned. If the height is not within those bounds there is either a bug
	// or state corruption. 
	// If the height is the same as the latest indexed height this will be a no-op to ensure idempotency.
	// No errors are expected during normal operation.
	Store(entries flow.RegisterEntries, height uint64) error
}
