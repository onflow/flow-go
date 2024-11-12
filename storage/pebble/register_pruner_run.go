package pebble

import (
	"fmt"
	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
)

// RegisterPrunerRun handles the pruning of outdated register entries from a Pebble database.
// It removes entries older than a specified prune height, ensuring efficient batch deletions.
// This struct also tracks the total number of keys pruned during its run.
type RegisterPrunerRun struct {
	logger      zerolog.Logger
	pruneHeight uint64

	db                   *pebble.DB
	lastRegisterID       flow.RegisterID // The last processed register ID, used for deduplication.
	keepFirstRelevantKey bool            // Flag to indicate if the first relevant key is kept.

	totalKeysPruned int // Counter for the total number of pruned keys.
}

// NewRegisterPrunerRun initializes a new RegisterPrunerRun instance.
//
// Parameters:
//   - db: The Pebble database instance to prune registers from.
//   - logger: Logger instance for logging pruning events and errors.
//   - pruneHeight: The height below which entries should be pruned.
//
// Returns:
//   - A pointer to the newly created RegisterPrunerRun instance, ready to execute pruning operations.
//
// Example usage:
//
//	pruner := NewRegisterPrunerRun(db, logger, 10000)
func NewRegisterPrunerRun(db *pebble.DB, logger zerolog.Logger, pruneHeight uint64) *RegisterPrunerRun {
	return &RegisterPrunerRun{
		logger:          logger,
		pruneHeight:     pruneHeight,
		db:              db,
		totalKeysPruned: 0,
	}
}

// BatchDelete removes the specified keys from the database in a single batch operation.
// This method optimizes deletion by grouping multiple delete operations into a single
// batch, which improves performance and reduces the load on the database.
//
// Parameters:
//   - ctx: The context for managing the operation, including cancellation.
//   - lookupKeys: A slice of keys to delete from the database.
//
// No errors are expected during normal operations.
func (p *RegisterPrunerRun) BatchDelete(lookupKeys [][]byte) error {
	dbBatch := p.db.NewBatch()
	defer func() {
		if cerr := dbBatch.Close(); cerr != nil {
			p.logger.Err(cerr).Msg("error while closing the db batch")
		}
	}()

	for _, key := range lookupKeys {
		if err := dbBatch.Delete(key, nil); err != nil {
			keyHeight, registerID, _ := lookupKeyToRegisterID(key)
			return fmt.Errorf("failed to delete lookupKey (height: %d, registerID: %v): %w", keyHeight, registerID, err)
		}
	}

	if err := dbBatch.Commit(pebble.Sync); err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	p.totalKeysPruned += len(lookupKeys)

	return nil
}

// CanPruneKey evaluates if a given key can be pruned based on its height and
// register ID. This function ensures that only the earliest relevant key for
// each register ID is retained, which helps to preserve the latest state.
//
// Parameters:
//   - key: The key to evaluate for pruning eligibility.
//
// No errors are expected during normal operations.
func (p *RegisterPrunerRun) CanPruneKey(key []byte) (bool, error) {
	keyHeight, registerID, err := lookupKeyToRegisterID(key)
	if err != nil {
		return false, fmt.Errorf("malformed lookup key %v: %w", key, err)
	}

	// Reset the state if this is a new register ID.
	if p.lastRegisterID != registerID {
		p.keepFirstRelevantKey = false
		p.lastRegisterID = registerID
	}

	// If the height of the key is above the prune height, it should not be pruned.
	if keyHeight > p.pruneHeight {
		return false, nil
	}

	// For each register ID, find the first key whose height is less than or equal to the prune height.
	// This is the earliest entry to keep. For example, if pruneHeight is 99989:
	// [0x01/key/owner1/99990] [keep, > 99989]
	// [0x01/key/owner1/99988] [first key to keep < 99989]
	// [0x01/key/owner1/85000] [remove]
	// ...
	// [0x01/key/owner2/99989] [first key to keep == 99989]
	// [0x01/key/owner2/99988] [remove]
	// ...
	// [0x01/key/owner3/99988] [first key to keep < 99989]
	// [0x01/key/owner3/98001] [remove]
	// ...
	// [0x02/key/owner0/99900] [first key to keep < 99989]
	if !p.keepFirstRelevantKey {
		p.keepFirstRelevantKey = true
		return false, nil
	}

	return true, nil
}

// TotalKeysPruned returns the total number of keys that have been pruned during
// this run of the RegisterPrunerRun instance.
//
// Returns:
//   - An integer representing the total count of keys that were pruned.
//
// Example usage:
//
//	totalPruned := pruner.TotalKeysPruned()
func (p *RegisterPrunerRun) TotalKeysPruned() int {
	return p.totalKeysPruned
}
