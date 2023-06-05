package finalized_and_executed

import (
	"errors"
	"fmt"
	"sync"

	"github.com/hashicorp/golang-lru/simplelru"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	psEvents "github.com/onflow/flow-go/state/protocol/events"
	storageerr "github.com/onflow/flow-go/storage"
)

type Consumer interface {
	FinalizedAndExecuted(h *flow.Header)
}

// StopControlHeaders is an interface for fetching headers
// Its jut a small subset of storage.Headers for comments see storage.Headers
type StopControlHeaders interface {
	ByHeight(height uint64) (*flow.Header, error)
}

type Distributor struct {
	// adding psEvents.Noop makes this a protocol.Consumer
	psEvents.Noop
	DistributorConfig

	component.Component
	cm *component.ComponentManager

	// mu protects subscribers and highestFinalizedAndExecutedBlock
	// everything else is expected to be accessed only from one goroutine
	mu                               sync.RWMutex
	subscribers                      []Consumer
	highestFinalizedAndExecutedBlock *flow.Header

	blockFinalizedChan chan *flow.Header
	blockExecutedChan  chan *flow.Header

	// The amount of db calls that are expected per height is
	// 1 (to check if the block is executed) + N (to check if the block is finalized)
	// where N is the number of execution forks (N >= 1).
	// The caches reduce the number of db calls by ~1 per height as long as the execution
	// and finalization are not to far apart (see DistributorConfig.LRUCacheSize).
	executedLru  simplelru.LRUCache
	finalizedLru simplelru.LRUCache

	// exeState is used to check if a block is executed
	exeState state.ReadOnlyExecutionState
	// headers are used to check if a block is finalized
	headers StopControlHeaders

	log zerolog.Logger
}

type DistributorConfig struct {
	// cacheSize is the size of the LRU cache used to store the last finalized and
	// executed blocks. Two caches are created with this size. the cache is used to avoid
	// checking the database if a block has been executed or finalized.
	LRUCacheSize      int
	ChannelBufferSize int
}

var DefaultDistributorConfig = DistributorConfig{
	LRUCacheSize:      1000,
	ChannelBufferSize: 1000,
}

func NewDistributor(
	log zerolog.Logger,
	highestFinalizedAndExecutedBlock *flow.Header,
	exeState state.ReadOnlyExecutionState,
	headers StopControlHeaders,
	config DistributorConfig,
) *Distributor {

	// the events should be consumed very fast but, just in case,
	// we use a buffered channel. We should not miss any events.
	blockFinalizedChan := make(chan *flow.Header, config.ChannelBufferSize)
	blockExecutedChan := make(chan *flow.Header, config.ChannelBufferSize)

	d := &Distributor{
		log: log.With().
			Str("component", "finalized_and_executed_distributor").
			Logger(),

		subscribers:        []Consumer{},
		blockExecutedChan:  blockFinalizedChan,
		blockFinalizedChan: blockExecutedChan,

		highestFinalizedAndExecutedBlock: highestFinalizedAndExecutedBlock,

		exeState: exeState,
		headers:  headers,

		DistributorConfig: config,
	}

	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(d.processEvents)

	d.cm = cm.Build()
	d.Component = d.cm

	return d
}

func (d *Distributor) AddConsumer(consumer Consumer) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.subscribers = append(d.subscribers, consumer)
}

// GetHighestFinalizedAndExecutedBlock returns the highest block that has been
// finalized and executed
func (d *Distributor) GetHighestFinalizedAndExecutedBlock() *flow.Header {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.highestFinalizedAndExecutedBlock
}

func (d *Distributor) BlockFinalized(h *flow.Header) {
	d.blockFinalizedChan <- h
}

func (d *Distributor) BlockExecuted(h *flow.Header) {
	d.blockExecutedChan <- h
}

type blockEvent int

const (
	finalized blockEvent = 0
	executed  blockEvent = 1
)

// processEvents is a worker that processes the finalized or executed events
func (d *Distributor) processEvents(
	ctx irrecoverable.SignalerContext,
	ready component.ReadyFunc,
) {
	executedLru, err := simplelru.NewLRU(d.LRUCacheSize, nil)
	if err != nil {
		err = fmt.Errorf("failed to create executed LRU cache: %w", err)
		d.log.Err(err).
			Int("cache_size", d.LRUCacheSize).
			Msg("failed to create LRU cache")
		ctx.Throw(err)
		return
	}
	finalizedLru, err := simplelru.NewLRU(d.LRUCacheSize, nil)
	if err != nil {
		err = fmt.Errorf("failed to create finalized LRU cache: %w", err)
		d.log.Err(err).
			Int("cache_size", d.LRUCacheSize).
			Msg("failed to create LRU cache")
		ctx.Throw(err)
		return
	}

	d.executedLru = executedLru
	d.finalizedLru = finalizedLru

	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case h := <-d.blockFinalizedChan:
			d.onBlockEvent(ctx, h, finalized)
		case h := <-d.blockExecutedChan:
			d.onBlockEvent(ctx, h, executed)
		}
	}
}

// onBlockEvent is called when a block is finalized or executed.
func (d *Distributor) onBlockEvent(
	ctx irrecoverable.SignalerContext,
	h *flow.Header,
	event blockEvent,
) {
	switch event {
	case finalized:
		d.finalizedLru.Add(h, nil)
	case executed:
		d.executedLru.Add(h, nil)
	}

	if d.highestFinalizedAndExecutedBlock.Height+1 != h.Height {
		// we don't need to process the block if it's not the next one.
		// The assumption here is that the incoming events don't have gaps.
		return
	}

	isExecutedAndFinalized := false
	switch event {
	case finalized:
		isExecutedAndFinalized = d.isBlockExecuted(ctx, h)
	case executed:
		isExecutedAndFinalized = d.isBlockFinalized(ctx, h)
	}

	if !isExecutedAndFinalized {
		return
	}

	// remove the block from the LRU cache
	// we don't need these records anymore
	// since we will never ask for them again.
	d.finalizedLru.Remove(h)
	d.executedLru.Remove(h)

	d.updateHeightAndSignalConsumers(h)
}

// updateHeightAndSignalConsumers updates the highestFinalizedAndExecutedBlock
// and signals all the consumers.
func (d *Distributor) updateHeightAndSignalConsumers(h *flow.Header) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.highestFinalizedAndExecutedBlock = h

	for _, sub := range d.subscribers {
		sub.FinalizedAndExecuted(h)
	}
}

// isBlockExecuted checks if the block is executed.
// db call can cause a ctx.Throw.
//
// no lock is needed since this is called from the processEvents goroutine.
func (d *Distributor) isBlockExecuted(
	ctx irrecoverable.SignalerContext,
	h *flow.Header,
) bool {
	if d.executedLru.Contains(h) {
		return true
	}

	executed, err := state.IsBlockExecuted(ctx, d.exeState, h.ID())
	if err != nil {
		d.log.Error().
			Err(err).
			Msg("failed to check if block is executed")
		ctx.Throw(err)
		return false
	}

	return executed
}

// isBlockFinalized checks if the block is finalized.
// db call can cause a ctx.Throw.
//
// no lock is needed since this is called from the processEvents goroutine.
func (d *Distributor) isBlockFinalized(
	ctx irrecoverable.SignalerContext,
	h *flow.Header,
) bool {
	if d.finalizedLru.Contains(h) {
		return true
	}

	// This uses the fact that only finalized headers can be retrieved by height.
	finalizedHeader, err := d.headers.ByHeight(h.Height)
	if err != nil {
		if errors.Is(err, storageerr.ErrNotFound) {
			return false
		}

		d.log.Err(err).
			Msg("failed to get finalized header")
		ctx.Throw(err)
		return false
	}

	// this assumes the block has been executed. For executed block, it must have a QC,
	// which means its view is unique
	finalized := finalizedHeader.View == h.View

	return finalized
}

// SetHighestFinalizedAndExecutedBlockForTestingOnly sets the highest finalized and
// executed block. Only use for testing.
func (d *Distributor) SetHighestFinalizedAndExecutedBlockForTestingOnly(h *flow.Header) {
	d.highestFinalizedAndExecutedBlock = h
}
