package pruner

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/tracker"
	"github.com/onflow/flow-go/module/irrecoverable"
)

const (
	DefaultHeightRangeTarget = uint64(2_000_000)
	DefaultThreshold         = uint64(100_000)
	DefaultPruningInterval   = 10 * time.Minute
)

// Pruner is a component responsible for pruning data from
// execution data storage. It is configured with the following
// parameters:
//   - Height range target: The target number of most recent blocks
//     to store data for. This controls the total amount of data
//     stored on disk.
//   - Threshold: The number of block heights that we can exceed
//     the height range target by before pruning is triggered. This
//     controls the frequency of pruning.
//
// The Pruner consumes a stream of tracked height notifications,
// and triggers pruning once the difference between the tracked
// height and the last pruned height reaches the height range
// target + threshold.
// A height is considered fulfilled once it has both been executed,
// tracked, and sealed.
type Pruner struct {
	storage       tracker.Storage
	pruneCallback func(ctx context.Context) error

	lastFulfilledHeight uint64
	lastPrunedHeight    uint64

	// the height range is the range of heights between the last pruned and last fulfilled
	// heightRangeTarget is the target minimum value for this range, so that after pruning
	// the height range is equal to the target.
	heightRangeTarget *atomic.Uint64

	// threshold defines the maximum height range and how frequently pruning is performed.
	// once the height range reaches `heightRangeTarget+threshold`, `threshold` many blocks
	// are pruned
	threshold *atomic.Uint64

	// pruningInterval how frequently pruning can be performed
	pruningInterval time.Duration

	logger  zerolog.Logger
	metrics module.ExecutionDataPrunerMetrics

	component.Component
	cm *component.ComponentManager

	registeredHeightRecorders []execution_data.ProcessedHeightRecorder
}

type PrunerOption func(*Pruner)

// WithHeightRangeTarget is used to configure the pruner with a custom
// height range target.
func WithHeightRangeTarget(heightRangeTarget uint64) PrunerOption {
	return func(p *Pruner) {
		p.heightRangeTarget.Store(heightRangeTarget)
	}
}

// WithThreshold is used to configure the pruner with a custom threshold.
func WithThreshold(threshold uint64) PrunerOption {
	return func(p *Pruner) {
		p.threshold.Store(threshold)
	}
}

// WithPruneCallback sets a custom callback function to be called after pruning.
func WithPruneCallback(callback func(context.Context) error) PrunerOption {
	return func(p *Pruner) {
		p.pruneCallback = callback
	}
}

// WithPruningInterval is used to configure the pruner with a custom pruning interval.
func WithPruningInterval(interval time.Duration) PrunerOption {
	return func(p *Pruner) {
		p.pruningInterval = interval
	}
}

// NewPruner creates a new Pruner.
func NewPruner(logger zerolog.Logger, metrics module.ExecutionDataPrunerMetrics, storage tracker.Storage, opts ...PrunerOption) (*Pruner, error) {
	lastPrunedHeight, err := storage.GetPrunedHeight()
	if err != nil {
		return nil, fmt.Errorf("failed to get pruned height: %w", err)
	}

	fulfilledHeight, err := storage.GetFulfilledHeight()
	if err != nil {
		return nil, fmt.Errorf("failed to get fulfilled height: %w", err)
	}

	p := &Pruner{
		logger:              logger.With().Str("component", "execution_data_pruner").Logger(),
		storage:             storage,
		pruneCallback:       func(ctx context.Context) error { return nil },
		lastFulfilledHeight: fulfilledHeight,
		lastPrunedHeight:    lastPrunedHeight,
		heightRangeTarget:   atomic.NewUint64(DefaultHeightRangeTarget),
		threshold:           atomic.NewUint64(DefaultThreshold),
		metrics:             metrics,
		pruningInterval:     DefaultPruningInterval,
	}
	p.cm = component.NewComponentManagerBuilder().
		AddWorker(p.loop).
		Build()
	p.Component = p.cm

	for _, opt := range opts {
		opt(p)
	}

	return p, nil
}

// RegisterHeightRecorder registers an execution data height recorder with the Pruner.
//
// Parameters:
//   - recorder: The execution data height recorder to register.
func (p *Pruner) RegisterHeightRecorder(recorder execution_data.ProcessedHeightRecorder) {
	p.registeredHeightRecorders = append(p.registeredHeightRecorders, recorder)
}

// SetHeightRangeTarget updates the Pruner's height range target.
func (p *Pruner) SetHeightRangeTarget(heightRangeTarget uint64) {
	p.heightRangeTarget.Store(heightRangeTarget)
}

// SetThreshold update's the Pruner's threshold.
func (p *Pruner) SetThreshold(threshold uint64) {
	p.threshold.Store(threshold)
}

// loop is the main worker for the Pruner, responsible for triggering
// pruning operations at regular intervals. It monitors the heights
// of registered height recorders and checks if pruning is necessary.
func (p *Pruner) loop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	ticker := time.NewTicker(p.pruningInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(p.registeredHeightRecorders) > 0 {
				lowestHeight := p.lowestRecordersHeight()

				err := p.updateFulfilledHeight(lowestHeight)
				if err != nil {
					ctx.Throw(fmt.Errorf("failed to update lowest fulfilled height: %w", err))
				}
			}
			p.checkPrune(ctx)
		}
	}
}

// lowestRecordersHeight returns the lowest height among all height recorders.
//
// Returns:
//   - uint64: The lowest height among all registered height recorders.
func (p *Pruner) lowestRecordersHeight() uint64 {
	lowestHeight := uint64(math.MaxUint64)

	for _, recorder := range p.registeredHeightRecorders {
		height := recorder.HighestCompleteHeight()
		if height < lowestHeight {
			lowestHeight = height
		}
	}
	return lowestHeight
}

// updateFulfilledHeight updates the last fulfilled height and stores it in the storage.
//
// Parameters:
//   - height: The new fulfilled height.
//
// No errors are expected during normal operations.
func (p *Pruner) updateFulfilledHeight(height uint64) error {
	if height > p.lastFulfilledHeight {
		p.lastFulfilledHeight = height
		return p.storage.SetFulfilledHeight(height)
	}
	return nil
}

// checkPrune checks if pruning should be performed based on the height range
// and triggers the pruning operation if necessary.
//
// Parameters:
//   - ctx: The context for managing the pruning operation.
func (p *Pruner) checkPrune(ctx irrecoverable.SignalerContext) {
	threshold := p.threshold.Load()
	heightRangeTarget := p.heightRangeTarget.Load()

	if p.lastFulfilledHeight > heightRangeTarget+threshold+p.lastPrunedHeight {
		pruneHeight := p.lastFulfilledHeight - heightRangeTarget

		p.logger.Info().Uint64("prune_height", pruneHeight).Msg("pruning storage")
		start := time.Now()

		if err := p.storage.PruneUpToHeight(pruneHeight); err != nil {
			ctx.Throw(fmt.Errorf("failed to prune: %w", err))
		}

		if err := p.pruneCallback(ctx); err != nil {
			ctx.Throw(err)
		}

		duration := time.Since(start)
		p.logger.Info().Dur("duration", duration).Msg("pruned storage")

		p.metrics.Pruned(pruneHeight, duration)

		p.lastPrunedHeight = pruneHeight
	}
}
