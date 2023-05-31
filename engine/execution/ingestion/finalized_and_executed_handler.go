package ingestion

import (
	"context"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
)

type OnBlockFinalizedAndExecutedConsumer = func(h *flow.Header)

// FinalizedAndExecutedDispatcher dispatches on block finalized and executed events
type FinalizedAndExecutedDispatcher interface {
	AddOnFinalizedAndExecutedConsumer(OnBlockFinalizedAndExecutedConsumer)
	OnBlockExecuted(ctx context.Context, h *flow.Header) error
	OnBlockFinalized(ctx context.Context, h *flow.Header) error
}

type finalizedAndExecutedDispatcher struct {
	sync.RWMutex
	log zerolog.Logger

	blockFinalizedAndExecutedConsumers []OnBlockFinalizedAndExecutedConsumer
	latestFinalizedAndExecutedHeight   uint64

	isBlockFinalized func(context.Context, *flow.Header) (bool, error)
	isBlockExecuted  func(context.Context, *flow.Header) (bool, error)
}

type FinalizedAndExecutedDispatcherOption func(*finalizedAndExecutedDispatcher)

// FinalizedAndExecutedDispatcherWithLogger sets the logger for the
// FinalizedAndExecutedDispatcher and adds a component field to the logger
func FinalizedAndExecutedDispatcherWithLogger(
	log zerolog.Logger,
) FinalizedAndExecutedDispatcherOption {
	return func(f *finalizedAndExecutedDispatcher) {
		f.log = log.With().
			Str("component", "finalized-and-executed-dispatcher").
			Logger()
	}
}

// NewFinalizedAndExecutedDispatcher returns a new FinalizedAndExecutedDispatcher
func NewFinalizedAndExecutedDispatcher(
	latestFinalizedAndExecutedHeight uint64,
	isBlockFinalized func(context.Context, *flow.Header) (bool, error),
	isBlockExecuted func(context.Context, *flow.Header) (bool, error),
	options ...FinalizedAndExecutedDispatcherOption,
) FinalizedAndExecutedDispatcher {
	d := &finalizedAndExecutedDispatcher{
		log: zerolog.Nop(),

		blockFinalizedAndExecutedConsumers: []OnBlockFinalizedAndExecutedConsumer{},
		latestFinalizedAndExecutedHeight:   latestFinalizedAndExecutedHeight,

		isBlockFinalized: isBlockFinalized,
		isBlockExecuted:  isBlockExecuted,
	}

	for _, option := range options {
		option(d)
	}

	return d
}

func (f *finalizedAndExecutedDispatcher) AddOnFinalizedAndExecutedConsumer(
	consumer OnBlockFinalizedAndExecutedConsumer,
) {
	f.Lock()
	defer f.Unlock()

	f.blockFinalizedAndExecutedConsumers =
		append(f.blockFinalizedAndExecutedConsumers, consumer)
}

func (f *finalizedAndExecutedDispatcher) OnBlockExecuted(
	ctx context.Context,
	h *flow.Header,
) error {
	isNew, err := f.checkOnBlockExecuted(ctx, h)
	if !isNew || err != nil {
		return err
	}

	f.dispatchFinalizedAndExecuted(h)

	return nil
}

func (f *finalizedAndExecutedDispatcher) OnBlockFinalized(
	ctx context.Context,
	h *flow.Header,
) error {
	isNew, err := f.checkOnBlockFinalized(ctx, h)
	if !isNew || err != nil {
		return err
	}

	f.dispatchFinalizedAndExecuted(h)

	return nil
}

// checkOnBlockExecuted checks if this is a new highest executed and finalized block
// we know its executed because we are in the OnBlockExecuted callback
func (f *finalizedAndExecutedDispatcher) checkOnBlockExecuted(
	ctx context.Context,
	h *flow.Header,
) (bool, error) {
	f.RLock()
	defer f.RUnlock()

	// we check latestFinalizedAndExecutedHeight+1 to further ensure that we are not
	// skipping blocks
	if h.Height != f.latestFinalizedAndExecutedHeight+1 {
		return false, nil
	}

	if finalized, err := f.isBlockFinalized(ctx, h); !finalized || err != nil {
		return false, err
	}

	return true, nil
}

// checkOnBlockFinalized checks if this is a new highest executed and finalized block
// we know its finalized because we are in the OnBlockFinalized callback
func (f *finalizedAndExecutedDispatcher) checkOnBlockFinalized(
	ctx context.Context,
	h *flow.Header,
) (bool, error) {
	f.RLock()
	defer f.RUnlock()

	// we check latestFinalizedAndExecutedHeight+1 to further ensure that we are not
	// skipping blocks
	if h.Height != f.latestFinalizedAndExecutedHeight+1 {
		return false, nil
	}

	if executed, err := f.isBlockExecuted(ctx, h); !executed || err != nil {
		return false, err
	}

	return true, nil
}

// dispatchFinalizedAndExecuted dispatches events to all registered consumers
func (f *finalizedAndExecutedDispatcher) dispatchFinalizedAndExecuted(h *flow.Header) {
	f.Lock()
	defer f.Unlock()

	f.latestFinalizedAndExecutedHeight = h.Height

	for _, consumer := range f.blockFinalizedAndExecutedConsumers {
		consumer(h)
	}
}
