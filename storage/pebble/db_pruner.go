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

type DBPruner struct {
	logger zerolog.Logger
	// pruneThrottleDelay controls a small pause between batches of registers inspected and pruned
	pruneThrottleDelay time.Duration
	pruneHeight        uint64

	dbBatch        *pebble.Batch
	lastRegisterID flow.RegisterID
	keepKeyFound   bool
}

var _ io.Closer = (*DBPruner)(nil)

func NewDBPruner(db *pebble.DB, logger zerolog.Logger, pruneThrottleDelay time.Duration, pruneHeight uint64) *DBPruner {
	return &DBPruner{
		logger:             logger,
		pruneThrottleDelay: pruneThrottleDelay,
		pruneHeight:        pruneHeight,
		dbBatch:            db.NewBatch(),
	}
}

// BatchDelete deletes the provided keys from the database in a single batch operation.
// It creates a new batch, deletes each key from the batch, and commits the batch to ensure
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

	// Throttle to prevent excessive system load
	select {
	case <-ctx.Done():
	case <-time.After(p.pruneThrottleDelay):
	}

	return nil
}

func (p *DBPruner) CanPruneKey(key []byte) (bool, error) {
	keyHeight, registerID, err := lookupKeyToRegisterID(key)
	if err != nil {
		return false, fmt.Errorf("malformed lookup key %v: %w", key, err)
	}

	// New register prefix, reset the state
	if !p.keepKeyFound || p.lastRegisterID != registerID {
		p.keepKeyFound = false
		p.lastRegisterID = registerID
	}

	if keyHeight > p.pruneHeight {
		return false, nil
	}

	if !p.keepKeyFound {
		// Keep the first entry found for this registerID that is <= pruneHeight
		p.keepKeyFound = true
		return false, nil
	}

	return true, nil
}

func (p *DBPruner) Close() error {
	return p.dbBatch.Close()
}
