package finder

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

type Engine struct {
	unit            *engine.Unit
	log             zerolog.Logger
	me              module.Local
	match           network.Engine
	receipts        mempool.Receipts    // used to keep the receipts as mempool
	headerStorage   storage.Headers     // used to check block existence before verifying
	processedResult mempool.Identifiers // used to keep track of the processed results
}

func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	match network.Engine,
	receipts mempool.Receipts,
	headerStorage storage.Headers,
	processedResults mempool.Identifiers,
) (*Engine, error) {
	e := &Engine{
		unit:            engine.NewUnit(),
		log:             log.With().Str("engine", "finder").Logger(),
		me:              me,
		match:           match,
		headerStorage:   headerStorage,
		receipts:        receipts,
		processedResult: processedResults,
	}

	_, err := net.Register(engine.ExecutionReceiptProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on execution receipt provider channel: %w", err)
	}
	return e, nil
}

// Ready returns a channel that is closed when the verifier engine is ready.
func (e *Engine) Ready() <-chan struct{} {
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
			e.log.Error().Err(err).Msg("could not process submitted event")
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

// handleExecutionReceipt receives an execution receipt and adds it to receipts mempool if all of following
// conditions are satisfied:
// - It has not yet been added to the mempool
func (e *Engine) handleExecutionReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {
	receiptID := receipt.ID()
	resultID := receipt.ExecutionResult.ID()

	log := e.log.With().
		Str("engine", "finder").
		Hex("origin_id", logging.ID(originID)).
		Hex("receipt_id", logging.ID(receiptID)).
		Hex("result_id", logging.ID(resultID)).Logger()
	log.Info().Msg("execution receipt arrived")

	// checks if the result has already been handled
	if e.processedResult.Has(resultID) {
		log.Debug().Msg("drops handling already processed result")
		return nil
	}

	// adds the execution receipt in the mempool
	added := e.receipts.Add(receipt)
	if !added {
		log.Debug().Msg("drops adding duplicate receipt")
		return nil
	}

	log.Info().Msg("execution receipt successfully handled")

	// checks receipt being processable
	e.checkReceipts([]*flow.ExecutionReceipt{receipt})

	return nil
}

// To implement FinalizationConsumer
func (e *Engine) OnBlockIncorporated(*model.Block) {

}

// OnFinalizedBlock is part of implementing FinalizationConsumer interface
//
// OnFinalizedBlock notifications are produced by the Finalization Logic whenever
// a block has been finalized. They are emitted in the order the blocks are finalized.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
func (e *Engine) OnFinalizedBlock(block *model.Block) {
	// TODO only receipts referencing block should be checked #4028
	e.checkReceipts(e.receipts.All())
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
func (e *Engine) processResult(result *flow.ExecutionResult) error {
	if e.processedResult.Has(result.ID()) {
		return fmt.Errorf("result already processed")
	}
	err := e.match.ProcessLocal(result)
	if err != nil {
		return fmt.Errorf("submission error to match engine: %w", err)
	}

	return nil
}

// onResultProcessed is called whenever a result is processed completely and
// is passed to the match engine. It marks the result as processed, and removes
// all receipts with the same result from mempool.
func (e *Engine) onResultProcessed(resultID flow.Identifier) {
	log := e.log.With().
		Hex("result_id", logging.ID(resultID)).
		Logger()

	// marks result as processed
	added := e.processedResult.Add(resultID)
	if added {
		log.Debug().Msg("result marked as processed")
	}

	// drops all receipts with the same result
	for _, receipt := range e.receipts.All() {
		if receipt.ExecutionResult.ID() == resultID {
			receiptID := receipt.ID()
			removed := e.receipts.Rem(receiptID)
			if removed {
				log.Debug().
					Hex("receipt_id", logging.ID(receiptID)).
					Msg("processed result cleaned up")
			}
		}
	}
}

// checkReceipts receives a set of receipts and evaluates each of them
// against being processable. If a receipt is processable, it gets processed.
func (e *Engine) checkReceipts(receipts []*flow.ExecutionReceipt) {
	e.unit.Lock()
	defer e.unit.Unlock()

	for _, receipt := range receipts {
		if e.isProcessable(&receipt.ExecutionResult) {
			// checks if result is ready to process
			err := e.processResult(&receipt.ExecutionResult)
			if err != nil {
				e.log.Error().
					Err(err).
					Hex("receipt_id", logging.Entity(receipt)).
					Hex("result_id", logging.Entity(receipt.ExecutionResult)).
					Msg("could not process result")
				continue
			}

			// performs clean up
			e.onResultProcessed(receipt.ExecutionResult.ID())
		}
	}
}
