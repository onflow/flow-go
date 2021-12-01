// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// time to wait for the all the missing collections to be received at node startup
const collectionCatchupTimeout = 30 * time.Second

// time to poll the storage to check if missing collections have been received
const collectionCatchupDBPollInterval = 10 * time.Millisecond

// time to update the FullBlockHeight index
const fullBlockUpdateInterval = 1 * time.Minute

// a threshold of number of blocks with missing collections beyond which collections should be re-requested
// this is to prevent spamming the collection nodes with request
const missingCollsForBlkThreshold = 100

// a threshold of block height beyond which collections should be re-requested (regardless of the number of blocks for which collection are missing)
// this is to ensure that if a collection is missing for a long time (in terms of block height) it is eventually re-requested
const missingCollsForHeightThreshold = 100

var defaultCollectionCatchupTimeout = collectionCatchupTimeout
var defaultCollectionCatchupDBPollInterval = collectionCatchupDBPollInterval
var defaultFullBlockUpdateInterval = fullBlockUpdateInterval
var defaultMissingCollsForBlkThreshold = missingCollsForBlkThreshold
var defaultMissingCollsForHeightThreshold = missingCollsForHeightThreshold

// Engine represents the ingestion engine, used to funnel data from other nodes
// to a centralized location that can be queried by a user
type Engine struct {
	unit    *engine.Unit     // used to manage concurrency & shutdown
	log     zerolog.Logger   // used to log relevant actions with context
	state   protocol.State   // used to access the  protocol state
	me      module.Local     // used to access local node information
	request module.Requester // used to request collections

	// storage
	// FIX: remove direct DB access by substituting indexer module
	blocks            storage.Blocks
	headers           storage.Headers
	collections       storage.Collections
	transactions      storage.Transactions
	executionReceipts storage.ExecutionReceipts
	executionResults  storage.ExecutionResults

	// metrics
	transactionMetrics         module.TransactionMetrics
	collectionsToMarkFinalized *stdmap.Times
	collectionsToMarkExecuted  *stdmap.Times
	blocksToMarkExecuted       *stdmap.Times

	rpcEngine *rpc.Engine
}

// New creates a new access ingestion engine
func New(
	log zerolog.Logger,
	net network.Network,
	state protocol.State,
	me module.Local,
	request module.Requester,
	blocks storage.Blocks,
	headers storage.Headers,
	collections storage.Collections,
	transactions storage.Transactions,
	executionResults storage.ExecutionResults,
	executionReceipts storage.ExecutionReceipts,
	transactionMetrics module.TransactionMetrics,
	collectionsToMarkFinalized *stdmap.Times,
	collectionsToMarkExecuted *stdmap.Times,
	blocksToMarkExecuted *stdmap.Times,
	rpcEngine *rpc.Engine,
) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	eng := &Engine{
		unit:                       engine.NewUnit(),
		log:                        log.With().Str("engine", "ingestion").Logger(),
		state:                      state,
		me:                         me,
		request:                    request,
		blocks:                     blocks,
		headers:                    headers,
		collections:                collections,
		transactions:               transactions,
		executionResults:           executionResults,
		executionReceipts:          executionReceipts,
		transactionMetrics:         transactionMetrics,
		collectionsToMarkFinalized: collectionsToMarkFinalized,
		collectionsToMarkExecuted:  collectionsToMarkExecuted,
		blocksToMarkExecuted:       blocksToMarkExecuted,
		rpcEngine:                  rpcEngine,
	}

	// register engine with the execution receipt provider
	_, err := net.Register(engine.ReceiveReceipts, eng)
	if err != nil {
		return nil, fmt.Errorf("could not register for results: %w", err)
	}

	return eng, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the ingestion engine, we consider the engine up and running
// upon syncing all the missing collections
func (e *Engine) Ready() <-chan struct{} {
	// request all the missing collection upfront
	readyChan := e.unit.Ready(func() {
		ctx, cancel := context.WithTimeout(context.Background(), defaultCollectionCatchupTimeout)
		defer cancel()
		err := e.requestMissingCollections(ctx)
		if err != nil {
			e.log.Error().Err(err).Msg("requesting missing collections failed")
		}
	})
	e.unit.LaunchPeriodically(e.updateLastFullBlockReceivedIndex, defaultFullBlockUpdateInterval, time.Duration(0))
	return readyChan
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the ingestion engine, it only waits for all submit goroutines to end.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.unit.Launch(func() {
		err := e.process(e.me.NodeID(), event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(channel network.Channel, originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.process(originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(e.me.NodeID(), event)
	})
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

// process processes the given ingestion engine event. Events that are given
// to this function originate within the expulsion engine on the node with the
// given origin ID.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch entity := event.(type) {
	case *flow.ExecutionReceipt:
		return e.handleExecutionReceipt(originID, entity)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// OnFinalizedBlock is called by the follower engine after a block has been finalized and the state has been updated
func (e *Engine) OnFinalizedBlock(hb *model.Block) {
	e.unit.Launch(func() {
		blockID := hb.BlockID
		err := e.processFinalizedBlock(blockID)
		if err != nil {
			e.log.Error().Err(err).Hex("block_id", blockID[:]).Msg("failed to process block")
			return
		}
		e.trackFinalizedMetricForBlock(hb)
	})
}

// processBlock handles an incoming finalized block.
func (e *Engine) processFinalizedBlock(blockID flow.Identifier) error {

	block, err := e.blocks.ByID(blockID)
	if err != nil {
		return fmt.Errorf("failed to lookup block: %w", err)
	}

	// Notify rpc handler of new finalized block height
	e.rpcEngine.SubmitLocal(block)

	// FIX: we can't index guarantees here, as we might have more than one block
	// with the same collection as long as it is not finalized

	// TODO: substitute an indexer module as layer between engine and storage

	// index the block storage with each of the collection guarantee
	err = e.blocks.IndexBlockForCollections(block.Header.ID(), flow.GetIDs(block.Payload.Guarantees))
	if err != nil {
		return fmt.Errorf("could not index block for collections: %w", err)
	}

	// queue requesting each of the collections from the collection node
	e.requestCollections(block.Payload.Guarantees)

	return nil
}

func (e *Engine) trackFinalizedMetricForBlock(hb *model.Block) {
	// retrieve the block
	block, err := e.blocks.ByID(hb.BlockID)
	if err != nil {
		e.log.Warn().Err(err).Msg("could not track tx finalized metric: finalized block not found locally")
		return
	}

	// TODO lookup actual finalization time by looking at the block finalizing `b`
	now := time.Now().UTC()

	// mark all transactions as finalized
	// TODO: sample to reduce performance overhead
	for _, g := range block.Payload.Guarantees {
		l, err := e.collections.LightByID(g.CollectionID)
		if errors.Is(err, storage.ErrNotFound) {
			e.collectionsToMarkFinalized.Add(g.CollectionID, now)
			continue
		} else if err != nil {
			e.log.Warn().Err(err).Str("collection_id", g.CollectionID.String()).
				Msg("could not track tx finalized metric: finalized collection not found locally")
			continue
		}

		for _, t := range l.Transactions {
			e.transactionMetrics.TransactionFinalized(t, now)
		}
	}

	if ti, found := e.blocksToMarkExecuted.ByID(hb.BlockID); found {
		e.trackExecutedMetricForBlock(block, ti)
		e.blocksToMarkExecuted.Rem(hb.BlockID)
	}
}

func (e *Engine) handleExecutionReceipt(originID flow.Identifier, r *flow.ExecutionReceipt) error {
	// persist the execution receipt locally, storing will also index the receipt
	err := e.executionReceipts.Store(r)
	if err != nil {
		return fmt.Errorf("failed to store execution receipt: %w", err)
	}

	// TODO - handle possibly conflicting execution results in a proper way
	// for example, for unsealed blocks any ER is valid, but for sealed blocks
	// there should match the seal data.
	err = e.executionResults.ForceIndex(r.ExecutionResult.BlockID, r.ExecutionResult.ID())
	if err != nil {
		return fmt.Errorf("failed to index execution result: %w", err)
	}

	e.trackExecutedMetricForReceipt(r)
	return nil
}

func (e *Engine) trackExecutedMetricForReceipt(r *flow.ExecutionReceipt) {
	// TODO add actual execution time to execution receipt?
	now := time.Now().UTC()

	// retrieve the block
	b, err := e.blocks.ByID(r.ExecutionResult.BlockID)
	if errors.Is(err, storage.ErrNotFound) {
		e.blocksToMarkExecuted.Add(r.ExecutionResult.BlockID, now)
		return
	} else if err != nil {
		e.log.Warn().Err(err).Msg("could not track tx executed metric: executed block not found locally")
		return
	}
	e.trackExecutedMetricForBlock(b, now)
}

func (e *Engine) trackExecutedMetricForBlock(block *flow.Block, ti time.Time) {

	// mark all transactions as executed
	// TODO: sample to reduce performance overhead
	for _, g := range block.Payload.Guarantees {
		l, err := e.collections.LightByID(g.CollectionID)
		if errors.Is(err, storage.ErrNotFound) {
			e.collectionsToMarkExecuted.Add(g.CollectionID, ti)
			continue
		} else if err != nil {
			e.log.Warn().Err(err).Str("collection_id", g.CollectionID.String()).
				Msg("could not track tx executed metric: executed collection not found locally")
			continue
		}

		for _, t := range l.Transactions {
			e.transactionMetrics.TransactionExecuted(t, ti)
		}
	}
}

// handleCollection handles the response of the a collection request made earlier when a block was received
func (e *Engine) handleCollection(originID flow.Identifier, entity flow.Entity) error {

	// convert the entity to a strictly typed collection
	collection, ok := entity.(*flow.Collection)
	if !ok {
		return fmt.Errorf("invalid entity type (%T)", entity)
	}

	light := collection.Light()

	if ti, found := e.collectionsToMarkFinalized.ByID(light.ID()); found {
		for _, t := range light.Transactions {
			e.transactionMetrics.TransactionFinalized(t, ti)
		}
		e.collectionsToMarkFinalized.Rem(light.ID())
	}

	if ti, found := e.collectionsToMarkExecuted.ByID(light.ID()); found {
		for _, t := range light.Transactions {
			e.transactionMetrics.TransactionExecuted(t, ti)
		}
		e.collectionsToMarkExecuted.Rem(light.ID())
	}

	// FIX: we can't index guarantees here, as we might have more than one block
	// with the same collection as long as it is not finalized

	// store the light collection (collection minus the transaction body - those are stored separately)
	// and add transaction ids as index
	err := e.collections.StoreLightAndIndexByTransaction(&light)
	if err != nil {
		// ignore collection if already seen
		if errors.Is(err, storage.ErrAlreadyExists) {
			e.log.Debug().
				Hex("collection_id", logging.Entity(light)).
				Msg("collection is already seen")
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

func (e *Engine) OnCollection(originID flow.Identifier, entity flow.Entity) {
	err := e.handleCollection(originID, entity)
	if err != nil {
		e.log.Error().Err(err).Msg("could not handle collection")
		return
	}
}

// OnBlockIncorporated is a noop for this engine since access node is only dealing with finalized blocks
func (e *Engine) OnBlockIncorporated(*model.Block) {
}

// OnDoubleProposeDetected is a noop for this engine since access node is only dealing with finalized blocks
func (e *Engine) OnDoubleProposeDetected(*model.Block, *model.Block) {
}

// requestMissingCollections requests missing collections for all blocks in the local db storage once at startup
func (e *Engine) requestMissingCollections(ctx context.Context) error {

	var startHeight, endHeight uint64

	// get the height of the last block for which all collections were received
	lastFullHeight, err := e.blocks.GetLastFullBlockHeight()
	if err != nil {
		return fmt.Errorf("failed to complete requests for missing collections: %w", err)
	}

	// start from the next block
	startHeight = lastFullHeight + 1

	// end at the finalized block
	finalBlk, err := e.state.Final().Head()
	if err != nil {
		return err
	}
	endHeight = finalBlk.Height

	e.log.Info().
		Uint64("start_height", startHeight).
		Uint64("end_height", endHeight).
		Msg("starting collection catchup")

	// collect all missing collection ids in a map
	var missingCollMap = make(map[flow.Identifier]struct{})

	// iterate through the complete chain and request the missing collections
	for i := startHeight; i <= endHeight; i++ {

		// if deadline exceeded or someone cancelled the context
		if ctx.Err() != nil {
			return fmt.Errorf("failed to complete requests for missing collections: %w", ctx.Err())
		}

		missingColls, err := e.missingCollectionsAtHeight(i)
		if err != nil {
			return fmt.Errorf("failed to retreive missing collections by height %d during collection catchup: %w", i, err)
		}

		// request the missing collections
		e.requestCollections(missingColls)

		// add them to the missing collection id map to track later
		for _, cg := range missingColls {
			missingCollMap[cg.CollectionID] = struct{}{}
		}
	}

	// if no collections were found to be missing we are done.
	if len(missingCollMap) == 0 {
		// nothing more to do
		e.log.Info().Msg("no missing collections found")
		return nil
	}

	// the collection catchup needs to happen ASAP when the node starts up. Hence, force the requester to dispatch all request
	e.request.Force()

	// track progress of retrieving all the missing collections by polling the db periodically
	ticker := time.NewTicker(defaultCollectionCatchupDBPollInterval)
	defer ticker.Stop()

	// while there are still missing collections, keep polling
	for len(missingCollMap) > 0 {
		select {
		case <-ctx.Done():
			// context may have expired
			return fmt.Errorf("failed to complete collection retreival: %w", ctx.Err())
		case <-ticker.C:

			// log progress
			e.log.Info().
				Int("total_missing_collections", len(missingCollMap)).
				Msg("retrieving missing collections...")

			var foundColls []flow.Identifier
			// query db to find if collections are still missing
			for collId := range missingCollMap {
				found, err := e.lookupCollection(collId)
				if err != nil {
					return err
				}
				// if collection found in local db, remove it from missingColls later
				if found {
					foundColls = append(foundColls, collId)
				}
			}

			// update the missingColls list by removing collections that have now been received
			for _, c := range foundColls {
				delete(missingCollMap, c)
			}
		}
	}

	e.log.Info().Msg("collection catchup done")
	return nil
}

// updateLastFullBlockReceivedIndex keeps the FullBlockHeight index upto date and requests missing collections if
// the number of blocks missing collection have reached the defaultMissingCollsForBlkThreshold value.
// (The FullBlockHeight index indicates that block for which all collections have been received)
func (e *Engine) updateLastFullBlockReceivedIndex() {

	logError := func(err error) {
		e.log.Error().Err(err).Msg("failed to update the last full block height")
	}

	lastFullHeight, err := e.blocks.GetLastFullBlockHeight()
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			logError(err)
			return
		}
		// use the root height as the last full height
		header, err := e.state.Params().Root()
		if err != nil {
			logError(err)
			return
		}
		lastFullHeight = header.Height
	}
	e.log.Debug().Uint64("last_full_block_height", lastFullHeight).Msg("updating LastFullBlockReceived index...")

	finalBlk, err := e.state.Final().Head()
	if err != nil {
		logError(err)
		return
	}
	finalizedHeight := finalBlk.Height

	// track number of incomplete blocks
	incompleteBlksCnt := 0

	// track the latest contiguous full height
	latestFullHeight := lastFullHeight

	// collect all missing collections
	var allMissingColls []*flow.CollectionGuarantee

	// start from the next block till we either hit the finalized block or cross the max collection missing threshold
	for i := lastFullHeight + 1; i <= finalizedHeight && incompleteBlksCnt < defaultMissingCollsForBlkThreshold; i++ {

		// find missing collections for block at height i
		missingColls, err := e.missingCollectionsAtHeight(i)
		if err != nil {
			logError(err)
			return
		}

		// if there are missing collections
		if len(missingColls) > 0 {

			// increment number of incomplete blocks
			incompleteBlksCnt++

			// collect the missing collections for requesting later
			allMissingColls = append(allMissingColls, missingColls...)

			continue
		}

		// if there are no missing collections so far, advance the latestFullHeight pointer
		if incompleteBlksCnt == 0 {
			latestFullHeight = i
		}
	}

	// if more contiguous blocks are now complete, update db
	if latestFullHeight > lastFullHeight {
		err = e.blocks.UpdateLastFullBlockHeight(latestFullHeight)
		if err != nil {
			logError(err)
			return
		}
	}

	// additionally, if more than threshold blocks have missing collection OR collections are missing since defaultMissingCollsForHeightThreshold, re-request those collections
	if incompleteBlksCnt >= defaultMissingCollsForBlkThreshold || (finalizedHeight-lastFullHeight) > uint64(defaultMissingCollsForHeightThreshold) {
		// warn log since this should generally not happen
		e.log.Warn().
			Int("missing_collection_blk_count", incompleteBlksCnt).
			Int("threshold", defaultMissingCollsForBlkThreshold).
			Uint64("last_full_blk_height", latestFullHeight).
			Msg("re-requesting missing collections")
		e.requestCollections(allMissingColls)
	}

	e.log.Debug().Uint64("last_full_blk_height", latestFullHeight).Msg("updated LastFullBlockReceived index")
}

// missingCollectionsAtHeight returns all missing collection guarantees at a given height
func (e *Engine) missingCollectionsAtHeight(h uint64) ([]*flow.CollectionGuarantee, error) {
	blk, err := e.blocks.ByHeight(h)
	if err != nil {
		return nil, fmt.Errorf("failed to retreive block by height %d: %w", h, err)
	}

	var missingColls []*flow.CollectionGuarantee
	for _, guarantee := range blk.Payload.Guarantees {

		collID := guarantee.CollectionID
		found, err := e.lookupCollection(collID)
		if err != nil {
			return nil, err
		}
		if !found {
			missingColls = append(missingColls, guarantee)
		}
	}
	return missingColls, nil
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
	return false, fmt.Errorf("failed to retreive collection %s: %w", collId.String(), err)
}

// requestCollections registers collection requests with the requester engine
func (e *Engine) requestCollections(missingColls []*flow.CollectionGuarantee) {
	for _, cg := range missingColls {
		e.request.EntityByID(cg.ID(), filter.HasNodeID(cg.SignerIDs...))
	}
}
