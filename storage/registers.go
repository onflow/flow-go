package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// RegisterIndexReader defines readonly methods for the register index.
type RegisterIndexReader interface {
	// Get register by the register ID at a given block height.
	//
	// If the register at the given height was not indexed, returns the highest
	// height the register was indexed at.
	// Expected errors:
	// - storage.ErrHeightNotIndexed if the given height was not indexed yet or lower than the first indexed height.
	// - storage.ErrNotFound if the given height is indexed, but the register does not exist.
	Get(ID flow.RegisterID, height uint64) (flow.RegisterValue, error)

	// LatestHeight returns the latest indexed height.
	LatestHeight() uint64

	// FirstHeight at which we started to index. Returns the first indexed height found in the store.
	FirstHeight() uint64

	// ByKeyPrefix returns an iterator over all registers whose key starts with keyPrefix,
	// at or before height, across all owners. It uses a single pebble iterator that seeks
	// monotonically forward through the address space.
	//
	// When keyPrefix is an exact key (e.g. "contract_names"), the iterator yields one entry
	// per owner that has that key. When keyPrefix is a partial key (e.g. "code."), it yields
	// one entry per unique matching (owner, key) pair. Using the "code." example, it yields
	// the most recent contract code for each contract in each account as of the given height.
	//
	// If cursor is provided, the iterator will start from the next register key after the cursor.
	// Use this to resume iteration from a previous position. This is useful when performing long running
	// iterations, so you can close and reopen the iterator to avoid pausing compaction for too long.
	//
	// No error returns are expected during normal operation.
	ByKeyPrefix(keyPrefix string, height uint64, cursor *flow.RegisterID) IndexIterator[flow.RegisterValue, flow.RegisterID]
}

// RegisterIndex defines methods for the register index.
type RegisterIndex interface {
	RegisterIndexReader

	// Store batch of register entries at the provided block height.
	//
	// The provided height must either be one higher than the current height or the same to ensure idempotency,
	// otherwise and error is returned. If the height is not within those bounds there is either a bug
	// or state corruption.
	//
	// No errors are expected during normal operation.
	Store(entries flow.RegisterEntries, height uint64) error
}
