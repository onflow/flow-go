package finder

import (
	"errors"
	"fmt"
	"sync"

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
	unit               *engine.Unit
	log                zerolog.Logger
	me                 module.Local
	match              network.Engine
	receipts           mempool.Receipts // used to keep the receipts as mempool
	headerStorage      storage.Headers  // used to check block existence to improve performance
	receiptHandlerLock sync.Mutex       // used to avoid race condition in handling receipts
}

func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	match network.Engine,
	receipts mempool.Receipts,
	headerStorage storage.Headers,
) (*Engine, error) {
	e := &Engine{
		unit:          engine.NewUnit(),
		log:           log,
		me:            me,
		match:         match,
		receipts:      receipts,
		headerStorage: headerStorage,
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

// process receives and submits an event to the verifier engine for processing.
// It returns an error so the verifier engine will not propagate an event unless
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

func (e *Engine) handleExecutionReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {
	// avoids race condition in processing receipts
	e.receiptHandlerLock.Lock()
	defer e.receiptHandlerLock.Unlock()

	receiptID := receipt.ID()
	resultID := receipt.ExecutionResult.ID()

	log := e.log.With().
		Hex("origin_id", logging.ID(originID)).
		Hex("receipt_id", logging.ID(receiptID))

	if l.ingestedResultIDs.Has(resultID) {
		l.log.Debug().
			Hex("origin_id", logging.ID(originID)).
			Hex("receipt_id", logging.ID(receiptID)).
			Msg("execution receipt with already ingested result discarded")
		// discards the receipt if its result has already been ingested
		return nil
	}

	// stores the execution receipt in the mempool
	ok := l.receipts.Add(receipt)
	l.log.Debug().
		Hex("origin_id", logging.ID(originID)).
		Hex("receipt_id", logging.ID(receiptID)).
		Bool("mempool_insertion", ok).
		Msg("execution receipt added to mempool")

	// checks if the execution result has empty chunk
	if receipt.ExecutionResult.Chunks.Len() == 0 {
		// TODO potential attack on availability
		l.log.Debug().
			Hex("receipt_id", logging.ID(receiptID)).
			Hex("result_id", logging.ID(resultID)).
			Msg("could not ingest execution result with zero chunks")
		return nil
	}

	mychunks, err := l.myAssignedChunks(&receipt.ExecutionResult)
	// extracts list of chunks assigned to this Verification node
	if err != nil {
		l.log.Error().
			Err(err).
			Hex("result_id", logging.Entity(receipt.ExecutionResult)).
			Msg("could not fetch assigned chunks")
		return fmt.Errorf("could not perfrom chunk assignment on receipt: %w", err)
	}

	l.log.Debug().
		Hex("receipt_id", logging.ID(receiptID)).
		Hex("result_id", logging.ID(resultID)).
		Int("total_chunks", receipt.ExecutionResult.Chunks.Len()).
		Int("assigned_chunks", len(mychunks)).
		Msg("chunk assignment is done")

	for _, chunk := range mychunks {
		err := l.handleChunk(chunk, receipt)
		if err != nil {
			l.log.Err(err).
				Hex("receipt_id", logging.ID(receiptID)).
				Hex("result_id", logging.ID(resultID)).
				Hex("chunk_id", logging.ID(chunk.ID())).
				Msg("could not handle chunk")
		}
	}

	// checks pending chunks for this receipt
	l.checkPendingChunks([]*flow.ExecutionReceipt{receipt})

	return nil

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

	// block should be in the storage
	_, err := e.headerStorage.ByBlockID(block.BlockID)
	if errors.Is(err, storage.ErrNotFound) {
		e.log.Error().
			Hex("block_id", logging.ID(block.BlockID)).
			Msg("block is not available in storage")
		return
	}
	if err != nil {
		e.log.Error().
			Hex("block_id", logging.ID(block.BlockID)).
			Msg("could not check block availability in storage")
		return
	}
}

// To implement FinalizationConsumer
func (e *Engine) OnDoubleProposeDetected(*model.Block, *model.Block) {}
