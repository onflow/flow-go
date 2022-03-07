package matching

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/opentracing/opentracing-go/log"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// Config is a structure of values that configure behavior of matching engine
type Config struct {
	SealingThreshold    uint // threshold between sealed and finalized blocks
	MaxResultsToRequest uint // maximum number of receipts to request
}

func DefaultConfig() Config {
	return Config{
		SealingThreshold:    10,
		MaxResultsToRequest: 20,
	}
}

// Core represents the matching business logic, used to process receipts received from
// p2p network. Performs processing of pending receipts, storing of receipts and re-requesting
// missing execution receipts.
type Core struct {
	log              zerolog.Logger                  // used to log relevant actions with context
	tracer           module.Tracer                   // used to trace execution
	metrics          module.ConsensusMetrics         // used to track consensus metrics
	mempool          module.MempoolMetrics           // used to track mempool size
	state            protocol.State                  // used to access the  protocol state
	headersDB        storage.Headers                 // used to check sealed headers
	receiptsDB       storage.ExecutionReceipts       // to persist received execution receipts
	receipts         mempool.ExecutionTree           // holds execution receipts; indexes them by height; can search all receipts derived from a given parent result
	pendingReceipts  mempool.PendingReceipts         // buffer for receipts where an ancestor result is missing, so they can't be connected to the sealed results
	seals            mempool.IncorporatedResultSeals // holds candidate seals for incorporated results that have acquired sufficient approvals; candidate seals are constructed  without consideration of the sealability of parent results
	receiptValidator module.ReceiptValidator         // used to validate receipts
	receiptRequester module.Requester                // used to request missing execution receipts by block ID
	config           Config                          // config for matching core
}

func NewCore(
	log zerolog.Logger,
	tracer module.Tracer,
	metrics module.ConsensusMetrics,
	mempool module.MempoolMetrics,
	state protocol.State,
	headersDB storage.Headers,
	receiptsDB storage.ExecutionReceipts,
	receipts mempool.ExecutionTree,
	pendingReceipts mempool.PendingReceipts,
	seals mempool.IncorporatedResultSeals,
	receiptValidator module.ReceiptValidator,
	receiptRequester module.Requester,
	config Config,
) *Core {
	return &Core{
		log:              log.With().Str("engine", "matching.Core").Logger(),
		tracer:           tracer,
		metrics:          metrics,
		mempool:          mempool,
		state:            state,
		headersDB:        headersDB,
		receiptsDB:       receiptsDB,
		receipts:         receipts,
		pendingReceipts:  pendingReceipts,
		seals:            seals,
		receiptValidator: receiptValidator,
		receiptRequester: receiptRequester,
		config:           config,
	}
}

// ProcessReceipt processes a new execution receipt.
// Any error indicates an unexpected problem in the protocol logic. The node's
// internal state might be corrupted. Hence, returned errors should be treated as fatal.
func (c *Core) ProcessReceipt(receipt *flow.ExecutionReceipt) error {
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
			Hex("origin", logging.ID(receipt.ExecutorID)).
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
		err := c.ProcessReceipt(childReceipt)
		if err != nil {
			// we don't want to wrap the error with any info from its parent receipt,
			// because the error has nothing to do with its parent receipt.
			return err
		}
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
	defer func() {
		c.metrics.OnReceiptProcessingDuration(time.Since(startTime))
	}()

	receiptSpan, _, isSampled := c.tracer.StartBlockSpan(context.Background(), receipt.ExecutionResult.BlockID, trace.CONMatchProcessReceipt)
	if isSampled {
		receiptSpan.LogFields(log.String("result_id", receipt.ExecutionResult.ID().String()))
		receiptSpan.LogFields(log.String("executor", receipt.ExecutorID.String()))
	}
	defer receiptSpan.Finish()

	// setup logger to capture basic information about the receipt
	log := c.log.With().
		Hex("receipt_id", logging.Entity(receipt)).
		Hex("result_id", logging.Entity(receipt.ExecutionResult)).
		Hex("execution_data_id", receipt.ExecutionResult.ExecutionDataID[:]).
		Hex("previous_result", receipt.ExecutionResult.PreviousResultID[:]).
		Hex("block_id", receipt.ExecutionResult.BlockID[:]).
		Hex("executor_id", receipt.ExecutorID[:]).
		Logger()
	initialState, finalState, err := getStartAndEndStates(receipt)
	if err != nil {
		if errors.Is(err, flow.ErrNoChunks) {
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
	executedBlock, err := c.headersDB.ByBlockID(receipt.ExecutionResult.BlockID)
	if err != nil {
		log.Debug().Msg("discarding receipt for unknown block")
		return false, nil
	}

	log = log.With().
		Uint64("block_view", executedBlock.View).
		Uint64("block_height", executedBlock.Height).
		Logger()
	log.Debug().Msg("execution receipt received")

	// if Execution Receipt is for block whose height is lower or equal to already sealed height
	//  => drop Receipt
	sealed, err := c.state.Sealed().Head()
	if err != nil {
		return false, fmt.Errorf("could not find sealed block: %w", err)
	}
	if executedBlock.Height <= sealed.Height {
		log.Debug().Msg("discarding receipt for already sealed and finalized block height")
		return false, nil
	}

	childSpan := c.tracer.StartSpanFromParent(receiptSpan, trace.CONMatchProcessReceiptVal)
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

	_, err = c.storeReceipt(receipt, executedBlock)
	if err != nil {
		return false, fmt.Errorf("failed to store receipt: %w", err)
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

// requestPendingReceipts requests the execution receipts of unsealed finalized
// blocks.
// it returns the number of pending receipts requests being created, and
// the first finalized height at which there is no receipt for the block
func (c *Core) requestPendingReceipts() (int, uint64, error) {
	finalSnapshot := c.state.Final()
	final, err := finalSnapshot.Head() // last finalized block
	if err != nil {
		return 0, 0, fmt.Errorf("could not get finalized height: %w", err)
	}
	_, seal, err := finalSnapshot.SealedResult() // last finalized seal
	if err != nil {
		return 0, 0, fmt.Errorf("could not retrieve latest finalized seal: %w", err)
	}
	sealed, err := c.headersDB.ByBlockID(seal.BlockID) // last sealed block
	if err != nil {
		return 0, 0, fmt.Errorf("could not get sealed height: %w", err)
	}

	// only request if number of unsealed finalized blocks exceeds the threshold
	if uint(final.Height-sealed.Height) < c.config.SealingThreshold {
		return 0, 0, nil
	}

	// order the missing blocks by height from low to high such that when
	// passing them to the missing block requester, they can be requested in the
	// right order. The right order gives the priority to the execution result
	// of lower height blocks to be requested first, since a gap in the sealing
	// heights would stop the sealing.
	missingBlocksOrderedByHeight := make([]flow.Identifier, 0, c.config.MaxResultsToRequest)

	var firstMissingHeight uint64 = math.MaxUint64
	// traverse each unsealed and finalized block with height from low to high,
	// if the result is missing, then add the blockID to a missing block list in
	// order to request them.
HEIGHT_LOOP:
	for height := sealed.Height + 1; height <= final.Height; height++ {
		// add at most <maxUnsealedResults> number of results
		if len(missingBlocksOrderedByHeight) >= int(c.config.MaxResultsToRequest) {
			break
		}

		// get the block header at this height (should not error as heights are finalized)
		header, err := c.headersDB.ByHeight(height)
		if err != nil {
			return 0, 0, fmt.Errorf("could not get header (height=%d): %w", height, err)
		}
		blockID := header.ID()

		receipts, err := c.receiptsDB.ByBlockID(blockID)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return 0, 0, fmt.Errorf("could not get receipts by block ID: %v, %w", blockID, err)
		}

		// We require at least 2 consistent receipts from different ENs to seal a block. If don't need to fetching receipts.
		// CAUTION: This is a temporary shortcut incompatible with the mature BFT protocol!
		// There might be multiple consistent receipts that commit to a wrong result. To guarantee
		// sealing liveness, we need to fetch receipts from those ENs, whose receipts we don't have yet.
		// TODO: update for full BFT
		for _, receiptsForResult := range receipts.GroupByResultID() {
			if receiptsForResult.GroupByExecutorID().NumberGroups() >= 2 {
				continue HEIGHT_LOOP
			}
		}

		missingBlocksOrderedByHeight = append(missingBlocksOrderedByHeight, blockID)
		if height < firstMissingHeight {
			firstMissingHeight = height
		}
	}

	// request missing execution results, if sealed height is low enough
	for _, blockID := range missingBlocksOrderedByHeight {
		c.receiptRequester.Query(blockID, filter.Any)
	}

	return len(missingBlocksOrderedByHeight), firstMissingHeight, nil
}

func (c *Core) OnBlockFinalization() error {
	startTime := time.Now()

	// request execution receipts for unsealed finalized blocks
	pendingReceiptRequests, firstMissingHeight, err := c.requestPendingReceipts()
	if err != nil {
		return fmt.Errorf("could not request pending block results: %w", err)
	}

	// Prune Execution Tree
	lastSealed, err := c.state.Sealed().Head()
	if err != nil {
		return fmt.Errorf("could not retrieve last sealed block : %w", err)
	}
	err = c.receipts.PruneUpToHeight(lastSealed.Height)
	if err != nil {
		return fmt.Errorf("failed to prune execution tree up to latest sealed and finalized block %v, height: %v: %w",
			lastSealed.ID(), lastSealed.Height, err)
	}

	err = c.pendingReceipts.PruneUpToHeight(lastSealed.Height)
	if err != nil {
		return fmt.Errorf("failed to prune pending receipts mempool up to latest sealed and finalized block %v, height: %v: %w",
			lastSealed.ID(), lastSealed.Height, err)
	}

	c.log.Info().
		Uint64("first_height_missing_result", firstMissingHeight).
		Uint("seals_size", c.seals.Size()).
		Uint("receipts_size", c.receipts.Size()).
		Int("pending_receipt_requests", pendingReceiptRequests).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("finalized block processed successfully")

	return nil
}

// getStartAndEndStates returns the pair: (start state commitment; final state commitment)
// Error returns:
//  * ErrNoChunks: if there are no chunks, i.e. the ExecutionResult is malformed
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
