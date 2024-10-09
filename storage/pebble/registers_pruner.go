package pebble

import (
	"context"
	"fmt"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

const (
	DefaultPruneThreshold      = uint64(100_000)
	DefaultPruneThrottleDelay  = 10 * time.Millisecond
	DefaultPruneTickerInterval = 10 * time.Minute
)

// TODO: This configuration should be changed after testing it with real network data for better performance.
const (
	// pruneIntervalRatio represents an additional percentage of pruneThreshold which is used to calculate pruneInterval
	pruneIntervalRatio = 0.1 // 10%
	// deleteItemsPerBatch defines the number of database keys to delete in each batch operation.
	// This value is used to control the size of deletion operations during pruning.
	deleteItemsPerBatch = 256
)

// pruneInterval is a helper function which calculates interval for pruner
func pruneInterval(threshold uint64) uint64 {
	return threshold + uint64(float64(threshold)*pruneIntervalRatio)
}

type RegisterPruner struct {
	component.Component

	logger zerolog.Logger
	db     *pebble.DB

	metrics module.RegisterDBPrunerMetrics

	// pruningInterval is a number of pruned blocks in the db, above which pruning should be triggered
	pruneInterval uint64
	// threshold defines the number of blocks below the latestHeight to keep
	pruneThreshold uint64
	// pruneThrottleDelay controls a small pause between batches of registers inspected and pruned
	pruneThrottleDelay time.Duration
	// pruneTickerInterval defines how frequently pruning can be performed
	pruneTickerInterval time.Duration
}

type PrunerOption func(*RegisterPruner)

// WithPruneThreshold is used to configure the pruner with a custom threshold.
func WithPruneThreshold(threshold uint64) PrunerOption {
	return func(p *RegisterPruner) {
		p.pruneThreshold = threshold
		p.pruneInterval = pruneInterval(threshold)
	}
}

// WithPruneThrottleDelay is used to configure the pruner with a custom
// throttle delay.
func WithPruneThrottleDelay(throttleDelay time.Duration) PrunerOption {
	return func(p *RegisterPruner) {
		p.pruneThrottleDelay = throttleDelay
	}
}

// WithPruneTickerInterval is used to configure the pruner with a custom
// ticker interval.
func WithPruneTickerInterval(interval time.Duration) PrunerOption {
	return func(p *RegisterPruner) {
		p.pruneTickerInterval = interval
	}
}

// WithPrunerMetrics is used to sets the metrics for a RegisterPruner instance.
func WithPrunerMetrics(metrics module.RegisterDBPrunerMetrics) PrunerOption {
	return func(p *RegisterPruner) {
		p.metrics = metrics
	}
}

func NewRegisterPruner(
	logger zerolog.Logger,
	db *pebble.DB,
	opts ...PrunerOption,
) (*RegisterPruner, error) {
	pruner := &RegisterPruner{
		logger:              logger.With().Str("component", "registerdb_pruner").Logger(),
		db:                  db,
		pruneInterval:       pruneInterval(DefaultPruneThreshold),
		pruneThreshold:      DefaultPruneThreshold,
		pruneThrottleDelay:  DefaultPruneThrottleDelay,
		pruneTickerInterval: DefaultPruneTickerInterval,
	}

	pruner.Component = component.NewComponentManagerBuilder().
		AddWorker(pruner.loop).
		Build()

	for _, opt := range opts {
		opt(pruner)
	}

	return pruner, nil
}

// loop is the main worker for the Pruner, responsible for triggering
// pruning operations at regular intervals. It monitors the heights
// of registered height recorders and checks if pruning is necessary.
func (p *RegisterPruner) loop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	ticker := time.NewTicker(p.pruneTickerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := p.checkPrune(ctx)
			if err != nil {
				ctx.Throw(err)
			}
		}
	}
}

// checkPrune checks if pruning should be performed based on the height range
// and triggers the pruning operation if necessary.
//
// Parameters:
//   - ctx: The context for managing the pruning throttle delay operation.
//
// No errors are expected during normal operations.
func (p *RegisterPruner) checkPrune(ctx context.Context) error {
	firstHeight, err := firstStoredHeight(p.db)
	if err != nil {
		return fmt.Errorf("failed to get first height from register storage: %w", err)
	}

	latestHeight, err := latestStoredHeight(p.db)
	if err != nil {
		return fmt.Errorf("failed to get latest height from register storage: %w", err)
	}

	if latestHeight-firstHeight > p.pruneInterval+p.pruneThreshold {
		pruneHeight := latestHeight - p.pruneThreshold
		p.logger.Info().Uint64("prune_height", pruneHeight).Msg("pruning storage")

		// first, update firstHeight in the db
		// this ensures that if the node crashes during pruning, there will still be a consistent
		// view when the node starts up. Subsequent prunes will remove any leftover data.
		err = p.updateFirstStoredHeight(pruneHeight)
		if err != nil {
			return fmt.Errorf("failed to update first height for register storage: %w", err)
		}

		err := p.pruneUpToHeight(ctx, pruneHeight)
		if err != nil {
			return fmt.Errorf("failed to prune: %w", err)
		}

		p.metrics.NumberOfBlocksPruned(pruneHeight - firstHeight)
	}

	return nil
}

// pruneUpToHeight prunes all entries in the database with heights less than or equal
// to the specified pruneHeight. For each register prefix, it keeps the earliest entry
// that has a height less than or equal to pruneHeight, and deletes all other entries
// with lower heights.
//
// This function iterates over the database keys, identifies keys to delete in batches,
// and uses the batchDelete function to remove them efficiently.
//
// Parameters:
//   - ctx: The context for managing the pruning throttle delay operation.
//   - pruneHeight: The maximum height of entries to prune.
//
// No errors are expected during normal operations.
func (p *RegisterPruner) pruneUpToHeight(ctx context.Context, pruneHeight uint64) error {
	prefix := []byte{codeRegister}
	var batchKeysToRemove [][]byte

	start := time.Now()

	err := func(r pebble.Reader) error {
		options := pebble.IterOptions{
			LowerBound: prefix,
			UpperBound: []byte{codeFirstBlockHeight},
		}

		it, err := r.NewIter(&options)
		if err != nil {
			return fmt.Errorf("can not create iterator: %w", err)
		}
		defer it.Close()

		var lastRegisterID flow.RegisterID
		var keepKeyFound bool

		for it.SeekGE(prefix); it.Valid(); it.Next() {
			key := it.Key()

			keyHeight, registerID, err := lookupKeyToRegisterID(key)
			if err != nil {
				return fmt.Errorf("malformed lookup key %v: %w", key, err)
			}

			if p.metrics != nil {
				p.metrics.ElementVisited()
			}

			// New register prefix, reset the state
			if !keepKeyFound || lastRegisterID != registerID {
				keepKeyFound = false
				lastRegisterID = registerID
			}

			if keyHeight > pruneHeight {
				continue
			}

			if !keepKeyFound {
				// Keep the first entry found for this registerID that is <= pruneHeight
				keepKeyFound = true
				continue
			}

			// Otherwise, mark this key for removal
			// Create a copy of the key to avoid memory issues
			keyCopy := make([]byte, len(key))
			copy(keyCopy, key)
			batchKeysToRemove = append(batchKeysToRemove, keyCopy)

			if len(batchKeysToRemove) == deleteItemsPerBatch {
				// Perform batch delete
				if err := p.batchDelete(ctx, batchKeysToRemove); err != nil {
					return err
				}
				batchKeysToRemove = nil
			}
		}

		if len(batchKeysToRemove) > 0 {
			// Perform the final batch delete if there are any remaining keys
			if err := p.batchDelete(ctx, batchKeysToRemove); err != nil {
				return err
			}
		}

		return nil
	}(p.db)

	if err != nil {
		return err
	}

	if p.metrics != nil {
		p.metrics.Pruned(pruneHeight, time.Since(start))
	}

	return nil
}

// batchDelete deletes the provided keys from the database in a single batch operation.
// It creates a new batch, deletes each key from the batch, and commits the batch to ensure
// that the deletions are applied atomically.
//
// Parameters:
//   - ctx: The context for managing the pruning throttle delay operation.
//   - lookupKeys: A slice of keys to be deleted from the database.
//
// No errors are expected during normal operations.
func (p *RegisterPruner) batchDelete(ctx context.Context, lookupKeys [][]byte) error {
	batch := p.db.NewBatch()
	defer batch.Close()

	for _, key := range lookupKeys {
		if err := batch.Delete(key, nil); err != nil {
			keyHeight, registerID, _ := lookupKeyToRegisterID(key)
			return fmt.Errorf("failed to delete lookupKey: %w %d %v", err, keyHeight, registerID)
		}
	}

	if err := batch.Commit(pebble.Sync); err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	if p.metrics != nil {
		p.metrics.NumberOfRowsPruned(uint64(len(lookupKeys)))
	}

	// Throttle to prevent excessive system load
	select {
	case <-ctx.Done():
	case <-time.After(p.pruneThrottleDelay):
	}

	return nil
}

// updateFirstStoredHeight updates the first stored height in the database to the specified height.
// The height is stored using the `firstHeightKey` key.
//
// Parameters:
//   - height: The height value to store as the first stored height.
//
// No errors are expected during normal operations.
func (p *RegisterPruner) updateFirstStoredHeight(height uint64) error {
	return p.db.Set(firstHeightKey, encodedUint64(height), nil)
}
