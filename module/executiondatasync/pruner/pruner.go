package pruner

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/tracker"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
	"github.com/rs/zerolog"
)

const (
	defaultHeightRangeTarget = uint64(400000)
	defaultThreshold         = uint64(100000)
)

type Pruner struct {
	storage *tracker.Storage

	fulfilledHeightsIn    chan<- interface{}
	fulfilledHeightsOut   <-chan interface{}
	thresholdChan         chan uint64
	heightRangeTargetChan chan uint64

	lastPrunedHeight  uint64
	heightRangeTarget uint64
	threshold         uint64

	logger  zerolog.Logger
	metrics module.ExecutionDataPrunerMetrics

	component.Component
	cm *component.ComponentManager
}

type PrunerOption func(*Pruner)

func WithHeightRangeTarget(heightRangeTarget uint64) PrunerOption {
	return func(p *Pruner) {
		p.heightRangeTarget = heightRangeTarget
	}
}

func WithThreshold(threshold uint64) PrunerOption {
	return func(p *Pruner) {
		p.threshold = threshold
	}
}

func NewPruner(logger zerolog.Logger, metrics module.ExecutionDataPrunerMetrics, storage *tracker.Storage, opts ...PrunerOption) (*Pruner, error) {
	lastPrunedHeight, err := storage.GetPrunedHeight()
	if err != nil {
		return nil, fmt.Errorf("failed to get pruned height: %w", err)
	}

	fulfilledHeight, err := storage.GetFulfilledHeight()
	if err != nil {
		return nil, fmt.Errorf("failed to get fulfilled height: %w", err)
	}

	fulfilledHeightsIn, fulfilledHeightsOut := util.UnboundedChannel()
	fulfilledHeightsIn <- fulfilledHeight

	p := &Pruner{
		logger:                logger.With().Str("component", "execution_data_pruner").Logger(),
		storage:               storage,
		fulfilledHeightsIn:    fulfilledHeightsIn,
		fulfilledHeightsOut:   fulfilledHeightsOut,
		thresholdChan:         make(chan uint64),
		heightRangeTargetChan: make(chan uint64),
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

func (p *Pruner) NotifyFulfilledHeight(height uint64) {
	if util.CheckClosed(p.cm.ShutdownSignal()) {
		return
	}

	p.fulfilledHeightsIn <- height
}

func (p *Pruner) SetHeightRangeTarget(heightRangeTarget uint64) error {
	select {
	case p.heightRangeTargetChan <- heightRangeTarget:
		return nil
	case <-p.cm.ShutdownSignal():
		return component.ErrComponentShutdown
	}
}

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
		case h := <-p.fulfilledHeightsOut:
			fulfilledHeight := h.(uint64)
			if fulfilledHeight-p.lastPrunedHeight > p.heightRangeTarget+p.threshold {
				pruneHeight := fulfilledHeight - p.heightRangeTarget

				p.logger.Info().Uint64("prune_height", pruneHeight).Msg("pruning storage")
				start := time.Now()

				if err := p.storage.Prune(pruneHeight); err != nil {
					ctx.Throw(fmt.Errorf("failed to prune: %w", err))
				}

				duration := time.Since(start)
				p.logger.Info().Dur("duration", duration).Msg("pruned storage")

				p.metrics.Pruned(pruneHeight, duration)

				p.lastPrunedHeight = pruneHeight
			}
		case heightRangeTarget := <-p.heightRangeTargetChan:
			p.heightRangeTarget = heightRangeTarget
		case threshold := <-p.thresholdChan:
			p.threshold = threshold
		}
	}
}
