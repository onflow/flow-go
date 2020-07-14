package finder

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

type Engine struct {
	unit                     *engine.Unit
	log                      zerolog.Logger
	metrics                  module.VerificationMetrics
	me                       module.Local
	match                    network.Engine
	cachedReceipts           mempool.ReceiptDataPacks // used to keep incoming receipts before checking
	pendingReceipts          mempool.ReceiptDataPacks // used to keep the receipts pending for a block as mempool
	readyReceipts            mempool.ReceiptDataPacks // used to keep the receipts ready for process
	headerStorage            storage.Headers          // used to check block existence before verifying
	processedResultIDs       mempool.Identifiers      // used to keep track of the processed results
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
	match network.Engine,
	cachedReceipts mempool.ReceiptDataPacks,
	pendingReceipts mempool.ReceiptDataPacks,
	readyReceipts mempool.ReceiptDataPacks,
	headerStorage storage.Headers,
	processedResultIDs mempool.Identifiers,
	pendingReceiptIDsByBlock mempool.IdentifierMap,
	receiptsIDsByResult mempool.IdentifierMap,
	blockIDsCache mempool.Identifiers,
	processInterval time.Duration,
) (*Engine, error) {
	e := &Engine{
		unit:                     engine.NewUnit(),
		log:                      log.With().Str("engine", "finder").Logger(),
		metrics:                  metrics,
		me:                       me,
		match:                    match,
		headerStorage:            headerStorage,
		cachedReceipts:           cachedReceipts,
		pendingReceipts:          pendingReceipts,
		readyReceipts:            readyReceipts,
		processedResultIDs:       processedResultIDs,
		pendingReceiptIDsByBlock: pendingReceiptIDsByBlock,
		receiptIDsByResult:       receiptsIDsByResult,
		blockIDsCache:            blockIDsCache,
		processInterval:          processInterval,
		tracer:                   tracer,
	}

	_, err := net.Register(engine.ExecutionReceiptProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on execution receipt provider channel: %w", err)
	}
	return e, nil
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
	switch resource := event.(type) {
	case *flow.ExecutionReceipt:
		return e.handleExecutionReceipt(originID, resource)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
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
		Str("engine", "finder").
		Hex("origin_id", logging.ID(originID)).
		Hex("receipt_id", logging.ID(receiptID)).
		Hex("result_id", logging.ID(resultID)).Logger()
	log.Info().Msg("execution receipt arrived")

	// monitoring: increases number of received execution receipts
	e.metrics.OnExecutionReceiptReceived()

	// caches receipt as a receipt data pack for further processing
	rdp := &verification.ReceiptDataPack{
		Receipt:  receipt,
		OriginID: originID,
		Ctx:      ctx,
	}

	ok = e.cachedReceipts.Add(rdp)
	log.Info().
		Bool("cached", ok).
		Msg("execution receipt successfully handled")

	return nil
}

// To implement FinalizationConsumer
func (e *Engine) OnBlockIncorporated(*model.Block) {

}

// OnFinalizedBlock is part of implementing FinalizationConsumer interface
// On receiving a block, it caches the block ID to be checked in the next onTimer loop.
//
// OnFinalizedBlock notifications are produced by the Finalization Logic whenever
// a block has been finalized. They are emitted in the order the blocks are finalized.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
func (e *Engine) OnFinalizedBlock(block *model.Block) {
	ok := e.blockIDsCache.Add(block.BlockID)
	e.log.Debug().
		Bool("added_new_blocks", ok).
		Hex("block_id", logging.ID(block.BlockID)).
		Msg("new finalized block received")
}

// To implement FinalizationConsumer
func (e *Engine) OnDoubleProposeDetected(*model.Block, *model.Block) {}

// isProcessable returns true if the block for execution result is available in the storage
// otherwise it returns false. In the current version, it checks solely against the block that
// contains the collection guarantee.
func (e *Engine) isProcessable(result *flow.ExecutionResult) bool {
	// checks existence of block that result points to
	_, err := e.headerStorage.ByBlockID(result.BlockID)
	return err == nil
}

// processResult submits the result to the match engine.
// originID is the identifier of the node that initially sends a receipt containing this result.
func (e *Engine) processResult(ctx context.Context, originID flow.Identifier, result *flow.ExecutionResult) error {
	span, _ := e.tracer.StartSpanFromContext(ctx, trace.VERFindProcessResult)
	defer span.Finish()

	resultID := result.ID()
	if e.processedResultIDs.Has(resultID) {
		e.log.Debug().
			Hex("result_id", logging.ID(resultID)).
			Msg("result already processed")
		return nil
	}
	err := e.match.Process(originID, result)
	if err != nil {
		return fmt.Errorf("submission error to match engine: %w", err)
	}

	e.log.Debug().
		Hex("result_id", logging.ID(resultID)).
		Msg("result submitted to match engine")

	// monitoring: increases number of execution results sent
	e.metrics.OnExecutionResultSent()

	return nil
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
	ok = e.receiptIDsByResult.Rem(resultID)
	if !ok {
		log.Debug().Msg("could not remove processed result from receipt-ids-by-result")
	}

	// drops all receipts with the same result
	for _, receiptID := range receiptIDs {
		// removes receipt from mempool
		removed := e.readyReceipts.Rem(receiptID)
		if removed {
			log.Debug().
				Hex("receipt_id", logging.ID(receiptID)).
				Msg("receipt with processed result cleaned up")
		}
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
				Str("engine", "finder").
				Hex("origin_id", logging.ID(rdp.OriginID)).
				Hex("receipt_id", logging.ID(receiptID)).
				Hex("result_id", logging.ID(resultID)).Logger()

			// removes receipt from cache
			ok := e.cachedReceipts.Rem(receiptID)
			if !ok {
				log.Debug().Msg("cached receipt has been removed")
				return
			}

			// checks if the result has already been handled
			if e.processedResultIDs.Has(resultID) {
				log.Debug().Msg("drops handling already processed result")
				return
			}

			// adds receipt to pending or ready mempools depending on its processable status
			ready := e.isProcessable(&rdp.Receipt.ExecutionResult)
			if ready {
				// block for the receipt is available,
				// receipt is ready for process
				ok := e.readyReceipts.Add(rdp)
				if !ok {
					log.Debug().Msg("drops adding duplicate receipt to ready mempool")
					return
				}
			} else {
				// block for the receipt is not available
				// receipt moves to pending mempool
				ok := e.pendingReceipts.Add(rdp)
				if !ok {
					log.Debug().Msg("drops adding duplicate receipt to pending mempool")
					return
				}

				// marks receipt pending for its block ID
				_, err := e.pendingReceiptIDsByBlock.Append(rdp.Receipt.ExecutionResult.BlockID, receiptID)
				if err != nil {
					e.log.Error().
						Err(err).
						Hex("block_id", logging.ID(rdp.Receipt.ExecutionResult.BlockID)).
						Hex("receipt_id", logging.ID(receiptID)).
						Msg("could not append receipt to receipt-ids-by-block mempool")
				}
			}

			// records the execution receipt id based on its result id
			_, err := e.receiptIDsByResult.Append(resultID, receiptID)
			if err != nil {
				log.Debug().Err(err).Msg("could not add receipt id to receipt-ids-by-result mempool")
			}

			log.Info().
				Bool("ready", ready).
				Msg("cached execution receipt moved to proper mempool")
		}()
	}
}

// checkReceipts iterates over the new cached finalized blocks. It moves
// their corresponding receipt from pending to ready mempools.
func (e *Engine) checkPendingReceipts() {
	for _, blockID := range e.blockIDsCache.All() {
		// removes blockID from new blocks mempool
		ok := e.blockIDsCache.Rem(blockID)
		if !ok {
			e.log.Debug().
				Hex("block_id", logging.ID(blockID)).
				Msg("could not remove block ID from cache")
		}

		// retrieves all receipts that are pending for this block
		receiptIDs, ok := e.pendingReceiptIDsByBlock.Get(blockID)
		if !ok {
			// no pending receipt for this block
			continue
		}
		// removes list of receipt ids for this block
		ok = e.pendingReceiptIDsByBlock.Rem(blockID)
		if !ok {
			e.log.Debug().
				Hex("block_id", logging.ID(blockID)).
				Msg("could not remove receipt id from receipt-ids-by-block mempool for block")
		}

		// moves receipts from pending to ready
		for _, receiptID := range receiptIDs {
			// retrieves receipt from pending mempool
			rdp, ok := e.pendingReceipts.Get(receiptID)
			if !ok {
				e.log.Debug().
					Hex("receipt_id", logging.ID(receiptID)).
					Msg("could not retrieve receipt from pending receipts mempool")
				continue
			}

			// NOTE: this anonymous function is solely for sake of encapsulating a block of code
			// for tracing. To avoid closure, it should NOT encompass any goroutine involving rdp.
			func() {
				var span opentracing.Span
				span, _ = e.tracer.StartSpanFromContext(rdp.Ctx, trace.VERFindCheckPendingReceipts)
				defer span.Finish()

				// removes receipt from pending mempool
				ok = e.pendingReceipts.Rem(receiptID)
				if !ok {
					e.log.Debug().
						Hex("receipt_id", logging.ID(receiptID)).
						Msg("could not remove receipt from pending receipts mempool")
					return
				}

				// adds receipt to ready mempoool
				ok = e.readyReceipts.Add(rdp)
				if !ok {
					e.log.Debug().
						Hex("receipt_id", logging.ID(receiptID)).
						Msg("could not add receipt to ready mempool")
					return
				}

				e.log.Debug().
					Hex("receipt_id", logging.ID(receiptID)).
					Hex("block_id", logging.ID(blockID)).
					Msg("pending receipt moved to ready successfully")
			}()
		}
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

			err := e.processResult(ctx, rdp.OriginID, &rdp.Receipt.ExecutionResult)
			if err != nil {
				e.log.Error().
					Err(err).
					Hex("receipt_id", logging.ID(receiptID)).
					Hex("result_id", logging.ID(resultID)).
					Msg("could not process result")
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
