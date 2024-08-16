package pruner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/heightrecorder"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

const (
	// DefaultPruneInterval is the default interval between pruning cycles
	DefaultPruneInterval = 5 * time.Second

	// DefaultThrottleDelay is the default delay between pruning individual blocks
	DefaultThrottleDelay = 20 * time.Millisecond
)

type Core interface {
	Prune(ctx context.Context, baseHeader *flow.Header) error
}

type Pruner struct {
	component.Component

	log     zerolog.Logger
	headers storage.Headers

	rootSealedHeight uint64
	progress         storage.ConsumerProgress

	core           Core
	heightRecorder *heightrecorder.Manager

	mu sync.RWMutex
}

func New(
	log zerolog.Logger,
	core Core,
	heightRecorder *heightrecorder.Manager,
	headers storage.Headers,
	progress storage.ConsumerProgress,
	rootSealedHeight uint64,
) *Pruner {
	p := &Pruner{
		log: log.With().Str("component", "protocoldb_pruner").Logger(),

		core:             core,
		headers:          headers,
		progress:         progress,
		heightRecorder:   heightRecorder,
		rootSealedHeight: rootSealedHeight,
	}

	p.Component = component.NewComponentManagerBuilder().
		AddWorker(p.loop).
		Build()

	return p
}

// initProgress initializes the pruned height consumer progress
// If the progress is not found, it is initialized to the root sealed block height
// No errors are expected during normal operation.
func (p *Pruner) initProgress() (uint64, error) {
	lowestHeight, err := p.progress.ProcessedIndex()
	if err == nil {
		return lowestHeight, nil
	}

	if !errors.Is(err, storage.ErrNotFound) {
		return 0, fmt.Errorf("could not get processed index: %w", err)
	}

	err = p.progress.InitProcessedIndex(p.rootSealedHeight)
	if err != nil && !errors.Is(err, storage.ErrAlreadyExists) {
		return 0, fmt.Errorf("could not init processed index: %w", err)
	}

	p.log.Warn().
		Uint64("processed index", lowestHeight).
		Msg("processed index not found, initialized.")

	return p.progress.ProcessedIndex()
}

func (p *Pruner) loop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	lowestHeight, err := p.initProgress()
	if err != nil {
		ctx.Throw(err)
		return
	}

	ready()

	p.log.Info().
		Uint64("lowestHeight", lowestHeight).
		Msg("starting pruner")

	ticker := time.NewTicker(DefaultPruneInterval)
	defer ticker.Stop()

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		// baseHeight is the highest height for which we have completed all processing
		baseHeight, ok := p.heightRecorder.HighestCompleteHeight()
		if !ok {
			p.log.Debug().Msg("no registered height recorders")
			continue
		}

		baseHeader, err := p.headers.ByHeight(baseHeight)
		if err != nil {
			ctx.Throw(fmt.Errorf("could not get header for height %d: %w", baseHeight, err))
			return
		}

		err = p.core.Prune(ctx, baseHeader)
		if err != nil {
			ctx.Throw(err)
			return
		}
	}
}
