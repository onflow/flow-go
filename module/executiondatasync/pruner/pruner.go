package pruner

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/tracker"
	"github.com/onflow/flow-go/module/irrecoverable"
)

const (
	defaultHeightRangeTarget = uint64(400_000)
	defaultThreshold         = uint64(100_000)
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

	// channels used to send new fulfilled heights and config changes to the worker thread
	fulfilledHeight       engine.Notifier
	thresholdChan         chan uint64
	heightRangeTargetChan chan uint64

	lastFulfilledHeight uint64
	lastPrunedHeight    uint64

	// the height range is the range of heights between the last pruned and last fulfilled
	// heightRangeTarget is the target minimum value for this range, so that after pruning
	// the height range is equal to the target.
	heightRangeTarget uint64

	// threshold defines the maximum height range and how frequently pruning is performed.
	// once the height range reaches `heightRangeTarget+threshold`, `threshold` many blocks
	// are pruned
	threshold uint64

	logger  zerolog.Logger
	metrics module.ExecutionDataPrunerMetrics

	component.Component
	cm *component.ComponentManager

	registeredProducers []execution_data.ExecutionDataProducer
}

type PrunerOption func(*Pruner)

// WithHeightRangeTarget is used to configure the pruner with a custom
// height range target.
func WithHeightRangeTarget(heightRangeTarget uint64) PrunerOption {
	return func(p *Pruner) {
		p.heightRangeTarget = heightRangeTarget
	}
}

// WithThreshold is used to configure the pruner with a custom threshold.
func WithThreshold(threshold uint64) PrunerOption {
	return func(p *Pruner) {
		p.threshold = threshold
	}
}

func WithPruneCallback(callback func(context.Context) error) PrunerOption {
	return func(p *Pruner) {
		p.pruneCallback = callback
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

	fulfilledHeights := make(chan uint64, 32)
	fulfilledHeights <- fulfilledHeight

	p := &Pruner{
		logger:                logger.With().Str("component", "execution_data_pruner").Logger(),
		storage:               storage,
		pruneCallback:         func(ctx context.Context) error { return nil },
		fulfilledHeight:       engine.NewNotifier(),
		thresholdChan:         make(chan uint64),
		heightRangeTargetChan: make(chan uint64),
		lastFulfilledHeight:   fulfilledHeight,
		lastPrunedHeight:      lastPrunedHeight,
		heightRangeTarget:     defaultHeightRangeTarget,
		threshold:             defaultThreshold,
		metrics:               metrics,
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

func (p *Pruner) RegisterProducer(producer execution_data.ExecutionDataProducer) {
	fmt.Println("RegisterProducer")
	producer.Register(&p.fulfilledHeight)
	fmt.Println("RegisterProducer end ")

	p.registeredProducers = append(p.registeredProducers, producer)
}

// SetHeightRangeTarget updates the Pruner's height range target.
// This may block for the duration of a pruning operation.
func (p *Pruner) SetHeightRangeTarget(heightRangeTarget uint64) error {
	select {
	case p.heightRangeTargetChan <- heightRangeTarget:
		return nil
	case <-p.cm.ShutdownSignal():
		return component.ErrComponentShutdown
	}
}

// SetThreshold update's the Pruner's threshold.
// This may block for the duration of a pruning operation.
func (p *Pruner) SetThreshold(threshold uint64) error {
	select {
	case p.thresholdChan <- threshold:
		return nil
	case <-p.cm.ShutdownSignal():
		return component.ErrComponentShutdown
	}
}

func (p *Pruner) loop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.fulfilledHeight.Channel():
			lowestHeight, err := p.lowestProducersHeight()
			if err != nil {
				ctx.Throw(fmt.Errorf("failed to get lowest fulfilled height: %w", err))
			}

			err = p.updateFulfilledHeight(lowestHeight)
			if err != nil {
				ctx.Throw(fmt.Errorf("failed to update lowest fulfilled height: %w", err))
			}

			p.checkPrune(ctx)
		case heightRangeTarget := <-p.heightRangeTargetChan:
			p.heightRangeTarget = heightRangeTarget
			p.checkPrune(ctx)
		case threshold := <-p.thresholdChan:
			p.threshold = threshold
			p.checkPrune(ctx)
		}
	}
}

func (p *Pruner) lowestProducersHeight() (uint64, error) {
	var heights []uint64
	for _, producer := range p.registeredProducers {
		height, err := producer.LastProducedHeight()
		if err != nil {
			return 0, err
		}
		heights = append(heights, height)
	}
	sort.Slice(heights, func(i, j int) bool { return heights[i] < heights[j] })
	return heights[0], nil
}

func (p *Pruner) updateFulfilledHeight(height uint64) error {
	if height > p.lastFulfilledHeight {
		p.lastFulfilledHeight = height
		return p.storage.SetFulfilledHeight(height)
	}
	return nil
}

func (p *Pruner) checkPrune(ctx irrecoverable.SignalerContext) {
	if p.lastFulfilledHeight > p.heightRangeTarget+p.threshold+p.lastPrunedHeight {
		pruneHeight := p.lastFulfilledHeight - p.heightRangeTarget

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
