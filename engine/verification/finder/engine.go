package finder

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// Engine receives receipts and passes them to the match engine if the block of the receipt is available
// and the verification node is staked at the block ID of the result part of receipt.
//
// A receipt follows a lifecycle in this engine:
// cached: the receipt is received but not handled yet.
// pending: the receipt is handled, but its corresponding block has not received at this node yet.
// discarded: the receipt's block has received, but this verification node has not staked at block of the receipt.
// ready: the receipt's block has received, and this verification node is staked for that block,
// hence receipt's result is  ready to be forwarded to match engine
// processed: the receipt's result has been forwarded to matching engine.
//
// This engine ensures that each (ready) result is passed to match engine only once.
// Hence, among concurrent ready receipts with shared result, only one instance of result is passed to match engine.
type Engine struct {
	unit    *engine.Unit
	log     zerolog.Logger
	metrics module.VerificationMetrics
	me      module.Local
	match   network.Engine
	state   protocol.State

	assigner         module.ChunkAssigner  // used to determine chunks this node needs to verify
	chunksQueue      storage.ChunksQueue   // to store chunks to be verified
	newChunkListener module.NewJobListener // to notify about a new chunk
	finishProcessing finishProcessing      // to report a block has been processed

	cachedReceipts           mempool.ReceiptDataPacks // used to keep incoming receipts before checking
	pendingReceipts          mempool.ReceiptDataPacks // used to keep the receipts pending for a block as mempool
	readyReceipts            mempool.ReceiptDataPacks // used to keep the receipts ready for process
	headerStorage            storage.Headers          // used to check block existence before verifying
	processedResultIDs       mempool.Identifiers      // used to keep track of the processed results
	discardedResultIDs       mempool.Identifiers      // used to keep track of discarded results while node was not staked for epoch
	blockIDsCache            mempool.Identifiers      // used as a cache to keep track of new finalized blocks
	pendingReceiptIDsByBlock mempool.IdentifierMap    // used as a mapping to keep track of receipts associated with a block
	receiptIDsByResult       mempool.IdentifierMap    // used as a mapping to keep track of receipts with the same result
	processInterval          time.Duration            // used to define intervals at which engine moves receipts through pipeline
	tracer                   module.Tracer
}

func New(
	log zerolog.Logger,
	metrics module.VerificationMetrics,
	tracer module.Tracer,
	net module.Network,
	me module.Local,
	state protocol.State,
	match network.Engine,
	cachedReceipts mempool.ReceiptDataPacks,
	pendingReceipts mempool.ReceiptDataPacks,
	readyReceipts mempool.ReceiptDataPacks,
	headerStorage storage.Headers,
	processedResultIDs mempool.Identifiers,
	discardedResultIDs mempool.Identifiers,
	pendingReceiptIDsByBlock mempool.IdentifierMap,
	receiptsIDsByResult mempool.IdentifierMap,
	blockIDsCache mempool.Identifiers,
	processInterval time.Duration,

	assigner module.ChunkAssigner,
	chunksQueue storage.ChunksQueue,
	newChunkListener module.NewJobListener,
) (*Engine, error) {
	e := &Engine{
		unit:                     engine.NewUnit(),
		log:                      log.With().Str("engine", "finder").Logger(),
		metrics:                  metrics,
		me:                       me,
		state:                    state,
		match:                    match,
		headerStorage:            headerStorage,
		cachedReceipts:           cachedReceipts,
		pendingReceipts:          pendingReceipts,
		readyReceipts:            readyReceipts,
		processedResultIDs:       processedResultIDs,
		discardedResultIDs:       discardedResultIDs,
		pendingReceiptIDsByBlock: pendingReceiptIDsByBlock,
		receiptIDsByResult:       receiptsIDsByResult,
		blockIDsCache:            blockIDsCache,
		processInterval:          processInterval,
		tracer:                   tracer,
		assigner:                 assigner,
		chunksQueue:              chunksQueue,
		newChunkListener:         newChunkListener,
	}

	_, err := net.Register(engine.ReceiveReceipts, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on execution receipt provider channel: %w", err)
	}
	return e, nil
}

func (e *Engine) withFinishProcessing(finishProcessing FinishProcessing) {
	e.finishProcessing = finishProcessing
}

// Ready returns a channel that is closed when the finder engine is ready.
func (e *Engine) Ready() <-chan struct{} {
	// Runs a periodic check to iterate over receipts and move them through the pipeline.
	// If onTimer takes longer than processInterval, the next call will be blocked until the previous
	// call has finished. That being said, there won't be two onTimer running in parallel.
	// See test cases for LaunchPeriodically
	e.unit.LaunchPeriodically(e.onTimer, e.processInterval, time.Duration(0))
	return e.unit.Ready()
}

// Done returns a channel that is closed when the verifier engine is done.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

// process receives and submits an event to the finder engine for processing.
// It returns an error so the finder engine will not propagate an event unless
// it is successfully processed by the engine.
// The origin ID indicates the node which originally submitted the event to
// the peer-to-peer network.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	var err error

	switch resource := event.(type) {
	case *flow.ExecutionReceipt:
		err = e.handleExecutionReceipt(originID, resource)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}

	if err != nil {
		// logs the error instead of returning that.
		// returning error would be projected at a higher level by network layer.
		// however, this is an engine-level error, and not network layer error.
		e.log.Debug().Err(err).Msg("engine could not process event successfully")
	}

	return nil
}

// handleExecutionReceipt receives an execution receipt and adds it to the cached receipt mempool.
func (e *Engine) handleExecutionReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {
	span, ok := e.tracer.GetSpan(receipt.ID(), trace.VERProcessExecutionReceipt)
	ctx := context.Background()
	if !ok {
		span = e.tracer.StartSpan(receipt.ID(), trace.VERProcessExecutionReceipt)
		span.SetTag("execution_receipt_id", receipt.ID())
		defer span.Finish()
	}
	ctx = opentracing.ContextWithSpan(ctx, span)
	childSpan, _ := e.tracer.StartSpanFromContext(ctx, trace.VERFindHandleExecutionReceipt)
	defer childSpan.Finish()

	receiptID := receipt.ID()
	resultID := receipt.ExecutionResult.ID()

	log := e.log.With().
		Hex("origin_id", logging.ID(originID)).
		Hex("receipt_id", logging.ID(receiptID)).
		Hex("result_id", logging.ID(resultID)).Logger()
	log.Info().
		Msg("execution receipt arrived")

	// monitoring: increases number of received execution receipts
	e.metrics.OnExecutionReceiptReceived()

	// caches receipt as a receipt data pack for further processing
	rdp := &verification.ReceiptDataPack{
		Receipt:  receipt,
		OriginID: originID,
		Ctx:      ctx,
	}

	ok = e.cachedReceipts.Add(rdp)
	if !ok {
		return fmt.Errorf("duplicate execution receipt. receipt_id: %x", logging.ID(receiptID))
	}
	log.Debug().
		Msg("execution receipt successfully handled")

	return nil
}

func (e *Engine) ProcessFinalizedBlock(block *flow.Block) {
	// the block consumer will pull as many finalized blocks as
	// it can consume to process
}

// isProcessable returns true if the block for execution result is available in the storage
// otherwise it returns false. In the current version, it checks solely against the block that
// contains the collection guarantee.
func (e *Engine) isProcessable(result *flow.ExecutionResult) bool {
	// checks existence of block that result points to
	_, err := e.headerStorage.ByBlockID(result.BlockID)
	return err == nil
}

// stakedAtBlockID checks whether this instance of verification node has staked at specified block ID.
// It returns true and nil if verification node has staked at specified block ID, and returns false, and nil otherwise.
// It returns false and error if it could not extract the stake of (verification node) node at the specified block.
func (e *Engine) stakedAtBlockID(blockID flow.Identifier) (bool, error) {
	// extracts identity of verification node at block height of result
	staked, err := protocol.IsNodeStakedAtBlockID(e.state, blockID, e.me.NodeID())
	if err != nil {
		return false, fmt.Errorf("could not check if node is staked at block %v: %w", blockID, err)
	}
	return staked, nil
}

// processResult submits the result to the match engine.
// originID is the identifier of the node that initially sends a receipt containing this result.
// It returns true and nil if the result is submitted successfully to the match engine.
// Otherwise, it returns false, and error if the result is not going successfully to the match engine. It returns false,
// and nil, if the result has already been processed.
func (e *Engine) processResult(ctx context.Context, originID flow.Identifier, result *flow.ExecutionResult) (bool, error) {
	span, _ := e.tracer.StartSpanFromContext(ctx, trace.VERFindProcessResult)
	defer span.Finish()

	resultID := result.ID()
	log := e.log.With().Hex("result_id", logging.ID(resultID)).Logger()
	if e.processedResultIDs.Has(resultID) {
		log.Debug().Msg("result already processed")
		return false, nil
	}
	if e.discardedResultIDs.Has(resultID) {
		e.log.Debug().Msg("drops handling already discarded result")
		return false, nil
	}

	err := e.match.Process(originID, result)
	if err != nil {
		return false, fmt.Errorf("submission error to match engine: %w", err)
	}

	log.Info().Msg("result submitted to match engine")

	// monitoring: increases number of execution results sent
	e.metrics.OnExecutionResultSent()

	return true, nil
}

// onResultProcessed is called whenever a result is processed completely and
// is passed to the match engine. It marks the result as processed, and removes
// all receipts with the same result from mempool.
func (e *Engine) onResultProcessed(ctx context.Context, resultID flow.Identifier) {
	span, _ := e.tracer.StartSpanFromContext(ctx, trace.VERFindOnResultProcessed)
	defer span.Finish()

	log := e.log.With().
		Hex("result_id", logging.ID(resultID)).
		Logger()
	// marks result as processed
	added := e.processedResultIDs.Add(resultID)
	if added {
		log.Debug().Msg("result marked as processed")
	}

	// extracts all receipt ids with this result
	receiptIDs, ok := e.receiptIDsByResult.Get(resultID)
	if !ok {
		log.Debug().Msg("could not retrieve receipt ids associated with this result")
	}

	// removes indices of all receipts associated with processed result
	removed := e.receiptIDsByResult.Rem(resultID)
	log.Debug().
		Bool("removed", removed).
		Msg("removes processed result id from receipt-ids-by-result")

	// drops all receipts with the same result
	for _, receiptID := range receiptIDs {
		// removes receipt from mempool
		removed := e.readyReceipts.Rem(receiptID)
		log.Debug().
			Bool("removed", removed).
			Hex("receipt_id", logging.ID(receiptID)).
			Msg("removes receipt with process result")
	}
}

// checkCachedReceipts iterates over the newly cached receipts and moves them
// further in the pipeline depending on whether they are processable or not.
func (e *Engine) checkCachedReceipts() {
	for _, rdp := range e.cachedReceipts.All() {
		// NOTE: this anonymous function is solely for sake of encapsulating a block of code
		// for tracing. To avoid closure, it should NOT encompass any goroutine involving rdp.
		func() {
			var span opentracing.Span
			span, _ = e.tracer.StartSpanFromContext(rdp.Ctx, trace.VERFindCheckCachedReceipts)
			defer span.Finish()

			receiptID := rdp.Receipt.ID()
			resultID := rdp.Receipt.ExecutionResult.ID()

			log := e.log.With().
				Hex("origin_id", logging.ID(rdp.OriginID)).
				Hex("receipt_id", logging.ID(receiptID)).
				Hex("block_id", logging.ID(rdp.Receipt.ExecutionResult.BlockID)).
				Hex("result_id", logging.ID(resultID)).Logger()

			// removes receipt from cache
			removed := e.cachedReceipts.Rem(receiptID)
			log.Debug().
				Bool("removed", removed).
				Msg("cached receipt has been removed")

			// checks if the result has already been processed or discarded
			if e.processedResultIDs.Has(resultID) {
				log.Debug().Msg("drops handling already processed result")
				return
			}
			if e.discardedResultIDs.Has(resultID) {
				log.Debug().Msg("drops handling already discarded result")
				return
			}

			ready := e.isProcessable(&rdp.Receipt.ExecutionResult)
			if !ready {
				// adds receipt to pending mempool
				added, err := e.addToPending(rdp)
				if err != nil {
					log.Debug().Err(err).Msg("could not add receipt to pending mempool")
					return
				}
				log.Debug().
					Bool("added_to_pending_mempool", added).
					Msg("cached receipt checked for adding to pending mempool")
				return
			}

			// adds receipt to ready mempool
			added, discarded, err := e.addToReady(rdp)
			if err != nil {
				log.Debug().Err(err).Msg("could not add receipt to ready mempool")
				return
			}
			log.Debug().
				Bool("added_to_discarded_mempool", discarded).
				Bool("added_to_ready_mempool", added).
				Msg("cached receipt checked for adding to ready mempool")
		}()
	}
}

// addToReady encapsulates the logic around adding a ReceiptDataPack to ready receipts mempool.
// The ReceiptDataPack is however discarded it if finder engine is not staked at its block id.
//
// When no errors occurred, the first return value indicates
// whether or not the receipt has been added to the ready mempool, and the second return value indicates
// whether or not the receipt has been discarded due to the node being unstaked at the receipt block ID.
func (e *Engine) addToReady(receiptDataPack *verification.ReceiptDataPack) (bool, bool, error) {
	receiptID := receiptDataPack.Receipt.ID()
	resultID := receiptDataPack.Receipt.ExecutionResult.ID()
	blockID := receiptDataPack.Receipt.ExecutionResult.BlockID

	// checks whether verification node is staked at snapshot of this result's block.
	ok, err := e.stakedAtBlockID(blockID)
	if err != nil {
		return false, false, fmt.Errorf("could not verify stake of verification node for result: %w", err)
	}

	if !ok {
		discarded := e.discardedResultIDs.Add(resultID)
		return false, discarded, nil
	}

	// adds the receipt to the ready mempool
	ok = e.readyReceipts.Add(receiptDataPack)
	if !ok {
		return false, false, nil
	}

	// records the execution receipt id based on its result id
	err = e.receiptIDsByResult.Append(resultID, receiptID)
	if err != nil {
		return false, false, nil
	}

	return true, false, nil
}

// addToPending encapsulates the logic around adding a ReceiptDataPack to pending receipts mempool.
//
// When no errors occurred, the first return value indicates
// whether or not the receipt has been added to the pending mempool.
func (e *Engine) addToPending(receiptDataPack *verification.ReceiptDataPack) (bool, error) {
	receiptID := receiptDataPack.Receipt.ID()
	resultID := receiptDataPack.Receipt.ExecutionResult.ID()
	blockID := receiptDataPack.Receipt.ExecutionResult.BlockID

	ok := e.pendingReceipts.Add(receiptDataPack)
	if !ok {
		return false, nil
	}

	// marks receipt pending for its block ID
	err := e.pendingReceiptIDsByBlock.Append(blockID, receiptID)
	if err != nil {
		return false, fmt.Errorf("could not append receipt to receipt-ids-by-block mempool: %w", err)
	}

	// records the execution receipt id based on its result id
	err = e.receiptIDsByResult.Append(resultID, receiptID)
	if err != nil {
		return false, fmt.Errorf("could not append receipt to receipt-ids-by-result mempool: %w", err)
	}

	return true, nil
}

// pendingToReady receives a list of receipt identifiers and moves all their corresponding receipts
// from pending to ready mempools.
// blockID is the block identifier that all receipts are pointing to.
func (e *Engine) pendingToReady(receiptIDs flow.IdentifierList, blockID flow.Identifier) {
	for _, receiptID := range receiptIDs {
		// retrieves receipt from pending mempool
		rdp, ok := e.pendingReceipts.Get(receiptID)
		log := e.log.With().
			Hex("block_id", logging.ID(blockID)).
			Hex("receipt_id", logging.ID(receiptID)).
			Logger()
		if !ok {
			log.Debug().Msg("could not retrieve receipt from pending receipts mempool")
			continue
		}

		resultID := rdp.Receipt.ExecutionResult.ID()
		log = log.With().
			Hex("result_id", logging.ID(resultID)).
			Logger()

		// NOTE: this anonymous function is solely for sake of encapsulating a block of code
		// for opentracing. To avoid closure, it should NOT encompass any goroutine involving rdp.
		func() {
			var span opentracing.Span
			span, _ = e.tracer.StartSpanFromContext(rdp.Ctx, trace.VERFindCheckPendingReceipts)
			defer span.Finish()

			// moves receipt from pending to ready mempool
			removed := e.pendingReceipts.Rem(receiptID)
			log.Debug().
				Bool("removed", removed).
				Msg("removes receipt from pending receipts")

			added := e.readyReceipts.Add(rdp)
			log.Debug().
				Bool("added", added).
				Msg("adds receipt to ready receipts")
		}()
	}
}

// discardReceiptsFromPending receives a list of receipt ids, and removes
// all receipts from the pending receipts mempool and marks their execution result as discarded.
// blockID is the block identifier that all receipts are pointing to.
//
// finder engine discards a receipt if it is not staked at block id of that receipt.
func (e *Engine) discardReceiptsFromPending(receiptIDs flow.IdentifierList, blockID flow.Identifier) {
	for _, receiptID := range receiptIDs {
		log := e.log.With().
			Hex("block_id", logging.ID(blockID)).
			Hex("receipt_id", logging.ID(receiptID)).
			Logger()
		// retrieves receipt from pending mempool
		rdp, ok := e.pendingReceipts.Get(receiptID)
		if !ok {
			log.Debug().Msg("could not retrieve receipt from pending receipts mempool")
			continue
		}

		resultID := rdp.Receipt.ExecutionResult.ID()
		log = log.With().
			Hex("result_id", logging.ID(resultID)).
			Logger()

		// NOTE: this anonymous function is solely for sake of encapsulating a block of code
		// for opentracing. To avoid closure, it should NOT encompass any goroutine involving rdp.
		func() {
			var span opentracing.Span
			span, _ = e.tracer.StartSpanFromContext(rdp.Ctx, trace.VERFindCheckPendingReceipts)
			defer span.Finish()

			// marks result id of receipt as discarded.
			added := e.discardedResultIDs.Add(resultID)
			log.Debug().
				Bool("added_to_discard_pool", added).
				Msg("execution result marks discarded")

			// removes receipt from pending receipt
			removed := e.pendingReceipts.Rem(receiptID)
			log.Debug().
				Bool("removed", removed).
				Msg("removes receipt from pending receipts")
		}()
	}
}

// checkReceipts iterates over the new cached finalized blocks. It moves
// their corresponding receipt from pending to ready mempools.
func (e *Engine) checkPendingReceipts() {
	for _, blockID := range e.blockIDsCache.All() {
		// removes blockID from new blocks mempool
		removed := e.blockIDsCache.Rem(blockID)
		log := e.log.With().
			Hex("block_id", logging.ID(blockID)).
			Logger()

		log.Debug().
			Bool("removed", removed).
			Msg("removes block id from cached block ids")

		// retrieves all receipts that are pending for this block
		receiptIDs, ok := e.pendingReceiptIDsByBlock.Get(blockID)
		if !ok {
			// no pending receipt for this block
			log.Debug().Msg("no pending receipt for block")
			continue
		}
		log.Debug().
			Int("receipt_num", len(receiptIDs)).
			Msg("retrieved receipt ids pending for block")

		// removes list of receipt ids for this block
		removed = e.pendingReceiptIDsByBlock.Rem(blockID)
		log.Debug().
			Bool("removed", removed).
			Msg("removes all receipt ids pending for block")

		// checks whether verification node is staked at snapshot of this block id/
		ok, err := e.stakedAtBlockID(blockID)
		if err != nil {
			e.log.Debug().
				Err(err).
				Msg("could verify stake of verification node for result")
			continue
		}

		if !ok {
			// node is not staked at block id
			// discards all pending receipts for this block id.
			e.discardReceiptsFromPending(receiptIDs, blockID)
			continue
		}

		// moves receipts from pending to ready
		e.pendingToReady(receiptIDs, blockID)
	}
}

// checkReadyReceipts iterates over receipts ready for process and processes them.
func (e *Engine) checkReadyReceipts() {
	for _, rdp := range e.readyReceipts.All() {
		// NOTE: this anonymous function is solely for sake of encapsulating a block of code
		// for tracing. To avoid closure, it should NOT encompass any goroutine involving rdp.
		func() {
			span, ctx := e.tracer.StartSpanFromContext(rdp.Ctx, trace.VERFindCheckReadyReceipts)
			defer span.Finish()

			receiptID := rdp.Receipt.ID()
			resultID := rdp.Receipt.ExecutionResult.ID()

			ok, err := e.processResult(ctx, rdp.OriginID, &rdp.Receipt.ExecutionResult)
			if err != nil {
				e.log.Error().
					Err(err).
					Hex("receipt_id", logging.ID(receiptID)).
					Hex("result_id", logging.ID(resultID)).
					Msg("could not process result")
				return
			}

			if !ok {
				// result has already been processed, no cleanup is needed
				return
			}

			// performs clean up
			e.onResultProcessed(ctx, resultID)

			e.log.Debug().
				Hex("receipt_id", logging.ID(receiptID)).
				Hex("result_id", logging.ID(resultID)).
				Msg("result processed successfully")
		}()
	}
}

// onTimer is called periodically by the unit module of Finder engine.
// It encapsulates the set of handlers should be executed periodically in order.
func (e *Engine) onTimer() {
	wg := &sync.WaitGroup{}

	wg.Add(3)

	// moves receipts from cache to either ready or pending mempools
	go func() {
		e.checkCachedReceipts()
		wg.Done()
	}()

	// moves pending receipt to ready mempool
	go func() {
		e.checkPendingReceipts()
		wg.Done()
	}()

	// processes ready receipts
	go func() {
		e.checkReadyReceipts()
		wg.Done()
	}()

	wg.Wait()
}
