package pebble

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
)

// DBPruner is responsible for pruning outdated register entries in a Pebble database.
// It batches deletions to improve performance and reduce the load on the system.
type DBPruner struct {
	logger zerolog.Logger
	// pruneThrottleDelay controls a small pause between batches of registers inspected and pruned
	pruneThrottleDelay time.Duration
	pruneHeight        uint64

	dbBatch              *pebble.Batch
	lastRegisterID       flow.RegisterID
	keepFirstRelevantKey bool

	totalKeysPruned int
}

var _ io.Closer = (*DBPruner)(nil)

func NewDBPruner(db *pebble.DB, logger zerolog.Logger, pruneThrottleDelay time.Duration, pruneHeight uint64) *DBPruner {
	return &DBPruner{
		logger:             logger,
		pruneThrottleDelay: pruneThrottleDelay,
		pruneHeight:        pruneHeight,
		dbBatch:            db.NewBatch(),
		totalKeysPruned:    0,
	}
}

// BatchDelete deletes the provided keys from the database in a single batch operation.
// It resets the batch for reuse, deletes each key from the batch, and commits the batch to ensure
// that the deletions are applied atomically.
//
// Parameters:
//   - ctx: The context for managing the pruning throttle delay operation.
//   - lookupKeys: A slice of keys to be deleted from the database.
//
// No errors are expected during normal operations.
func (p *DBPruner) BatchDelete(ctx context.Context, lookupKeys [][]byte) error {
	defer p.dbBatch.Reset()

	for _, key := range lookupKeys {
		if err := p.dbBatch.Delete(key, nil); err != nil {
			keyHeight, registerID, _ := lookupKeyToRegisterID(key)
			return fmt.Errorf("failed to delete lookupKey: %w %d %v", err, keyHeight, registerID)
		}
	}

	if err := p.dbBatch.Commit(pebble.Sync); err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	p.totalKeysPruned += len(lookupKeys)

	// Throttle to prevent excessive system load
	select {
	case <-ctx.Done():
	case <-time.After(p.pruneThrottleDelay):
	}

	return nil
}

// CanPruneKey checks if a key can be pruned based on its height and the last processed register ID.
// It ensures that only the first relevant key (the earliest entry) for each register ID is kept.
//
// Parameters:
//   - key: The key to check for pruning eligibility.
//
// No errors are expected during normal operations.
func (p *DBPruner) CanPruneKey(key []byte) (bool, error) {
	keyHeight, registerID, err := lookupKeyToRegisterID(key)
	if err != nil {
		return false, fmt.Errorf("malformed lookup key %v: %w", key, err)
	}

	// If height is greater than prune height, the key cannot be pruned.
	if keyHeight > p.pruneHeight {
		return false, nil
	}

	// In case of a new register ID, reset the state.
	if p.lastRegisterID != registerID {
		p.keepFirstRelevantKey = false
		p.lastRegisterID = registerID
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

// TotalKeysPruned returns the total number of keys that have been pruned by the DBPruner.
func (p *DBPruner) TotalKeysPruned() int {
	return p.totalKeysPruned
}

// Close closes the batch associated with the DBPruner.
func (p *DBPruner) Close() error {
	return p.dbBatch.Close()
}
