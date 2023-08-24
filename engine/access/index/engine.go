package index

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// Engine represents the ingestion engine, used to funnel data from other nodes
// to a centralized location that can be queried by a user
type Engine struct {
	*component.ComponentManager
	executionDataNotifier engine.Notifier

	log     zerolog.Logger
	metrics module.AccessMetrics

	// storage
	// FIX: remove direct DB access by substituting indexer module
	collections  storage.Collections
	events       storage.Events
	transactions storage.Transactions

	index module.Indexer

	execDataCache *cache.ExecutionDataCache

	// lastFullyProcessedHeight contains the last block height for which we have fully indexed all
	// execution data.
	lastFullyProcessedHeight counters.SequentialCounter

	// highestHeight contains the highest consecutive block height for which we have received a
	// new Execution Data notification.
	highestHeight counters.StrictMonotonousCounter
}

// New creates a new access ingestion engine
func New(
	log zerolog.Logger,
	accessMetrics module.AccessMetrics,
	headers storage.Headers,
	collections storage.Collections,
	events storage.Events,
	transactions storage.Transactions,
	execDataCache *cache.ExecutionDataCache,
	index module.Indexer,
	lastFullyIndexedHeight uint64,
	highestAvailableHeight uint64,
) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		log:     log.With().Str("engine", "index").Logger(),
		metrics: accessMetrics,

		collections:   collections,
		events:        events,
		transactions:  transactions,
		execDataCache: execDataCache,
		index:         index,

		lastFullyProcessedHeight: counters.NewSequentialCounter(lastFullyIndexedHeight),
		highestHeight:            counters.NewMonotonousCounter(highestAvailableHeight),

		executionDataNotifier: engine.NewNotifier(),
	}

	// Add workers
	e.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(e.processExecutionData).
		Build()

	return e, nil
}

// OnExecutionData is called to notify the engine when a new execution data is received.
// The caller must guarantee that execution data is locally available for all blocks with
// heights between the initialBlockHeight provided during startup and the block height of
// the execution data provided.
func (e *Engine) OnExecutionData(executionData *execution_data.BlockExecutionDataEntity) {
	lg := e.log.With().Hex("block_id", logging.ID(executionData.BlockID)).Logger()

	lg.Trace().Msg("received execution data")

	height, err := e.index.HeightForBlock(executionData.BlockID)
	if err != nil {
		// if the execution data is available, the block must be locally finalized
		lg.Fatal().Err(err).Msg("failed to get header for execution data")
		return
	}

	if ok := e.highestHeight.Set(height); !ok {
		// this means that the height was lower than the current highest height

		// OnExecutionData is guaranteed by the requester to be called in order, but may be called
		// multiple times for the same block.
		lg.Debug().Msg("execution data for block already received")
		return
	}

	e.executionDataNotifier.Notify()
}

func (e *Engine) processExecutionData(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	notifier := e.executionDataNotifier.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case <-notifier:
			err := e.processAvailableExecutionData(ctx)
			if err != nil {
				// if an error reaches this point, it is unexpected
				ctx.Throw(err)
				return
			}
		}
	}
}

func (e *Engine) processAvailableExecutionData(ctx context.Context) error {

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// TODO: loop over all heights between our latest and the highest seen, and index
		// maybe use a jobqueue?

		height := e.lastFullyProcessedHeight.Value() + 1

		// reached the end
		if height > e.highestHeight.Value() {
			return nil
		}

		execData, err := e.getExecutionData(ctx, height)
		if err != nil {
			return fmt.Errorf("could not get execution data for block height %d: %w", height, err)
		}

		if err := e.handleExecutionData(execData); err != nil {
			return err
		}

		if ok := e.lastFullyProcessedHeight.Set(height); !ok {
			// TODO: should this cause the node to crash?
			return fmt.Errorf("could not set last processed height to %d", height)
		}

		err = e.index.StoreLast(height)
		if err != nil {
			return fmt.Errorf("could not persist last processed height to %d: %w", height, err)
		}

	}
}

// getExecutionData returns the execution data for the given block height.
// Expected errors during normal operation:
// - storage.ErrNotFound or execution_data.BlobNotFoundError: execution data for the given block height is not available.
func (e *Engine) getExecutionData(ctx context.Context, height uint64) (*execution_data.BlockExecutionDataEntity, error) {
	// fail early if no notification has been received for the given block height.
	// note: it's possible for the data to exist in the data store before the notification is
	// received. this ensures a consistent view is available to all callers.
	if height > e.highestHeight.Value() {
		return nil, fmt.Errorf("execution data for block %d is not available yet: %w", height, storage.ErrNotFound)
	}

	// TODO: we get finalized and not sealed block execution data, this could lead to issues if finalized block is not sealed.
	execData, err := e.execDataCache.ByHeight(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("could not get execution data for block %d: %w", height, err)
	}

	return execData, nil
}

func (e *Engine) handleExecutionData(execData *execution_data.BlockExecutionDataEntity) error {
	// TODO: should we parallelize indexing collections and events for each chunk?
	for i, chunkData := range execData.ChunkExecutionDatas {
		err := e.handleCollection(execData.BlockID, chunkData.Collection)
		if err != nil {
			return fmt.Errorf("could not index collection for chunk %d: %w", i, err)
		}

		err = e.handleEvents(execData.BlockID, chunkData.Events)
		if err != nil {
			return fmt.Errorf("could not index events for chunk %d: %w", i, err)
		}

		err = e.handleTrieUpdate(execData.BlockID, chunkData.TrieUpdate)
		if err != nil {
			return fmt.Errorf("could not index trie update for chunk %d: %w", i, err)
		}
	}

	return nil
}

func (e *Engine) handleCollection(blockID flow.Identifier, collection *flow.Collection) error {
	collID := collection.ID()
	light := collection.Light()

	lg := e.log.With().
		Hex("block_id", logging.ID(blockID)).
		Hex("collection_id", logging.ID(collID)).
		Logger()

	// if we haven't already indexed the collection, index it now
	// this is an optimization while we're still using the ingestion engine for collections.
	found, err := e.lookupCollection(collID)
	if err != nil {
		return err
	}

	// collection already indexed, nothing to do
	if found {
		return nil
	}

	// store the light collection (collection minus the transaction body - those are stored separately)
	// and add transaction ids as index
	err = e.collections.StoreLightAndIndexByTransaction(&light)
	if err != nil {
		// ignore collection if already seen
		if errors.Is(err, storage.ErrAlreadyExists) {
			lg.Debug().Msg("collection is already seen")
			return nil
		}
		return err
	}

	// now store each of the transaction body
	for _, tx := range collection.Transactions {
		err := e.transactions.Store(tx)
		if err != nil {
			return fmt.Errorf("could not store transaction (%x): %w", tx.ID(), err)
		}
	}

	return nil
}

func (e *Engine) handleEvents(blockID flow.Identifier, events flow.EventsList) error {
	// Note: service events are currently not included in execution data
	// see https://github.com/onflow/flow-go/issues/4624
	return e.events.Store(blockID, []flow.EventsList{events})
}

func (e *Engine) handleTrieUpdate(blockID flow.Identifier, update *ledger.TrieUpdate) error {
	// nothing to update
	if update == nil {
		return nil
	}

	if len(update.Paths) != len(update.Payloads) {
		return fmt.Errorf("trie update paths and payloads have different lengths")
	}

	height, err := e.index.HeightForBlock(blockID)
	if err != nil {
		return err
	}

	err = e.index.StorePayloads(update.Payloads, height)
	if err != nil {
		return err
	}

	return e.index.StoreCommitment(flow.StateCommitment(update.RootHash), height)
}

// lookupCollection looks up the collection from the collection db with collID
func (e *Engine) lookupCollection(collId flow.Identifier) (bool, error) {
	_, err := e.collections.LightByID(collId)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}
	return false, fmt.Errorf("failed to retrieve collection %s: %w", collId.String(), err)
}
