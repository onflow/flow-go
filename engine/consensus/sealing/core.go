// (c) 2021 Dapper Labs - ALL RIGHTS RESERVED

package sealing

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/onflow/flow-go/engine/consensus/approvals"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"sync/atomic"
	"time"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/rs/zerolog"
)

// DefaultRequiredApprovalsForSealConstruction is the default number of approvals required to construct a candidate seal
// for subsequent inclusion in block.
const DefaultRequiredApprovalsForSealConstruction = 0

// DefaultEmergencySealingThreshold is the default number of blocks which indicates that ER should be sealed using emergency
// sealing.
const DefaultEmergencySealingThreshold = 400

// DefaultEmergencySealingActive is a flag which indicates when emergency sealing is active, this is a temporary measure
// to make fire fighting easier while seal & verification is under development.
const DefaultEmergencySealingActive = false

type Options struct {
	emergencySealingActive               bool   // flag which indicates if emergency sealing is active or not. NOTE: this is temporary while sealing & verification is under development
	requiredApprovalsForSealConstruction uint   // min number of approvals required for constructing a candidate seal
	approvalRequestsThreshold            uint64 // threshold for re-requesting approvals: min height difference between the latest finalized block and the block incorporating a result
}

func DefaultOptions() Options {
	return Options{
		emergencySealingActive:               DefaultEmergencySealingActive,
		requiredApprovalsForSealConstruction: DefaultRequiredApprovalsForSealConstruction,
		approvalRequestsThreshold:            10,
	}
}

// Core is an implementation of ResultApprovalProcessor interface
// This struct is responsible for:
// 	- collecting approvals for execution results
// 	- processing multiple incorporated results
// 	- pre-validating approvals (if they are outdated or non-verifiable)
// 	- pruning already processed collectorTree
type Core struct {
	log                       zerolog.Logger                     // used to log relevant actions with context
	collectorTree             *approvals.AssignmentCollectorTree // levelled forest for assignment collectors
	approvalsCache            *approvals.LruCache                // in-memory cache of approvals that weren't verified
	atomicLastSealedHeight    uint64                             // atomic variable for last sealed block height
	atomicLastFinalizedHeight uint64                             // atomic variable for last finalized block height
	headers                   storage.Headers                    // used to access block headers in storage
	state                     protocol.State                     // used to access protocol state
	seals                     storage.Seals                      // used to get last sealed block
	requestTracker            *RequestTracker                    // used to keep track of number of approval requests, and blackout periods, by chunk
	pendingReceipts           mempool.PendingReceipts            // buffer for receipts where an ancestor result is missing, so they can't be connected to the sealed results
	metrics                   module.ConsensusMetrics            // used to track consensus metrics
	tracer                    module.Tracer                      // used to trace execution
	mempool                   module.MempoolMetrics              // used to track mempool size
	receiptsDB                storage.ExecutionReceipts          // to persist received execution receipts
	receiptValidator          module.ReceiptValidator            // used to validate receipts
	receipts                  mempool.ExecutionTree              // holds execution receipts; indexes them by height; can search all receipts derived from a given parent result
	options                   Options
}

func NewCore(
	log zerolog.Logger,
	tracer module.Tracer,
	mempool module.MempoolMetrics,
	conMetrics module.ConsensusMetrics,
	headers storage.Headers,
	state protocol.State,
	sealsDB storage.Seals,
	assigner module.ChunkAssigner,
	verifier module.Verifier,
	sealsMempool mempool.IncorporatedResultSeals,
	approvalConduit network.Conduit,
	receipts mempool.ExecutionTree,
	receiptsDB storage.ExecutionReceipts,
	receiptValidator module.ReceiptValidator,
	options Options,
) (*Core, error) {
	lastSealed, err := state.Sealed().Head()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve last sealed block: %w", err)
	}

	core := &Core{
		log:              log.With().Str("engine", "sealing.Core").Logger(),
		tracer:           tracer,
		mempool:          mempool,
		metrics:          conMetrics,
		approvalsCache:   approvals.NewApprovalsLRUCache(1000),
		headers:          headers,
		state:            state,
		seals:            sealsDB,
		options:          options,
		receiptsDB:       receiptsDB,
		receipts:         receipts,
		receiptValidator: receiptValidator,
		requestTracker:   NewRequestTracker(10, 30),
	}

	factoryMethod := func(result *flow.ExecutionResult) (*approvals.AssignmentCollector, error) {
		return approvals.NewAssignmentCollector(result, core.state, core.headers, assigner, sealsMempool, verifier,
			approvalConduit, core.requestTracker, options.requiredApprovalsForSealConstruction)
	}

	core.collectorTree = approvals.NewAssignmentCollectorTree(lastSealed, headers, factoryMethod)

	core.mempool.MempoolEntries(metrics.ResourceReceipt, core.receipts.Size())

	return core, nil
}

func (c *Core) lastSealedHeight() uint64 {
	return atomic.LoadUint64(&c.atomicLastSealedHeight)
}

func (c *Core) lastFinalizedHeight() uint64 {
	return atomic.LoadUint64(&c.atomicLastFinalizedHeight)
}

// processIncorporatedResult implements business logic for processing single incorporated result
// Returns:
// * engine.InvalidInputError - incorporated result is invalid
// * engine.UnverifiableInputError - result is unverifiable since referenced block cannot be found
// * engine.OutdatedInputError - result is outdated for instance block was already sealed
// * exception in case of any other error, usually this is not expected
// * nil - successfully processed incorporated result
func (c *Core) processIncorporatedResult(result *flow.IncorporatedResult) error {
	err := c.checkBlockOutdated(result.Result.BlockID)
	if err != nil {
		return fmt.Errorf("won't process outdated or unverifiable execution result %s: %w", result.Result.BlockID, err)
	}

	incorporatedBlock, err := c.headers.ByBlockID(result.IncorporatedBlockID)
	if err != nil {
		return fmt.Errorf("could not get block height for incorporated block %s: %w",
			result.IncorporatedBlockID, err)
	}
	incorporatedAtHeight := incorporatedBlock.Height

	lastFinalizedBlockHeight := c.lastFinalizedHeight()

	// check if we are dealing with finalized block or an orphan
	if incorporatedAtHeight <= lastFinalizedBlockHeight {
		finalized, err := c.headers.ByHeight(incorporatedAtHeight)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized block at height %d: %w", incorporatedAtHeight, err)
		}
		if finalized.ID() != result.IncorporatedBlockID {
			// it means that we got incorporated result for a block which doesn't extend our chain
			// and should be discarded from future processing
			return engine.NewOutdatedInputErrorf("won't process incorporated result from orphan block %s", result.IncorporatedBlockID)
		}
	}

	// in case block is not finalized we will create collector and start processing approvals
	// no checks for orphans can be made at this point
	// we expect that assignment collector will cleanup orphan IRs whenever new finalized block is processed

	lazyCollector, err := c.collectorTree.GetOrCreateCollector(result.Result)
	if err != nil {
		return fmt.Errorf("could not process incorporated result, cannot create collector: %w", err)
	}

	if !lazyCollector.Processable {
		return engine.NewOutdatedInputErrorf("collector for %s is marked as non processable", result.ID())
	}

	err = lazyCollector.Collector.ProcessIncorporatedResult(result)
	if err != nil {
		return fmt.Errorf("could not process incorporated result: %w", err)
	}

	// process pending approvals only if it's a new collector
	// pending approvals are those we haven't received its result yet,
	// once we received a result and created a new collector, we find the pending
	// approvals for this result, and process them
	// newIncorporatedResult should be true only for one goroutine even if multiple access this code at the same
	// time, ensuring that processing of pending approvals happens once for particular assignment
	if lazyCollector.Created {
		err = c.processPendingApprovals(lazyCollector.Collector)
		if err != nil {
			return fmt.Errorf("could not process cached approvals:  %w", err)
		}
	}

	return nil
}

func (c *Core) ProcessIncorporatedResult(result *flow.IncorporatedResult) error {
	err := c.processIncorporatedResult(result)

	// we expect that only engine.UnverifiableInputError,
	// engine.OutdatedInputError, engine.InvalidInputError are expected, otherwise it's an exception
	if engine.IsUnverifiableInputError(err) || engine.IsOutdatedInputError(err) || engine.IsInvalidInputError(err) {
		logger := c.log.Info()
		if engine.IsInvalidInputError(err) {
			logger = c.log.Error()
		}

		logger.Err(err).Msgf("could not process incorporated result %v", result.ID())
		return nil
	}

	return err
}

// checkBlockOutdated performs a sanity check if block is outdated
// Returns:
// * engine.UnverifiableInputError - sentinel error in case we haven't discovered requested blockID
// * engine.OutdatedInputError - sentinel error in case block is outdated
// * exception in case of unknown internal error
// * nil - block isn't sealed
func (c *Core) checkBlockOutdated(blockID flow.Identifier) error {
	block, err := c.headers.ByBlockID(blockID)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("failed to retrieve header for block %x: %w", blockID, err)
		}
		return engine.NewUnverifiableInputError("no header for block: %v", blockID)
	}

	// it's important to use atomic operation to make sure that we have correct ordering
	lastSealedHeight := c.lastSealedHeight()
	// drop approval, if it is for block whose height is lower or equal to already sealed height
	if lastSealedHeight >= block.Height {
		return engine.NewOutdatedInputErrorf("requested processing for already sealed block height")
	}

	return nil
}

func (c *Core) ProcessApproval(approval *flow.ResultApproval) error {
	startTime := time.Now()
	approvalSpan := c.tracer.StartSpan(approval.ID(), trace.CONMatchOnApproval)

	err := c.processApproval(approval)

	c.metrics.OnApprovalProcessingDuration(time.Since(startTime))
	approvalSpan.Finish()

	// we expect that only engine.UnverifiableInputError,
	// engine.OutdatedInputError, engine.InvalidInputError are expected, otherwise it's an exception
	if engine.IsUnverifiableInputError(err) || engine.IsOutdatedInputError(err) || engine.IsInvalidInputError(err) {
		logger := c.log.Info()
		if engine.IsInvalidInputError(err) {
			logger = c.log.Error()
		}

		logger.Err(err).
			Hex("approval_id", logging.Entity(approval)).
			Msgf("could not process result approval")

		return nil
	}
	marshalled, err := json.Marshal(approval)
	if err != nil {
		marshalled = []byte("json_marshalling_failed")
	}
	c.log.Error().Err(err).
		Hex("approval_id", logging.Entity(approval)).
		Str("approval", string(marshalled)).
		Msgf("unexpected error processing result approval")

	return fmt.Errorf("internal error processing result approval %x: %w", approval.ID(), err)

}

// processApproval implements business logic for processing single approval
// Returns:
// * engine.InvalidInputError - result approval is invalid
// * engine.UnverifiableInputError - result approval is unverifiable since referenced block cannot be found
// * engine.OutdatedInputError - result approval is outdated for instance block was already sealed
// * exception in case of any other error, usually this is not expected
// * nil - successfully processed result approval
func (c *Core) processApproval(approval *flow.ResultApproval) error {
	err := c.checkBlockOutdated(approval.Body.BlockID)
	if err != nil {
		return fmt.Errorf("won't process approval for oudated block (%x): %w", approval.Body.BlockID, err)
	}

	if collector, processable := c.collectorTree.GetCollector(approval.Body.ExecutionResultID); collector != nil {
		if !processable {
			return engine.NewOutdatedInputErrorf("collector for %s is marked as non processable", approval.Body.ExecutionResultID)
		}

		// if there is a collector it means that we have received execution result and we are ready
		// to process approvals
		err = collector.ProcessApproval(approval)
		if err != nil {
			return fmt.Errorf("could not process assignment: %w", err)
		}
	} else {
		// in case we haven't received execution result, cache it and process later.
		c.approvalsCache.Put(approval)
	}

	return nil
}

func (c *Core) checkEmergencySealing(lastSealedHeight, lastFinalizedHeight uint64) error {
	if !c.options.emergencySealingActive {
		return nil
	}

	emergencySealingHeight := lastSealedHeight + DefaultEmergencySealingThreshold

	// we are interested in all collectors that match condition:
	// lastSealedBlock + sealing.DefaultEmergencySealingThreshold < lastFinalizedHeight
	// in other words we should check for emergency sealing only if threshold was reached
	if emergencySealingHeight >= lastFinalizedHeight {
		return nil
	}

	delta := lastFinalizedHeight - emergencySealingHeight
	// if block is emergency sealable depends on it's incorporated block height
	// collectors tree stores collector by executed block height
	// we need to select multiple levels to find eligible collectors for emergency sealing
	for _, collector := range c.collectorTree.GetCollectorsByInterval(lastSealedHeight, lastSealedHeight+delta) {
		err := collector.CheckEmergencySealing(lastFinalizedHeight)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Core) processPendingApprovals(collector *approvals.AssignmentCollector) error {
	// filter cached approvals for concrete execution result
	for _, approval := range c.approvalsCache.TakeByResultID(collector.ResultID) {
		err := collector.ProcessApproval(approval)
		if err != nil {
			if engine.IsInvalidInputError(err) {
				c.log.Debug().
					Hex("result_id", collector.ResultID[:]).
					Err(err).
					Msgf("invalid approval with id %s", approval.ID())
			} else {
				return fmt.Errorf("could not process assignment: %w", err)
			}
		}
	}

	return nil
}

// ProcessReceipt processes a new execution receipt.
// Any error indicates an unexpected problem in the protocol logic. The node's
// internal state might be corrupted. Hence, returned errors should be treated as fatal.
// This function is viable only in phase 2 of sealing and verification where execution receipt
// can be retrieved from p2p network.
func (c *Core) ProcessReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {
	// When receiving a receipt, we might not be able to verify it if its previous result
	// is unknown.  In this case, instead of dropping it, we store it in the pending receipts
	// mempool, and process it later when its parent result has been received and processed.
	// Therefore, if a receipt is processed, we will check if it is the previous results of
	// some pending receipts and process them one after another.
	receiptID := receipt.ID()
	resultID := receipt.ExecutionResult.ID()

	processed, err := c.processReceipt(receipt)
	if err != nil {
		marshalled, encErr := json.Marshal(receipt)
		if encErr != nil {
			marshalled = []byte("json_marshalling_failed")
		}
		c.log.Error().Err(err).
			Hex("origin", logging.ID(originID)).
			Hex("receipt_id", receiptID[:]).
			Hex("result_id", resultID[:]).
			Str("receipt", string(marshalled)).
			Msg("internal error processing execution receipt")

		return fmt.Errorf("internal error processing execution receipt %x: %w", receipt.ID(), err)
	}

	if !processed {
		return nil
	}

	childReceipts := c.pendingReceipts.ByPreviousResultID(resultID)
	c.pendingReceipts.Rem(receipt.ID())

	for _, childReceipt := range childReceipts {
		// recursively processing the child receipts
		err := c.ProcessReceipt(childReceipt.ExecutorID, childReceipt)
		if err != nil {
			// we don't want to wrap the error with any info from its parent receipt,
			// because the error has nothing to do with its parent receipt.
			return err
		}
	}

	return nil
}

func (c *Core) ProcessFinalizedBlock(finalizedBlockID flow.Identifier) error {
	finalized, err := c.headers.ByBlockID(finalizedBlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve header for finalized block %s", finalizedBlockID)
	}

	// no need to process already finalized blocks
	if finalized.Height <= c.lastFinalizedHeight() {
		return nil
	}

	// it's important to use atomic operation to make sure that we have correct ordering
	atomic.StoreUint64(&c.atomicLastFinalizedHeight, finalized.Height)

	seal, err := c.seals.ByBlockID(finalizedBlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve seal for finalized block %s", finalizedBlockID)
	}
	lastSealed, err := c.headers.ByBlockID(seal.BlockID)
	if err != nil {
		c.log.Fatal().Err(err).Msgf("could not retrieve last sealed block %s", seal.BlockID)
	}

	// it's important to use atomic operation to make sure that we have correct ordering
	atomic.StoreUint64(&c.atomicLastSealedHeight, lastSealed.Height)

	// check if there are stale results qualified for emergency sealing
	err = c.checkEmergencySealing(lastSealed.Height, finalized.Height)
	if err != nil {
		return fmt.Errorf("could not check emergency sealing at block %v", finalizedBlockID)
	}

	// finalize forks to stop collecting approvals for orphan collectors
	c.collectorTree.FinalizeForkAtLevel(finalized, lastSealed)

	// as soon as we discover new sealed height, proceed with pruning collectors
	pruned, err := c.collectorTree.PruneUpToHeight(lastSealed.Height)
	if err != nil {
		return fmt.Errorf("could not prune collectorTree tree at block %v", finalizedBlockID)
	}

	// remove all pending items that we might have requested
	c.requestTracker.Remove(pruned...)

	// The receipts mempool is aware of the Execution Tree structure formed by the execution results.
	// It supports pruning by height: only results descending from the latest sealed and finalized
	// result are relevant. Hence, we can prune all results for blocks _below_ the latest block with
	// a finalized seal. Results of sufficient height for forks that conflict with the finalized fork
	// are retained in the mempool. However, such orphaned forks do not grow anymore and their
	// results will be progressively flushed out with increasing sealed-finalized height.
	err = c.receipts.PruneUpToHeight(lastSealed.Height)
	if err != nil {
		return fmt.Errorf("failed to clean receipts mempool: %w", err)
	}

	err = c.requestPendingApprovals(lastSealed.Height, finalized.Height)
	if err != nil {
		return fmt.Errorf("internal error while requesting pending approvals: %w", err)
	}

	return nil
}

// processReceipt checks validity of the given receipt and adds it to the node's validated information.
// Returns:
// * bool: true iff receipt is new (previously unknown), and its validity can be confirmed
// * error: any error indicates an unexpected problem in the protocol logic. The node's
//   internal state might be corrupted. Hence, returned errors should be treated as fatal.
func (c *Core) processReceipt(receipt *flow.ExecutionReceipt) (bool, error) {
	startTime := time.Now()
	receiptSpan := c.tracer.StartSpan(receipt.ID(), trace.CONMatchOnReceipt)
	defer func() {
		c.metrics.OnReceiptProcessingDuration(time.Since(startTime))
		receiptSpan.Finish()
	}()

	// setup logger to capture basic information about the receipt
	log := c.log.With().
		Hex("receipt_id", logging.Entity(receipt)).
		Hex("result_id", logging.Entity(receipt.ExecutionResult)).
		Hex("previous_result", receipt.ExecutionResult.PreviousResultID[:]).
		Hex("block_id", receipt.ExecutionResult.BlockID[:]).
		Hex("executor_id", receipt.ExecutorID[:]).
		Logger()
	initialState, finalState, err := getStartAndEndStates(receipt)
	if err != nil {
		if errors.Is(err, flow.NoChunksError) {
			log.Error().Err(err).Msg("discarding malformed receipt")
			return false, nil
		}
		return false, fmt.Errorf("internal problem retrieving start- and end-state commitment from receipt: %w", err)
	}
	log = log.With().
		Hex("initial_state", initialState[:]).
		Hex("final_state", finalState[:]).Logger()

	// if the receipt is for an unknown block, skip it. It will be re-requested
	// later by `requestPending` function.
	head, err := c.headers.ByBlockID(receipt.ExecutionResult.BlockID)
	if err != nil {
		log.Debug().Msg("discarding receipt for unknown block")
		return false, nil
	}

	log = log.With().
		Uint64("block_view", head.View).
		Uint64("block_height", head.Height).
		Logger()
	log.Debug().Msg("execution receipt received")

	lastSealeadHeight := c.lastSealedHeight()

	isSealed := head.Height <= lastSealeadHeight
	if isSealed {
		log.Debug().Msg("discarding receipt for already sealed and finalized block height")
		return false, nil
	}

	childSpan := c.tracer.StartSpanFromParent(receiptSpan, trace.CONMatchOnReceiptVal)
	err = c.receiptValidator.Validate(receipt)
	childSpan.Finish()

	if engine.IsUnverifiableInputError(err) {
		// If previous result is missing, we can't validate this receipt.
		// Although we will request its previous receipt(s),
		// we don't want to drop it now, because when the missing previous arrive
		// in a wrong order, they will still be dropped, and causing the catch up
		// to be inefficient.
		// Instead, we cache the receipt in case it arrives earlier than its
		// previous receipt.
		// For instance, given blocks A <- B <- C <- D <- E, if we receive their receipts
		// in the order of [E,C,D,B,A], then:
		// if we drop the missing previous receipts, then only A will be processed;
		// if we cache the missing previous receipts, then all of them will be processed, because
		// once A is processed, we will check if there is a child receipt pending,
		// if yes, then process it.
		c.pendingReceipts.Add(receipt)
		log.Info().Msg("receipt is cached because its previous result is missing")
		return false, nil
	}

	if err != nil {
		if engine.IsInvalidInputError(err) {
			log.Err(err).Msg("invalid execution receipt")
			return false, nil
		}
		return false, fmt.Errorf("failed to validate execution receipt: %w", err)
	}

	_, err = c.storeReceipt(receipt, head)
	if err != nil {
		return false, fmt.Errorf("failed to store receipt: %w", err)
	}

	// ATTENTION:
	//
	// In phase 2, we artificially create IncorporatedResults from incoming
	// receipts and set the IncorporatedBlockID to the result's block ID.
	//
	// In phase 3, the incorporated results mempool will be populated by the
	// finalizer when blocks are added to the chain, and the IncorporatedBlockID
	// will be the ID of the first block on its fork that contains a receipt
	// committing to this result.
	incorporatedResult := flow.NewIncorporatedResult(
		receipt.ExecutionResult.BlockID,
		&receipt.ExecutionResult,
	)

	err = c.ProcessIncorporatedResult(incorporatedResult)
	if err != nil {
		return false, fmt.Errorf("could not process receipt due to internal sealing error: %w", err)
	}

	log.Info().Msg("execution result processed and stored")

	return true, nil
}

// storeReceipt adds the receipt to the receipts mempool as well as to the persistent storage layer.
// Return values:
//  * bool to indicate whether the receipt is stored.
//  * exception in case something (unexpected) went wrong
func (c *Core) storeReceipt(receipt *flow.ExecutionReceipt, head *flow.Header) (bool, error) {
	added, err := c.receipts.AddReceipt(receipt, head)
	if err != nil {
		return false, fmt.Errorf("adding receipt (%x) to mempool failed: %w", receipt.ID(), err)
	}
	if !added {
		return false, nil
	}
	// TODO: we'd better wrap the `receipts` with the metrics method to avoid the metrics
	// getting out of sync
	c.mempool.MempoolEntries(metrics.ResourceReceipt, c.receipts.Size())

	// persist receipt in database. Even if the receipt is already in persistent storage,
	// we still need to process it, as it is not in the mempool. This can happen if the
	// mempool was wiped during a node crash.
	err = c.receiptsDB.Store(receipt) // internally de-duplicates
	if err != nil && !errors.Is(err, storage.ErrAlreadyExists) {
		return false, fmt.Errorf("could not persist receipt: %w", err)
	}
	return true, nil
}

// getStartAndEndStates returns the pair: (start state commitment; final state commitment)
// Error returns:
//  * NoChunksError: if there are no chunks, i.e. the ExecutionResult is malformed
//  * all other errors are unexpected and symptoms of node-internal problems
func getStartAndEndStates(receipt *flow.ExecutionReceipt) (initialState flow.StateCommitment, finalState flow.StateCommitment, err error) {
	initialState, err = receipt.ExecutionResult.InitialStateCommit()
	if err != nil {
		return initialState, finalState, fmt.Errorf("could not get commitment for initial state from receipt: %w", err)
	}
	finalState, err = receipt.ExecutionResult.FinalStateCommitment()
	if err != nil {
		return initialState, finalState, fmt.Errorf("could not get commitment for final state from receipt: %w", err)
	}
	return initialState, finalState, nil
}

// requestPendingApprovals requests approvals for chunks that haven't collected
// enough approvals. When the number of unsealed finalized blocks exceeds the
// threshold, we go through the entire mempool of incorporated-results, which
// haven't yet been sealed, and check which chunks need more approvals. We only
// request approvals if the block incorporating the result is below the
// threshold.
//
//                                   threshold
//                              |                   |
// ... <-- A <-- A+1 <- ... <-- D <-- D+1 <- ... -- F
//       sealed       maxHeightForRequesting      final
func (c *Core) requestPendingApprovals(lastSealedHeight, lastFinalizedHeight uint64) error {
	// skip requesting approvals if they are not required for sealing
	if c.options.requiredApprovalsForSealConstruction == 0 {
		return nil
	}

	if lastSealedHeight+c.options.approvalRequestsThreshold >= lastFinalizedHeight {
		return nil
	}

	// Reaching the following code implies:
	// 0 <= sealed.Height < final.Height - approvalRequestsThreshold
	// Hence, the following operation cannot underflow
	maxHeightForRequesting := lastFinalizedHeight - c.options.approvalRequestsThreshold

	for _, collector := range c.collectorTree.GetCollectorsByInterval(lastSealedHeight, lastSealedHeight+maxHeightForRequesting) {
		err := collector.RequestMissingApprovals(maxHeightForRequesting)
		if err != nil {
			return err
		}
	}

	return nil
}
