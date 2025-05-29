package ingestion2

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type Engine struct {
	component.Component
	cm *component.ComponentManager

	log   zerolog.Logger
	state protocol.State // used to access the  protocol state

	blocks            storage.Blocks
	headers           storage.Headers
	executionReceipts storage.ExecutionReceipts
	executionResults  storage.ExecutionResults

	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter

	// Job queue
	finalizedBlockConsumer *jobqueue.ComponentConsumer
	// Notifier for queue consumer
	finalizedBlockNotifier engine.Notifier

	resultsForest *ResultsForest
	maxForestSize uint

	collectionExecutedMetric module.CollectionExecutedMetric

	latestPersistedSealedResult flow.Identifier
}

var _ hotstuff.FinalizationConsumer = (*Engine)(nil)

func New(log zerolog.Logger) *Engine {
	resultsForest := NewResultsForest(log)

	e := &Engine{
		log:           log.With().Str("component", "ingestion").Logger(),
		resultsForest: resultsForest,
	}

	cm := component.NewComponentManagerBuilder().
		AddWorker(e.runForest).
		AddWorker(e.runForestLoader).
		Build()

	e.cm = cm
	e.Component = cm

	return e
}

func (e *Engine) runForest(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	e.resultsForest.Start(ctx)

	select {
	case <-ctx.Done():
	case <-e.resultsForest.Ready():
		ready()
	}

	<-e.resultsForest.Done()
}

func (e *Engine) runForestLoader(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	latestPersistedResultID, err := e.loadLatestPersistedSealedResult()
	if err != nil {
		ctx.Throw(err)
		return
	}

	loader := NewForestLoader(e.resultsForest, latestPersistedResultID, e.maxForestSize)

	select {
	case <-ctx.Done():
		return
	case <-e.resultsForest.Ready():
	}

	ready()
	if err := loader.Run(ctx); err != nil {
		ctx.Throw(err)
		return
	}
}

func (e *Engine) loadLatestPersistedSealedResult() (flow.Identifier, error) {
	return flow.ZeroID, fmt.Errorf("not implemented")
}

// OnFinalizedBlock is called by the follower engine after a block has been finalized and the state has been updated.
// Receives block finalized events from the finalization distributor and forwards them to the finalizedBlockConsumer.
func (e *Engine) OnFinalizedBlock(*model.Block) {
	e.finalizedBlockNotifier.Notify()
}

// OnBlockIncorporated is called by the follower engine after a block has been certified and the state has been updated.
// Receives block incorporated events from the finalization distributor.
func (e *Engine) OnBlockIncorporated(hotstuffBlock *model.Block) {
	// Per specification of the `hotstuff.FinalizationConsumer` consumers of the `OnBlockIncorporated` notification must
	// be non-blocking. This code is run on the hotpath of consensus and should induce as little overhead as possible.
	//
	// The input is coming from the node-internal consensus follower, which is a trusted component. Hence, we don't
	// need to verify the inputs and queue them directly for processing by one of the engine's workers.

	// ToDO: queue incoming incorporated hotstuffBlock for processing in a dedicated pipeline
	// The thread picking up the hotstuffBlock would then convert it to `flow.block` and process it further
	//
	//  block, err := e.blocks.ByID(hotstuffBlock.BlockID)
	//  if err != nil {
	//	  return irrecoverable.NewExceptionf("received incorporated block %s from consensus follower, but failed to retrieve full block: %w", err)
	//   }
	//   err = e.processCertifiedBlock(block)
	//      ...
}

// processCertifiedBlock adds results from the certified block to the results forest.
// No errors are expected during normal operation.
func (e *Engine) processCertifiedBlock(block *flow.Block) error {
	for _, result := range block.Payload.Results {
		if err := e.resultsForest.AddResult(result, block.Header, false); err != nil {
			return fmt.Errorf("could not add result %s to forest: %w", result.ID(), err)
		}
	}
	return nil
}

// processFinalizedBlock handles an incoming finalized block.
// It processes the block, indexes it for further processing, and requests missing collections if necessary.
//
// Expected errors during normal operation:
//   - storage.ErrNotFound - if last full block height does not exist in the database.
//   - storage.ErrAlreadyExists - if the collection within block or an execution result ID already exists in the database.
//   - generic error in case of unexpected failure from the database layer, or failure
//     to decode an existing database value.
func (e *Engine) processFinalizedBlock(block *flow.Block) error {
	// index the block storage with each of the collection guarantee
	err := e.blocks.IndexBlockForCollections(block.Header.ID(), flow.GetIDs(block.Payload.Guarantees))
	if err != nil {
		return fmt.Errorf("could not index block for collections: %w", err)
	}

	// index sealed results and notify the results forest
	for _, seal := range block.Payload.Seals {
		if err := e.executionResults.Index(seal.BlockID, seal.ResultID); err != nil {
			return fmt.Errorf("could not index block for execution result (id: %s): %w", seal.ResultID, err)
		}

		if err := e.resultsForest.OnResultSealed(seal.ResultID); err != nil {
			return fmt.Errorf("could not notify results forest of newly sealed result (id: %s): %w", seal.ResultID, err)
		}
	}

	e.collectionExecutedMetric.BlockFinalized(block)

	return nil
}

// handleExecutionReceipt persists the execution receipt locally.
// Storing the execution receipt and updates the collection executed metric.
//
// No errors are expected during normal operation.
func (e *Engine) handleExecutionReceipt(receipt *flow.ExecutionReceipt) error {
	// persist the execution receipt locally, storing will also index the receipt
	err := e.executionReceipts.Store(receipt)
	if err != nil {
		return fmt.Errorf("failed to store execution receipt: %w", err)
	}

	e.collectionExecutedMetric.ExecutionReceiptReceived(receipt)
	return nil
}
