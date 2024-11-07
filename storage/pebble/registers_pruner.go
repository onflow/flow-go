package pebble

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

const (
	// DefaultPruneThreshold defines the default number of blocks to retain below the latest block height. Blocks below
	// this threshold will be considered for pruning if they exceed the PruneInterval.
	DefaultPruneThreshold = uint64(100_000)

	// DefaultPruneThrottleDelay is the default delay between each batch of keys inspected and pruned. This helps
	// to reduce the load on the database during pruning.
	DefaultPruneThrottleDelay = 10 * time.Millisecond

	// DefaultPruneTickerInterval is the default interval between consecutive pruning checks. Pruning will be triggered
	// at this interval if conditions are met.
	DefaultPruneTickerInterval = 10 * time.Minute
)

// TODO: This configuration should be changed after testing it with real network data for better performance.
const (
	// pruneIntervalRatio represents an additional percentage of pruneThreshold which is used to calculate PruneInterval
	// Pruning will start if there are more than `(1 + pruneIntervalRatio) * pruneThreshold` unpruned blocks
	pruneIntervalRatio = 0.1 // 10%
	// deleteItemsPerBatch defines the number of database keys to delete in each batch operation.
	// This value is used to control the size of deletion operations during pruning.
	deleteItemsPerBatch = 256
)

// PruneInterval calculates the interval at which pruning is triggered based on the given pruneThreshold and
// the pruneIntervalRatio.
func PruneInterval(threshold uint64) uint64 {
	return threshold + uint64(float64(threshold)*pruneIntervalRatio)
}

// RegisterPruner manages the pruning process for register storage, handling
// threshold-based deletion of old data from the Pebble DB to optimize storage use.
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

// PrunerOption is a functional option used to configure a RegisterPruner instance.
type PrunerOption func(*RegisterPruner)

// WithPruneThreshold configures the RegisterPruner with a custom threshold. The pruneThreshold sets the number of
// blocks below the latest height to keep.
func WithPruneThreshold(threshold uint64) PrunerOption {
	return func(p *RegisterPruner) {
		p.pruneThreshold = threshold
		p.pruneInterval = PruneInterval(threshold)
	}
}

// WithPruneThrottleDelay configures the RegisterPruner with a custom delay between batches of keys inspected and pruned,
// reducing load on the database.
func WithPruneThrottleDelay(throttleDelay time.Duration) PrunerOption {
	return func(p *RegisterPruner) {
		p.pruneThrottleDelay = throttleDelay
	}
}

// WithPruneTickerInterval configures the RegisterPruner with a custom interval between consecutive pruning checks.
func WithPruneTickerInterval(interval time.Duration) PrunerOption {
	return func(p *RegisterPruner) {
		p.pruneTickerInterval = interval
	}
}

// WithPrunerMetrics sets the metrics interface for a RegisterPruner instance, allowing tracking of pruning performance
// and key metrics.
func WithPrunerMetrics(metrics module.RegisterDBPrunerMetrics) PrunerOption {
	return func(p *RegisterPruner) {
		p.metrics = metrics
	}
}

// NewRegisterPruner creates and initializes a new RegisterPruner instance with the specified logger, database connection,
// and optional configurations provided via PrunerOptions. This sets up the pruning component and returns an error if
// any issues occur.
func NewRegisterPruner(
	logger zerolog.Logger,
	db *pebble.DB,
	opts ...PrunerOption,
) (*RegisterPruner, error) {
	pruner := &RegisterPruner{
		logger:              logger.With().Str("component", "registerdb_pruner").Logger(),
		db:                  db,
		pruneInterval:       PruneInterval(DefaultPruneThreshold),
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

	worker := NewIntervalWorker(p.pruneTickerInterval)

	worker.Run(ctx, func() {
		if err := p.checkPrune(ctx); err != nil {
			ctx.Throw(err)
		}
	})
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

	if firstHeight > latestHeight {
		return errors.New("the latest height must be greater than the first height")
	}

	if latestHeight-firstHeight <= p.pruneInterval {
		return nil
	}

	pruneHeight := latestHeight - p.pruneThreshold
	p.logger.Info().Uint64("prune_height", pruneHeight).Msg("pruning storage")

	if err = p.pruneUpToHeight(ctx, p.db, pruneHeight); err != nil {
		return fmt.Errorf("failed to prune: %w", err)
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
func (p *RegisterPruner) pruneUpToHeight(ctx context.Context, r pebble.Reader, pruneHeight uint64) error {
	start := time.Now()

	// first, update firstHeight in the db
	// this ensures that if the node crashes during pruning, there will still be a consistent
	// view when the node starts up. Subsequent prunes will remove any leftover data.
	if err := p.updateFirstStoredHeight(pruneHeight); err != nil {
		return fmt.Errorf("failed to update first height for register storage: %w", err)
	}

	dbPruner := RegisterPrunerRun(p.db, p.logger, p.pruneThrottleDelay, pruneHeight)

	prefix := []byte{codeRegister}
	it, err := r.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: []byte{codeRegister + 1},
	})
	if err != nil {
		return fmt.Errorf("cannot create iterator: %w", err)
	}

	defer func() {
		if cerr := it.Close(); cerr != nil {
			p.logger.Err(cerr).Msg("error while closing the iterator")
		}
	}()

	var batchKeysToRemove [][]byte

	for it.SeekGE(prefix); it.Valid(); it.Next() {
		key := it.Key()

		canPruneKey, err := dbPruner.CanPruneKey(key)
		if err != nil {
			return fmt.Errorf("cannot check key %v for pruning, reason: %w", key, err)
		}

		if !canPruneKey {
			continue
		}

		// Create a copy of the key to avoid memory issues
		batchKeysToRemove = append(batchKeysToRemove, bytes.Clone(key))

		if len(batchKeysToRemove) >= deleteItemsPerBatch {
			// Perform batch delete
			if err := dbPruner.BatchDelete(ctx, batchKeysToRemove); err != nil {
				return err
			}

			// Throttle to prevent excessive system load
			select {
			case <-ctx.Done():
			case <-time.After(p.pruneThrottleDelay):
			}

			// Reset batchKeysToRemove to empty slice while retaining capacity
			batchKeysToRemove = batchKeysToRemove[:0]
		}
	}

	if len(batchKeysToRemove) > 0 {
		// Perform the final batch delete if there are any remaining keys
		if err := dbPruner.BatchDelete(ctx, batchKeysToRemove); err != nil {
			return err
		}
	}

	p.logger.Info().
		Uint64("height", pruneHeight).
		Int("keys_pruned", dbPruner.TotalKeysPruned()).
		Dur("duration_ms", time.Since(start)).
		Msg("pruning complete")

	p.metrics.LatestPrunedHeight(pruneHeight)

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
	return p.db.Set(firstHeightKey, encodedUint64(height), pebble.Sync)
}
