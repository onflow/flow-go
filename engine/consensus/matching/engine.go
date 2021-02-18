// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package matching

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	chmodule "github.com/onflow/flow-go/module/chunks"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/module/validation"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
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

// Engine is the Matching engine, which builds seals by matching receipts (aka
// ExecutionReceipt, from execution nodes) and approvals (aka ResultApproval,
// from verification nodes), and saves the seals into seals mempool for adding
// into a new block.
type Engine struct {
	notifications.NoopConsumer

	unit                                 *engine.Unit                    // used to control startup/shutdown
	log                                  zerolog.Logger                  // used to log relevant actions with context
	engineMetrics                        module.EngineMetrics            // used to track sent and received messages
	tracer                               module.Tracer                   // used to trace execution
	mempool                              module.MempoolMetrics           // used to track mempool size
	metrics                              module.ConsensusMetrics         // used to track consensus metrics
	state                                protocol.State                  // used to access the  protocol state
	me                                   module.Local                    // used to access local node information
	receiptRequester                     module.Requester                // used to request missing execution receipts by block ID
	approvalConduit                      network.Conduit                 // used to request missing approvals from verification nodes
	receiptsDB                           storage.ExecutionReceipts       // to persist received execution receipts
	headersDB                            storage.Headers                 // used to check sealed headers
	indexDB                              storage.Index                   // used to check payloads for results
	payloadsDB                           storage.Payloads                // used to index receipt by the executed block ID
	incorporatedResults                  mempool.IncorporatedResults     // holds results that are connected to the sealed results waiting to be sealed
	receipts                             mempool.ExecutionTree           // holds indexes execution receipts by heights and can return connected children forks from a parent
	approvals                            mempool.Approvals               // holds result approvals in memory
	seals                                mempool.IncorporatedResultSeals // holds the seals to be included in the next proposal
	pendingReceipts                      mempool.PendingReceipts         // to buffer receipts whose previous result is missing
	missing                              map[flow.Identifier]uint        // track how often a block was missing
	assigner                             module.ChunkAssigner            // chunk assignment object
	isCheckingSealing                    *atomic.Bool                    // used to rate limit the checksealing call
	sealingThreshold                     uint                            // how many blocks between sealed/finalized before we request execution receipts
	maxResultsToRequest                  int                             // max number of finalized blocks for which we request execution results
	requiredApprovalsForSealConstruction uint                            // min number of approvals required for constructing a candidate seal
	receiptValidator                     module.ReceiptValidator         // used to validate receipts
	requestTracker                       *RequestTracker                 // used to keep track of number of approval requests, and blackout periods, by chunk
	approvalRequestsThreshold            uint64                          // min height difference between the latest finalized block and the block incorporating a result we would re-request approvals for
	emergencySealingActive               bool                            // flag which indicates if emergency sealing is active or not. NOTE: this is temporary while sealing & verification is under development
}

func New(
	log zerolog.Logger,
	engineMetrics module.EngineMetrics,
	tracer module.Tracer,
	mempool module.MempoolMetrics,
	conMetrics module.ConsensusMetrics,
	net module.Network,
	state protocol.State,
	me module.Local,
	receiptRequester module.Requester,
	receiptsDB storage.ExecutionReceipts,
	headersDB storage.Headers,
	indexDB storage.Index,
	payloadsDB storage.Payloads,
	incorporatedResults mempool.IncorporatedResults,
	receipts mempool.ExecutionTree,
	approvals mempool.Approvals,
	seals mempool.IncorporatedResultSeals,
	pendingReceipts mempool.PendingReceipts,
	assigner module.ChunkAssigner,
	validator module.ReceiptValidator,
	requiredApprovalsForSealConstruction uint,
	emergencySealingActive bool,
) (*Engine, error) {
	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:                                 engine.NewUnit(),
		log:                                  log.With().Str("engine", "matching").Logger(),
		engineMetrics:                        engineMetrics,
		tracer:                               tracer,
		mempool:                              mempool,
		metrics:                              conMetrics,
		state:                                state,
		me:                                   me,
		receiptRequester:                     receiptRequester,
		receiptsDB:                           receiptsDB,
		headersDB:                            headersDB,
		indexDB:                              indexDB,
		payloadsDB:                           payloadsDB,
		incorporatedResults:                  incorporatedResults,
		receipts:                             receipts,
		approvals:                            approvals,
		seals:                                seals,
		pendingReceipts:                      pendingReceipts,
		missing:                              make(map[flow.Identifier]uint),
		isCheckingSealing:                    atomic.NewBool(false),
		sealingThreshold:                     10,
		maxResultsToRequest:                  20,
		assigner:                             assigner,
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
		receiptValidator:                     validator,
		requestTracker:                       NewRequestTracker(10, 30),
		approvalRequestsThreshold:            10,
		emergencySealingActive:               emergencySealingActive,
	}

	e.mempool.MempoolEntries(metrics.ResourceResult, e.incorporatedResults.Size())
	e.mempool.MempoolEntries(metrics.ResourceReceipt, e.receipts.Size())
	e.mempool.MempoolEntries(metrics.ResourceApproval, e.approvals.Size())
	e.mempool.MempoolEntries(metrics.ResourceSeal, e.seals.Size())

	// register engine with the receipt provider
	_, err := net.Register(engine.ReceiveReceipts, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for results: %w", err)
	}

	// register engine with the approval provider
	_, err = net.Register(engine.ReceiveApprovals, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for approvals: %w", err)
	}

	// register engine to the channel for requesting missing approvals
	e.approvalConduit, err = net.Register(engine.RequestApprovalsByChunk, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for requesting approvals: %w", err)
	}

	// Context:
	// We expect a lot more Approvals compared to blocks or receipts. However, the level of
	// information only changes significantly with new blocks or new receipts.
	// We used to kick off the sealing check after every approval and receipt. In cases where
	// the sealing check takes a lot more time than processing the actual messages (which we
	// assume for the current implementation), we incur a large overhead as we check a lot
	// of conditions, which only change with new blocks or new receipts.
	// TEMPORARY FIX: to avoid sealing checks to monopolize the engine and delay processing
	// of receipts and approvals. Specifically, we schedule sealing checks every 2 seconds.
	e.unit.LaunchPeriodically(e.checkSealing, 2*time.Second, 120*time.Second)

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the propagation engine, we consider the engine up and running
// upon initialization.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the propagation engine, it closes the channel when all submit goroutines
// have ended.
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

// HandleReceipt pipes explicitly requested receipts to the process function.
// Receipts can come from this function or the receipt provider setup in the
// engine constructor.
func (e *Engine) HandleReceipt(originID flow.Identifier, receipt flow.Entity) {
	e.log.Debug().Msg("received receipt from requester engine")

	e.unit.Launch(func() {
		err := e.process(originID, receipt)
		if err != nil {
			e.log.Error().Err(err).Hex("origin", originID[:]).Msg("could not process receipt")
		}
	})
}

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {

	switch ev := event.(type) {
	case *flow.ExecutionReceipt:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageExecutionReceipt)
		e.unit.Lock()
		defer e.unit.Unlock()
		defer e.engineMetrics.MessageHandled(metrics.EngineMatching, metrics.MessageExecutionReceipt)
		return e.onReceipt(originID, ev)
	case *flow.ResultApproval:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		if e.requiredApprovalsForSealConstruction < 1 {
			// if we don't require approvals to construct a seal, don't even process approvals.
			return nil
		}
		e.unit.Lock()
		defer e.unit.Unlock()
		defer e.engineMetrics.MessageHandled(metrics.EngineMatching, metrics.MessageResultApproval)
		return e.onApproval(originID, ev)
	case *messages.ApprovalResponse:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		if e.requiredApprovalsForSealConstruction < 1 {
			// if we don't require approvals to construct a seal, don't even process approvals.
			return nil
		}
		e.unit.Lock()
		defer e.unit.Unlock()
		defer e.engineMetrics.MessageHandled(metrics.EngineMatching, metrics.MessageResultApproval)
		return e.onApproval(originID, &ev.Approval)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// onReceipt processes a new execution receipt.
func (e *Engine) onReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {
	// when receiving a receipt, we might not be able to verify it if its previous result
	// is unknown.  In this case, instead of dropping it, we store it in the pending receipts
	// mempool, and process it later when its parent result has been received and processed.
	// therefore, if a receipt is processed, we will check if it is the previous results of
	// some pending receipts, if there are, then processing them one after another, as onReceipt
	// is behind a lock
	receiptID := receipt.ID()
	resultID := receipt.ExecutionResult.ID()

	processed, err := e.onCurrentReceipt(receipt)
	if err != nil {
		e.log.Error().
			Err(err).
			Hex("receipt_id", receiptID[:]).
			Hex("result_id", resultID[:]).
			Hex("origin_id", originID[:]).
			Msg("could not process receipt")
		return nil
	}

	if !processed {
		return nil
	}

	childReceipts := e.pendingReceipts.ByPreviousResultID(resultID)
	e.pendingReceipts.Rem(receiptID)

	for _, childReceipt := range childReceipts {
		// recursively processing the child receipts, since onReceipt
		// is logging error internal already, we could ignore the returned
		// error here
		_ = e.onReceipt(childReceipt.ExecutorID, childReceipt)
	}

	return nil
}

func (e *Engine) onCurrentReceipt(receipt *flow.ExecutionReceipt) (bool, error) {
	startTime := time.Now()
	receiptSpan := e.tracer.StartSpan(receipt.ID(), trace.CONMatchOnReceipt)
	defer func() {
		e.metrics.OnReceiptProcessingDuration(time.Since(startTime))
		receiptSpan.Finish()
	}()

	receiptID := receipt.ID()
	resultID := receipt.ExecutionResult.ID()

	log := e.log.With().
		Hex("receipt_id", receiptID[:]).
		Hex("result_id", resultID[:]).
		Hex("previous_result", receipt.ExecutionResult.PreviousResultID[:]).
		Hex("block_id", receipt.ExecutionResult.BlockID[:]).
		Hex("executor_id", receipt.ExecutorID[:]).
		Logger()

	initialState, finalState, err := validation.IntegrityCheck(receipt)
	if err != nil {
		log.Error().Msg("received execution receipt that didn't pass the integrity check")
		return false, engine.NewInvalidInputErrorf("%w", err)
	}

	log = log.With().
		Hex("initial_state", initialState).
		Hex("final_state", finalState).Logger()

	// if the receipt is for an unknown block, skip it. It will be re-requested
	// later by `requestPending` function.
	head, err := e.headersDB.ByBlockID(receipt.ExecutionResult.BlockID)
	if err != nil {
		log.Debug().Msg("discarding receipt for unknown block")
		return false, nil
	}

	log = log.With().
		Uint64("block_view", head.View).
		Uint64("block_height", head.Height).
		Logger()
	log.Debug().Msg("execution receipt received")

	// if Execution Receipt is for block whose height is lower or equal to already sealed height
	//  => drop Receipt
	sealed, err := e.state.Sealed().Head()
	if err != nil {
		return false, fmt.Errorf("could not find sealed block: %w", err)
	}

	isSealed := head.Height <= sealed.Height
	if isSealed {
		log.Debug().Msg("discarding receipt for already sealed and finalized block height")
		return false, nil
	}

	// we don't want to validate the same receipt more than once,
	// if this receipt has been stored in receipts database, it must have been
	// validated, either by ourselve or the consensus committee, so we could skip
	// validating it.
	_, err = e.receiptsDB.ByID(receiptID)
	if err != nil {
		// it's a known receipt, skip processing
		childSpan := e.tracer.StartSpanFromParent(receiptSpan, trace.CONMatchOnReceiptVal)
		err = e.receiptValidator.Validate([]*flow.ExecutionReceipt{receipt})
		childSpan.Finish()

		if validation.IsMissingPreviousResultError(err) {
			// If previous result is missing, we can't validate this receipt.
			// althrough we will request its previous (potentially multiple receipts),
			// We don't want to drop it now, because when the missing previous arrive
			// in a wrong order, they will still be dropped, and causing the catch up
			// to be inefficient.
			// Instead, we could cache the receipt in case it arrives earlier than its
			// previous receipt.
			// for instance, given blocks A <- B <- C <- D <- E, if we receive their receipts
			// in the order of [E,C,D,B,A], then:
			// if we drop the missing previous receipts, then only A will be processed;
			// if we cache the missing previous receipts, then all of them will be processed, because
			// once A is processed, we will check if there is a child receipt pending,
			// if yes, then process it.
			e.pendingReceipts.Add(receipt)
			log.Info().Msg("receipt is cached because its previous result is missing")
			return false, nil
		}
	}

	if err != nil {
		return false, fmt.Errorf("failed to validate execution receipt: %w", err)
	}

	_, err = e.storeReceipt(receipt, head)
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
	_, err = e.storeIncorporatedResult(receipt)
	if err != nil {
		return false, fmt.Errorf("failed to store incorporated result: %w", err)
	}

	log.Info().Msg("execution result processed and stored")

	return true, nil
}

// storeReceipt adds the receipt to the receipts mempool as well as to the persistent storage layer.
// Return values:
//  * bool to indicate whether the receipt is stored.
// 	* `engine.DuplicatedEntryError` [sentinel error] if entry already present in mempool and we don't need to process this again
//	* exception in case something went wrong
// 	* nil in case of success
func (e *Engine) storeReceipt(receipt *flow.ExecutionReceipt, head *flow.Header) (bool, error) {
	added, err := e.receipts.AddReceipt(receipt, head)
	if err != nil {
		return false, fmt.Errorf("adding receipt (%x) to mempool failed: %w", receipt.ID(), err)
	}
	if !added {
		return false, nil
	}
	e.mempool.MempoolEntries(metrics.ResourceReceipt, e.receipts.Size())

	// persist receipt in database. Even if the receipt is already in persistent storage,
	// we still need to process it, as it is not in the mempool. This can happen if the
	// mempool was wiped during a node crash.
	err = e.receiptsDB.Store(receipt) // internally de-duplicates
	if err != nil && !errors.Is(err, storage.ErrAlreadyExists) {
		return false, fmt.Errorf("could not persist receipt: %w", err)
	}
	return true, nil
}

// storeIncorporatedResult creates an `IncorporatedResult` and adds it to incorporated results mempool
// Error returns:
//  * bool to indicate whether the receipt is stored.
// 	* `engine.DuplicatedEntryError` [sentinel error] if entry already present in mempool
//	* exception in case something went wrong
// 	* nil in case of success
func (e *Engine) storeIncorporatedResult(receipt *flow.ExecutionReceipt) (bool, error) {
	// Create an IncorporatedResult and add it to the mempool
	added, err := e.incorporatedResults.Add(
		flow.NewIncorporatedResult(
			receipt.ExecutionResult.BlockID,
			&receipt.ExecutionResult,
		),
	)
	if err != nil {
		return false, fmt.Errorf("error inserting incorporated result in mempool: %w", err)
	}
	if !added {
		return false, nil
	}
	e.mempool.MempoolEntries(metrics.ResourceResult, e.incorporatedResults.Size())
	return true, nil
}

// onApproval processes a new result approval.
func (e *Engine) onApproval(originID flow.Identifier, approval *flow.ResultApproval) error {
	startTime := time.Now()
	approvalSpan := e.tracer.StartSpan(approval.ID(), trace.CONMatchOnApproval)
	defer func() {
		e.metrics.OnApprovalProcessingDuration(time.Since(startTime))
		approvalSpan.Finish()
	}()

	log := e.log.With().
		Hex("approval_id", logging.Entity(approval)).
		Hex("block_id", approval.Body.BlockID[:]).
		Hex("result_id", approval.Body.ExecutionResultID[:]).
		Logger()
	log.Info().Msg("result approval received")

	// Check that the message's origin (as established by the networking layer) is
	// equal to the message's creator as reported by the message itself. Thereby,
	// we rely on the networking layer for enforcing message integrity via the
	// networking key.
	if approval.Body.ApproverID != originID {
		return engine.NewInvalidInputErrorf("invalid origin for approval: %x", originID)
	}

	// check if we already have the block the approval pertains to
	head, err := e.state.AtBlockID(approval.Body.BlockID).Head()
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("failed to retrieve header for block %x: %w", approval.Body.BlockID, err)
		}
		// Don't error if the block is not known yet, because the checks in the
		// else-branch below are called again when we try to match approvals to chunks.
	} else {
		// drop approval, if it is for block whose height is lower or equal to already sealed height
		sealed, err := e.state.Sealed().Head()
		if err != nil {
			return fmt.Errorf("could not find sealed block: %w", err)
		}
		if sealed.Height >= head.Height {
			log.Debug().Msg("discarding approval for already sealed and finalized block height")
			return nil
		}

		// Check if the approver was a staked verifier at that block.
		err = e.ensureStakedNodeWithRole(approval.Body.ApproverID, head, flow.RoleVerification)
		if err != nil {
			return fmt.Errorf("failed to process approval: %w", err)
		}

		// TODO: check the approval's cryptographic integrity
	}

	// store in the memory pool (it won't be added if it is already in there).
	added, err := e.approvals.Add(approval)
	if err != nil {
		return err
	}
	if !added {
		log.Debug().Msg("skipping approval already in mempool")
		return nil
	}
	e.mempool.MempoolEntries(metrics.ResourceApproval, e.approvals.Size())

	return nil
}

// checkSealing checks if there is anything worth sealing at the moment.
func (e *Engine) checkSealing() {
	// only check sealing when no one else is checking
	canCheck := e.isCheckingSealing.CAS(false, true)
	if !canCheck {
		return
	}

	defer e.isCheckingSealing.Store(false)

	e.unit.Lock()
	defer e.unit.Unlock()

	err := e.checkingSealing()
	if err != nil {
		e.log.Fatal().Err(err).Msg("error in sealing protocol")
	}
}

func (e *Engine) checkingSealing() error {
	startTime := time.Now()
	sealingSpan, _ := e.tracer.StartSpanFromContext(context.Background(), trace.CONMatchCheckSealing)
	defer func() {
		e.metrics.CheckSealingDuration(time.Since(startTime))
		sealingSpan.Finish()
	}()

	sealableResultsSpan := e.tracer.StartSpanFromParent(sealingSpan, trace.CONMatchCheckSealingSealableResults)

	// get all results that have collected enough approvals on a per-chunk basis
	sealableResults, nextUnsealeds, err := e.sealableResults()
	if err != nil {
		return fmt.Errorf("could not get sealable execution results: %w", err)
	}

	lg := e.log.With().
		Int("sealable_results_count", len(sealableResults)).
		Str("next_unsealed_results", nextUnsealeds.String()).
		Logger()

	// log warning if we are going to overflow the seals mempool
	if space := e.seals.Limit() - e.seals.Size(); len(sealableResults) > int(space) {
		lg.Warn().
			Int("space", int(space)).
			Msg("overflowing seals mempool")
	}

	// Start spans for tracing within the parent spans trace.CONProcessBlock and
	// trace.CONProcessCollection
	for _, incorporatedResult := range sealableResults {
		// For each execution result, we load the trace.CONProcessBlock span for the executed block. If we find it, we
		// start a child span that will run until this function returns.
		if span, ok := e.tracer.GetSpan(incorporatedResult.Result.BlockID, trace.CONProcessBlock); ok {
			childSpan := e.tracer.StartSpanFromParent(span, trace.CONMatchCheckSealing, opentracing.StartTime(startTime))
			defer childSpan.Finish()
		}

		// For each execution result, we load all the collection that are in the executed block.
		index, err := e.indexDB.ByBlockID(incorporatedResult.Result.BlockID)
		if err != nil {
			continue
		}
		for _, id := range index.CollectionIDs {
			// For each collection, we load the trace.CONProcessCollection span. If we find it, we start a child span
			// that will run until this function returns.
			if span, ok := e.tracer.GetSpan(id, trace.CONProcessCollection); ok {
				childSpan := e.tracer.StartSpanFromParent(span, trace.CONMatchCheckSealing, opentracing.StartTime(startTime))
				defer childSpan.Finish()
			}
		}
	}

	// seal the matched results
	var sealedResultIDs []flow.Identifier
	var sealedBlockIDs []flow.Identifier
	for _, incorporatedResult := range sealableResults {
		err := e.sealResult(incorporatedResult)
		if err != nil {
			return fmt.Errorf("failed to seal result (%x): %w", incorporatedResult.ID(), err)
		}

		// mark the result cleared for mempool cleanup
		// TODO: for Phase 2a, we set the value of IncorporatedResult.IncorporatedBlockID
		// to the block the result is for. Therefore, it must be possible to
		// incorporate the result and seal it on one fork and subsequently on a
		// different fork incorporate same result and seal it. So we need to
		// keep it in the mempool for now. This will be changed in phase 3.

		// sealedResultIDs = append(sealedResultIDs, incorporatedResult.ID())
		sealedBlockIDs = append(sealedBlockIDs, incorporatedResult.Result.BlockID)
	}

	// finish tracing spans
	sealableResultsSpan.Finish()
	for _, blockID := range sealedBlockIDs {
		index, err := e.indexDB.ByBlockID(blockID)
		if err != nil {
			continue
		}
		for _, id := range index.CollectionIDs {
			e.tracer.FinishSpan(id, trace.CONProcessCollection)
		}
		e.tracer.FinishSpan(blockID, trace.CONProcessBlock)
	}

	// clear the memory pools
	clearPoolsSpan := e.tracer.StartSpanFromParent(sealingSpan, trace.CONMatchCheckSealingClearPools)
	err = e.clearPools(sealedResultIDs)
	clearPoolsSpan.Finish()
	if err != nil {
		return fmt.Errorf("failed to clean mempools: %w", err)
	}

	// request execution receipts for unsealed finalized blocks
	requestReceiptsSpan := e.tracer.StartSpanFromParent(sealingSpan, trace.CONMatchCheckSealingRequestPendingReceipts)
	pendingReceiptRequests, firstMissingHeight, err := e.requestPendingReceipts()
	requestReceiptsSpan.Finish()

	if err != nil {
		return fmt.Errorf("could not request pending block results: %w", err)
	}

	// request result approvals for pending incorporated results
	requestApprovalsSpan := e.tracer.StartSpanFromParent(sealingSpan, trace.CONMatchCheckSealingRequestPendingApprovals)
	pendingApprovalRequests, err := e.requestPendingApprovals()
	requestApprovalsSpan.Finish()
	if err != nil {
		return fmt.Errorf("could not request pending result approvals: %w", err)
	}

	mempoolHasNextSeal := false

	// check if we end up created a seal in mempool for the next unsealed block
	for _, unsealed := range nextUnsealeds {
		_, mempoolHasNextSeal = e.seals.ByID(unsealed.IncorporatedResultID)
		if mempoolHasNextSeal {
			break
		}
	}

	lg.Info().
		Int("sealable_incorporated_results", len(sealedBlockIDs)).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Bool("mempool_has_next_seal", mempoolHasNextSeal).
		Uint64("first_height_missing_result", firstMissingHeight).
		Uint("seals_size", e.seals.Size()).
		Uint("receipts_size", e.receipts.Size()).
		Uint("incorporated_size", e.incorporatedResults.Size()).
		Uint("approval_size", e.approvals.Size()).
		Int("pending_receipt_requests", pendingReceiptRequests).
		Int("pending_approval_requests", pendingApprovalRequests).
		Uint("pending_receipts_count", e.pendingReceipts.Size()).
		Msg("checking sealing finished successfully")

	return nil
}

func nextUnsealedID(state protocol.State) (flow.Identifier, bool, error) {
	lastSealed, err := state.Sealed().Head()
	if err != nil {
		return flow.ZeroID, false, fmt.Errorf("could not get last sealed: %w", err)
	}

	nextUnsealedHeight := lastSealed.Height + 1
	nextUnsealed, err := state.AtHeight(nextUnsealedHeight).Head()
	if errors.Is(err, storage.ErrNotFound) {
		// next unsealed block has not been finalized yet.
		return flow.ZeroID, false, nil
	}
	if err != nil {
		return flow.ZeroID, false, fmt.Errorf("could not get block at heigh:%v, %w", nextUnsealed, err)
	}
	return nextUnsealed.ID(), true, nil
}

// sealableResults returns the IncorporatedResults from the mempool that have
// collected enough approvals on a per-chunk basis, as defined by the matchChunk
// function. It also filters out results that have an incorrect sub-graph.
// It specifically returns the information for the next unsealed results which will
// be useful for debugging the potential sealing halt issue
func (e *Engine) sealableResults() ([]*flow.IncorporatedResult, nextUnsealedResults, error) {
	var results []*flow.IncorporatedResult

	lastFinalized, err := e.state.Final().Head()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get last finalized block: %w", err)
	}

	nextUnsealed, nextUnsealedIsFinalized, err := nextUnsealedID(e.state)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get the next unsealed block id: %w", err)
	}

	nextUnsealeds := make([]*nextUnsealedResult, 0)
	// go through the results mempool and check which ones we have collected
	// enough approvals for
	for _, incorporatedResult := range e.incorporatedResults.All() {
		// not finding the block header for an incorporated result is a fatal
		// implementation bug, as we only add results to the IncorporatedResults
		// mempool, where _both_ the block that incorporates the result as well
		// as the block the result pertains to are known
		block, err := e.headersDB.ByBlockID(incorporatedResult.Result.BlockID)
		if err != nil {
			return nil, nil, fmt.Errorf("could not retrieve block: %w", err)
		}

		// At this point we can be sure that all needed checks on validity of ER
		// were executed prior to this point, since we perform validation of every ER
		// before adding it into mempool. Hence, the mempool can contain only valid entries.

		// the chunk assigment is based on the first block in its fork which
		// contains a receipt that commits to this result.
		assignment, err := e.assigner.Assign(incorporatedResult.Result, incorporatedResult.IncorporatedBlockID)
		if state.IsNoValidChildBlockError(err) {
			continue
		}
		if err != nil {
			// at this point, we know the block and a valid child block exists. Not being able to compute
			// the assignment constitutes a fatal implementation bug:
			return nil, nil, fmt.Errorf("could not determine chunk assignment: %w", err)
		}

		// The production system must always have a system chunk. Any result without chunks
		// is wrong and should _not_ be sealed. While it is fundamentally the ReceiptValidator's
		// responsibility to filter out results without chunks, there is no harm in also in enforcing
		// the same condition here as well. It simplifies the code, because otherwise the
		// matching engine must enforce equality of start and end state for a result with zero chunks,
		// in the absence of anyone else doing do.
		matched := false
		unmatchedIndex := -1
		// check that each chunk collects enough approvals
		for i, chunk := range incorporatedResult.Result.Chunks {
			matched, err = e.matchChunk(incorporatedResult, block, chunk, assignment)
			if err != nil {
				return nil, nil, fmt.Errorf("could not match chunk: %w", err)
			}
			if !matched {
				unmatchedIndex = i
				break
			}
		}

		emergencySealed := false
		// ATTENTION: this is a temporary solution called emergency sealing. Emergency sealing is a special case
		// when we seal ERs that don't have enough approvals but are deep enough in the chain resulting in halting sealing
		// process. This will be removed when implementation of seal & verification is finished.
		if !matched && e.emergencySealingActive {
			incorporatedBlock, err := e.headersDB.ByBlockID(incorporatedResult.IncorporatedBlockID)
			if err != nil {
				return nil, nil, fmt.Errorf("could not get block %v: %w", incorporatedResult.IncorporatedBlockID, err)
			}
			// Note:
			// we assume the incorporatedBlock is for a unsealed block, if
			// there are DefaultEmergencySealingThreshold number of blocks between incorporatedBlock
			// and lastFinalized, it means there are as many unsealed and finalized blocks, because
			// should trigger the emergency sealing
			if incorporatedBlock.Height+DefaultEmergencySealingThreshold <= lastFinalized.Height {
				emergencySealed = true
			}
		}

		if matched || emergencySealed {
			// add the result to the results that should be sealed

			// HOTFIX: the following will only consider a block sealable if we have _more_ than 1 receipt committing to the same result
			receipts, err := e.receiptsDB.ByBlockIDAllExecutionReceipts(incorporatedResult.Result.BlockID)
			if err != nil {
				log.Error().Err(err).
					Hex("block_id", logging.ID(incorporatedResult.Result.BlockID)).
					Msg("could not get receipts by block ID")
				continue
			}
			block2ResultsCounter := make(map[flow.Identifier]uint)
			for _, rcpt := range receipts {
				resultID := rcpt.ExecutionResult.ID()
				block2ResultsCounter[resultID] += 1
				if block2ResultsCounter[resultID] >= 2 {
					results = append(results, incorporatedResult)
				}
			}
		}

		if nextUnsealedIsFinalized {
			if incorporatedResult.Result.BlockID == nextUnsealed {
				nextUnsealeds = append(nextUnsealeds, &nextUnsealedResult{
					BlockID:                       incorporatedResult.Result.BlockID,
					Height:                        block.Height,
					ResultID:                      incorporatedResult.Result.ID(),
					IncorporatedResultID:          incorporatedResult.ID(),
					TotalChunks:                   len(incorporatedResult.Result.Chunks),
					FirstUnmatchedChunkIndex:      unmatchedIndex,
					SufficientApprovalsForSealing: matched,
					QualifiesForEmergencySealing:  emergencySealed,
				})
			}
		}
	}

	return results, nextUnsealeds, nil
}

// matchChunk checks that the number of ResultApprovals collected by a chunk
// exceeds the required threshold. It also populates the IncorporatedResult's
// collection of approval signatures to avoid repeated work.
func (e *Engine) matchChunk(incorporatedResult *flow.IncorporatedResult, block *flow.Header, chunk *flow.Chunk, assignment *chunks.Assignment) (bool, error) {

	// get all the chunk approvals from mempool
	approvals := e.approvals.ByChunk(incorporatedResult.Result.ID(), chunk.Index)

	validApprovals := uint(0)
	for approverID, approval := range approvals {
		// skip if the incorporated result already has a signature for that
		// chunk and verifier
		_, ok := incorporatedResult.GetSignature(chunk.Index, approverID)
		if ok {
			validApprovals++
			continue
		}

		// if the approval comes from a node that wasn't even a staked
		// verifier at that block, remove the approval from the mempool.
		err := e.ensureStakedNodeWithRole(approverID, block, flow.RoleVerification)
		if err != nil {
			if engine.IsInvalidInputError(err) {
				_, err = e.approvals.RemApproval(approval)
				if err != nil {
					return false, fmt.Errorf("failed to remove approval from mempool: %w", err)
				}
				continue
			}
			return false, fmt.Errorf("failed to match chunks: %w", err)
		}
		// skip approval if verifier was not assigned to this chunk.
		if !chmodule.IsValidVerifer(assignment, chunk, approverID) {
			continue
		}

		// AddReceipt signature to incorporated result so that we don't have to check it again.
		incorporatedResult.AddSignature(chunk.Index, approverID, approval.Body.AttestationSignature)
		validApprovals++
	}

	return validApprovals >= e.requiredApprovalsForSealConstruction, nil
}

// TODO: to be extracted as a common function in state/protocol/state.go
// ToDo: add check that node was not ejected
// checkIsStakedNodeWithRole checks whether, at the given block, `nodeID`
//   * is an authorized member of the network
//   * has _positive_ weight
//   * and has the expected role
// Returns the following errors:
//   * sentinel engine.InvalidInputError if any of the above-listed conditions are violated.
//   * generic error indicating a fatal internal bug
// Note: the method receives the block header as proof of its existence.
// Therefore, we consider the case where the respective block is unknown to the
// protocol state as a symptom of a fatal implementation bug.
func (e *Engine) ensureStakedNodeWithRole(nodeID flow.Identifier, block *flow.Header, expectedRole flow.Role) error {
	// get the identity of the origin node
	identity, err := e.state.AtBlockID(block.ID()).Identity(nodeID)
	if err != nil {
		if protocol.IsIdentityNotFound(err) {
			return engine.NewInvalidInputErrorf("unknown node identity: %w", err)
		}
		// unexpected exception
		return fmt.Errorf("failed to retrieve node identity: %w", err)
	}

	// check that the origin is a verification node
	if identity.Role != expectedRole {
		return engine.NewInvalidInputErrorf("expected node %x to have identity %s but got %s", nodeID, expectedRole, identity.Role)
	}

	// check if the identity has a stake
	if identity.Stake == 0 {
		return engine.NewInvalidInputErrorf("node has zero stake (%x)", identity.NodeID)
	}

	// check that node was not ejected
	if identity.Ejected {
		return engine.NewInvalidInputErrorf("node has zero stake (%x)", identity.NodeID)
	}

	return nil
}

// sealResult creates a seal for the incorporated result and adds it to the
// seals mempool.
func (e *Engine) sealResult(incorporatedResult *flow.IncorporatedResult) error {
	// collect aggregate signatures
	aggregatedSigs := incorporatedResult.GetAggregatedSignatures()

	// get final state of execution result
	finalState, ok := incorporatedResult.Result.FinalStateCommitment()
	if !ok {
		// message correctness should have been checked before: failure here is an internal implementation bug
		return fmt.Errorf("failed to get final state commitment from Execution Result")
	}

	// TODO: Check SPoCK proofs

	// generate & store seal
	seal := &flow.Seal{
		BlockID:                incorporatedResult.Result.BlockID,
		ResultID:               incorporatedResult.Result.ID(),
		FinalState:             finalState,
		AggregatedApprovalSigs: aggregatedSigs,
	}

	// we don't care if the seal is already in the mempool
	_, err := e.seals.Add(&flow.IncorporatedResultSeal{
		IncorporatedResult: incorporatedResult,
		Seal:               seal,
	})
	if err != nil {
		return fmt.Errorf("failed to store IncorporatedResultSeal in mempool: %w", err)
	}

	return nil
}

// clearPools clears the memory pools of all entities related to blocks that are
// already sealed. If we don't know the block, we purge the entities once we
// have called checkSealing 1000 times without seeing the block (it's probably
// no longer a valid extension of the state anyway).
func (e *Engine) clearPools(sealedIDs []flow.Identifier) error {

	clear := make(map[flow.Identifier]bool)
	for _, sealedID := range sealedIDs {
		clear[sealedID] = true
	}

	sealed, err := e.state.Sealed().Head()
	if err != nil {
		return fmt.Errorf("could not get sealed head: %w", err)
	}

	// build a helper function that determines if an entity should be cleared
	// if it references the block with the given ID
	missingIDs := make(map[flow.Identifier]bool) // count each missing block only once
	shouldClear := func(blockID flow.Identifier) (bool, error) {
		if e.missing[blockID] >= 1000 {
			return true, nil // clear if block is missing for 1000 seals already
		}
		header, err := e.headersDB.ByBlockID(blockID)
		if errors.Is(err, storage.ErrNotFound) {
			missingIDs[blockID] = true
			return false, nil // keep if the block is missing, but count times missing
		}
		if err != nil {
			return false, fmt.Errorf("could not check block expiry: %w", err)
		}
		if header.Height <= sealed.Height {
			return true, nil // clear if sealed block is same or higher than referenced block
		}
		return false, nil
	}

	// The receipts mempool is aware of the Execution Tree structure formed by the execution results.
	// It supports pruning by height: only results descending from the latest sealed and finalized
	// result are relevant. Hence, we can prune all results for blocks _below_ the latest block with
	// a finalized seal. Results of sufficient height for forks that conflict with the finalized fork
	// are retained in the mempool. However, such orphaned forks do not grow anymore and their
	// results will be progressively flushed out with increasing sealed-finalized height.
	err = e.receipts.PruneUpToHeight(sealed.Height)
	if err != nil {
		return fmt.Errorf("failed to clean receipts mempool: %w", err)
	}

	// for each memory pool, clear if the related block is no longer relevant or
	// if the seal was already built for it (except for seals themselves)
	for _, result := range e.incorporatedResults.All() {
		remove, err := shouldClear(result.Result.BlockID)
		if err != nil {
			return fmt.Errorf("failed to evaluate cleaning condition for incorporated results mempool: %w", err)
		}
		if remove || clear[result.ID()] {
			_ = e.incorporatedResults.Rem(result)
		}
	}

	// clear approvals mempool
	for _, approval := range e.approvals.All() {
		remove, err := shouldClear(approval.Body.BlockID)
		if err != nil {
			return fmt.Errorf("failed to evaluate cleaning condition for approvals mempool: %w", err)
		}
		if remove || clear[approval.Body.ExecutionResultID] {
			// delete all the approvals for the corresponding chunk
			_, err = e.approvals.RemChunk(approval.Body.ExecutionResultID, approval.Body.ChunkIndex)
			if err != nil {
				return fmt.Errorf("failed to clean approvals mempool: %w", err)
			}
		}
	}

	// clear seals mempool
	for _, seal := range e.seals.All() {
		remove, err := shouldClear(seal.Seal.BlockID)
		if err != nil {
			return fmt.Errorf("failed to evaluate cleaning condition for seals mempool: %w", err)
		}
		if remove {
			_ = e.seals.Rem(seal.ID())
		}
	}

	// clear the request tracker of all items corresponding to results that are
	// no longer in the incorporated-results mempool
	for resultID := range e.requestTracker.GetAll() {
		if _, _, ok := e.incorporatedResults.ByResultID(resultID); !ok {
			e.requestTracker.Remove(resultID)
		}
	}

	// for each missing block that we are tracking, remove it from tracking if
	// we now know that block or if we have just cleared related resources; then
	// increase the count for the remaining missing blocks
	for missingID, count := range e.missing {
		_, err := e.headersDB.ByBlockID(missingID)
		if count >= 1000 || err == nil {
			delete(e.missing, missingID)
		}
	}
	for missingID := range missingIDs {
		e.missing[missingID]++
	}

	e.mempool.MempoolEntries(metrics.ResourceResult, e.incorporatedResults.Size())
	e.mempool.MempoolEntries(metrics.ResourceReceipt, e.receipts.Size())
	e.mempool.MempoolEntries(metrics.ResourceApproval, e.approvals.Size())
	e.mempool.MempoolEntries(metrics.ResourceSeal, e.seals.Size())
	return nil
}

// requestPendingReceipts requests the execution receipts of unsealed finalized
// blocks.
// it returns the number of pending receipts requests being created
func (e *Engine) requestPendingReceipts() (int, uint64, error) {

	// last sealed block
	sealed, err := e.state.Sealed().Head()
	if err != nil {
		return 0, 0, fmt.Errorf("could not get sealed height: %w", err)
	}

	// last finalized block
	final, err := e.state.Final().Head()
	if err != nil {
		return 0, 0, fmt.Errorf("could not get finalized height: %w", err)
	}

	// only request if number of unsealed finalized blocks exceeds the threshold
	if uint(final.Height-sealed.Height) < e.sealingThreshold {
		return 0, 0, nil
	}

	// order the missing blocks by height from low to high such that when
	// passing them to the missing block requester, they can be requested in the
	// right order. The right order gives the priority to the execution result
	// of lower height blocks to be requested first, since a gap in the sealing
	// heights would stop the sealing.
	missingBlocksOrderedByHeight := make([]flow.Identifier, 0, e.maxResultsToRequest)

	// turn mempool into Lookup table: BlockID -> Result
	knownResultForBlock := make(map[flow.Identifier]struct{})
	for _, r := range e.incorporatedResults.All() {
		knownResultForBlock[r.Result.BlockID] = struct{}{}
	}
	for _, s := range e.seals.All() {
		knownResultForBlock[s.Seal.BlockID] = struct{}{}
	}

	var firstMissingHeight uint64
	// traverse each unsealed and finalized block with height from low to high,
	// if the result is missing, then add the blockID to a missing block list in
	// order to request them.
	for height := sealed.Height + 1; height <= final.Height; height++ {
		// add at most <maxUnsealedResults> number of results
		if len(missingBlocksOrderedByHeight) >= e.maxResultsToRequest {
			break
		}

		// get the block header at this height (should not error as heights are finalized)
		header, err := e.headersDB.ByHeight(height)
		if err != nil {
			return 0, 0, fmt.Errorf("could not get header (height=%d): %w", height, err)
		}

		// check if we have an result for the block at this height
		blockID := header.ID()

		_, ok := knownResultForBlock[blockID]
		if ok {
			continue
		}

		// since the index is only added when the block which includes the receipts
		// get finalized, so the returned receipts must be from finalized blocks.
		// Therefore, the return receipts must be incoporated receipts, which
		// are safe to be added to the mempool
		receipts, err := e.receiptsDB.ByBlockIDAllExecutionReceipts(blockID)
		if err != nil {
			return 0, 0, fmt.Errorf("could not get receipts by block ID: %v, %w", blockID, err)
		}

		if len(receipts) > 0 {
			for _, receipt := range receipts {
				_, err = e.receipts.AddReceipt(receipt, header)
				if err != nil {
					return 0, 0, fmt.Errorf("could not add receipt to receipts mempool %v, %w", receipt.ID(), err)
				}

				_, err = e.incorporatedResults.Add(
					flow.NewIncorporatedResult(
						receipt.ExecutionResult.BlockID,
						&receipt.ExecutionResult,
					),
				)

				if err != nil {
					return 0, 0, fmt.Errorf("could not add result to incorporated results mempool %v, %w", receipt.ID(), err)
				}
			}
			continue
		}

		missingBlocksOrderedByHeight = append(missingBlocksOrderedByHeight, blockID)
		if firstMissingHeight == 0 {
			firstMissingHeight = height
		}

	}

	// request missing execution results, if sealed height is low enough
	for _, blockID := range missingBlocksOrderedByHeight {
		e.receiptRequester.EntityByID(blockID, filter.Any)
	}

	return len(missingBlocksOrderedByHeight), firstMissingHeight, nil
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
// it returns the number of pending approvals requests being created
func (e *Engine) requestPendingApprovals() (int, error) {
	// skip requesting approvals if they are not required for sealing
	if e.requiredApprovalsForSealConstruction == 0 {
		return 0, nil
	}

	sealed, err := e.state.Sealed().Head() // last sealed block
	if err != nil {
		return 0, fmt.Errorf("could not get sealed height: %w", err)
	}
	final, err := e.state.Final().Head() // last finalized block
	if err != nil {
		return 0, fmt.Errorf("could not get finalized height: %w", err)
	}
	if sealed.Height+e.approvalRequestsThreshold >= final.Height {
		return 0, nil
	}

	// Reaching the following code implies:
	// 0 <= sealed.Height < final.Height - approvalRequestsThreshold
	// Hence, the following operation cannot underflow
	maxHeightForRequesting := final.Height - e.approvalRequestsThreshold

	requestCount := 0
	for _, r := range e.incorporatedResults.All() {
		resultID := r.Result.ID()

		// not finding the block that the result was incorporated in is a fatal
		// error at this stage
		block, err := e.headersDB.ByBlockID(r.IncorporatedBlockID)
		if err != nil {
			return 0, fmt.Errorf("could not retrieve block: %w", err)
		}

		if block.Height > maxHeightForRequesting {
			continue
		}

		// If we got this far, height `block.Height` must be finalized, because
		// maxHeightForRequesting is lower than the finalized height.

		// Skip result if it is incorporated in a block that is _not_ part of
		// the finalized fork.
		finalizedBlockAtHeight, err := e.headersDB.ByHeight(block.Height)
		if err != nil {
			return 0, fmt.Errorf("could not retrieve finalized block for finalized height %d: %w", block.Height, err)
		}
		if finalizedBlockAtHeight.ID() != r.IncorporatedBlockID {
			// block is in an orphaned fork
			continue
		}

		// Skip results for already-sealed blocks. While such incorporated
		// results will eventually be removed from the mempool, there is a small
		// period, where they might still be in the mempool (until the cleanup
		// algorithm has caught them).
		resultBlock, err := e.headersDB.ByBlockID(r.Result.BlockID)
		if err != nil {
			return 0, fmt.Errorf("could not retrieve block: %w", err)
		}
		if resultBlock.Height <= sealed.Height {
			continue
		}

		// Compute the chunk assigment. Chunk approvals will only be requested
		// from verifiers that were assigned to the chunk. Note that the
		// assigner keeps a cache of computed assignments, so this is not
		// necessarily an expensive operation.
		assignment, err := e.assigner.Assign(r.Result, r.IncorporatedBlockID)
		if err != nil {
			// at this point, we know the block and a valid child block exists.
			// Not being able to compute the assignment constitutes a fatal
			// implementation bug:
			return 0, fmt.Errorf("could not determine chunk assignment: %w", err)
		}

		// send approval requests for chunks that haven't collected enough
		// approvals
		for _, c := range r.Result.Chunks {

			// skip if we already have enough valid approvals for this chunk
			sigs, haveChunkApprovals := r.GetChunkSignatures(c.Index)
			if haveChunkApprovals && uint(sigs.NumberSigners()) >= e.requiredApprovalsForSealConstruction {
				continue
			}

			// Retrieve information about requests made for this chunk. Skip
			// requesting if the blackout period hasn't expired. Otherwise,
			// update request count and reset blackout period.
			requestTrackerItem := e.requestTracker.Get(resultID, c.Index)
			if requestTrackerItem.IsBlackout() {
				continue
			}
			requestTrackerItem.Update()

			// for monitoring/debugging purposes, log requests if we start
			// making more than 10
			if requestTrackerItem.Requests >= 10 {
				e.log.Debug().Msgf("requesting approvals for result %v chunk %d: %d requests",
					resultID,
					c.Index,
					requestTrackerItem.Requests,
				)
			}

			// prepare the request
			req := &messages.ApprovalRequest{
				Nonce:      rand.Uint64(),
				ResultID:   resultID,
				ChunkIndex: c.Index,
			}

			// get the list of verification nodes assigned to this chunk
			assignedVerifiers := assignment.Verifiers(c)

			// keep only the ids of verifiers who haven't provided an approval
			var targetIDs flow.IdentifierList
			if haveChunkApprovals && sigs.NumberSigners() > 0 {
				targetIDs = flow.IdentifierList{}
				for _, id := range assignedVerifiers {
					if sigs.HasSigner(id) {
						targetIDs = append(targetIDs, id)
					}
				}
			} else {
				targetIDs = assignedVerifiers
			}

			// publish the approval request to the network
			requestCount++
			err = e.approvalConduit.Publish(req, targetIDs...)
			if err != nil {
				log.Error().Err(err).
					Hex("chunk_id", logging.Entity(c)).
					Msg("could not publish approval request for chunk")
			}
		}
	}

	return requestCount, nil
}

// OnFinalizedBlock implements the callback from the protocol state to notify a block
// is finalized, it guarantees every finalized block will be called at least once.
func (e *Engine) OnFinalizedBlock(block *model.Block) {
	// we index the execution receipts by the executed block ID only for all finalized blocks
	// that guarantees if we could retrieve the receipt by the index, then the receipts
	// must be for a finalized blocks.
	err := e.indexReceipts(block.BlockID)
	if err != nil {
		// the receipt index is only being used by pending receipts component,
		// which is not critical, no need to crash.
		e.log.Error().Err(err).Hex("block_id", block.BlockID[:]).
			Msg("could not index receipts for block")
	}
}

func (e *Engine) indexReceipts(blockID flow.Identifier) error {
	payload, err := e.payloadsDB.ByBlockID(blockID)

	if err != nil {
		return fmt.Errorf("could not get block payload: %w", err)
	}

	for _, receipt := range payload.Receipts {
		err := e.receiptsDB.IndexByExecutor(receipt)
		if err != nil {
			return fmt.Errorf("could not index receipt by executor, receipt id: %v: %w", receipt.ID(), err)
		}
	}

	return nil
}
