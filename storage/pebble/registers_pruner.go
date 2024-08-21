package pebble

import (
	"fmt"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

const (
	DefaultPruneThreshold      = uint64(100_000)
	DefaultPruneThrottleDelay  = 10 * time.Millisecond
	DefaultPruneTickerInterval = 10 * time.Minute
)

// pruneIntervalRatio represents an additional percentage of pruneThreshold which is used to calculate pruneInterval
const (
	pruneIntervalRatio  = 0.1
	deleteItemsPerBatch = 256
)

// pruneInterval is a helper function which calculates interval for pruner
func pruneInterval(threshold uint64) uint64 {
	return threshold + uint64(float64(threshold)*pruneIntervalRatio)
}

type RegisterPruner struct {
	component.Component
	componentManager *component.ComponentManager

	logger zerolog.Logger
	db     *pebble.DB

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

func NewRegisterPruner(
	logger zerolog.Logger,
	db *pebble.DB,
	opts ...PrunerOption,
) (*RegisterPruner, error) {
	pruner := &RegisterPruner{
		logger:              logger.With().Str("component", "register_pruner").Logger(),
		db:                  db,
		pruneInterval:       pruneInterval(DefaultPruneThreshold),
		pruneThreshold:      DefaultPruneThreshold,
		pruneThrottleDelay:  DefaultPruneThrottleDelay,
		pruneTickerInterval: DefaultPruneTickerInterval,
	}

	pruner.componentManager = component.NewComponentManagerBuilder().
		AddWorker(pruner.loop).
		Build()
	pruner.Component = pruner.componentManager

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
			p.checkPrune(ctx)
		}
	}
}

// checkPrune checks if pruning should be performed based on the height range
// and triggers the pruning operation if necessary.
//
// Parameters:
//   - ctx: The context for managing the pruning operation.
func (p *RegisterPruner) checkPrune(ctx irrecoverable.SignalerContext) {
	firstHeight, err := firstStoredHeight(p.db)
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to get first height from register storage: %w", err))
	}

	latestHeight, err := latestStoredHeight(p.db)
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to get latest height from register storage: %w", err))
	}

	pruneHeight := latestHeight - p.pruneThreshold

	if pruneHeight-firstHeight > p.pruneInterval {
		p.logger.Info().Uint64("prune_height", pruneHeight).Msg("pruning storage")

		err := p.pruneUpToHeight(pruneHeight)
		if err != nil {
			ctx.Throw(fmt.Errorf("failed to prune: %w", err))
		}

		// update first indexed height
		err = updateFirstStoredHeight(p.db, pruneHeight)
		if err != nil {
			ctx.Throw(fmt.Errorf("failed to update first height for register storage: %w", err))
		}
	}
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
//   - pruneHeight: The maximum height of entries to prune.
//
// No errors are expected during normal operations.
func (p *RegisterPruner) pruneUpToHeight(pruneHeight uint64) error {
	prefix := []byte{codeRegister}
	itemsPerBatch := deleteItemsPerBatch
	var batchKeysToRemove [][]byte

	err := func(r pebble.Reader) error {
		options := pebble.IterOptions{
			LowerBound: prefix,
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

			// New register prefix, reset the state
			if !keepKeyFound || lastRegisterID != registerID {
				keepKeyFound = false
				lastRegisterID = registerID
			}

			if keyHeight <= pruneHeight {
				if !keepKeyFound {
					// Keep the first entry found for this registerID that is <= pruneHeight
					keepKeyFound = true
					continue
				}

				// Otherwise, mark this key for removal
				batchKeysToRemove = append(batchKeysToRemove, key)

				if len(batchKeysToRemove) == itemsPerBatch {
					// Perform batch delete
					if err := p.batchDelete(batchKeysToRemove); err != nil {
						return err
					}
					batchKeysToRemove = nil
				}
			}
		}

		if len(batchKeysToRemove) > 0 {
			// Perform the final batch delete if there are any remaining keys
			if err := p.batchDelete(batchKeysToRemove); err != nil {
				return err
			}
		}

		return nil
	}(p.db)
	if err != nil {
		return err
	}

	return nil
}

// batchDelete deletes the provided keys from the database in a single batch operation.
// It creates a new batch, deletes each key from the batch, and commits the batch to ensure
// that the deletions are applied atomically.
//
// Parameters:
//   - lookupKeys: A slice of keys to be deleted from the database.
//
// No errors are expected during normal operations.
func (p *RegisterPruner) batchDelete(lookupKeys [][]byte) error {
	batch := p.db.NewBatch()
	defer batch.Close()

	for _, key := range lookupKeys {
		if err := batch.Delete(key, nil); err != nil {
			return fmt.Errorf("failed to delete lookupKey: %w", err)
		}
	}

	if err := batch.Commit(pebble.Sync); err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	return nil
}
