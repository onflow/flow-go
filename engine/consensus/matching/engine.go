// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package matching

import (
	"errors"
	"fmt"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	chmodule "github.com/onflow/flow-go/module/chunks"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

var ErrBlockNotFound = errors.New("block not found")

// Engine is the Matching engine, which builds seals by matching receipts (aka
// ExecutionReceipt, from execution nodes) and approvals (aka ResultApproval,
// from verification nodes), and saves the seals into seals mempool for adding
// into a new block.
type Engine struct {
	unit                    *engine.Unit                    // used to control startup/shutdown
	log                     zerolog.Logger                  // used to log relevant actions with context
	engineMetrics           module.EngineMetrics            // used to track sent and received messages
	tracer                  module.Tracer                   // used to trace execution
	mempool                 module.MempoolMetrics           // used to track mempool size
	metrics                 module.ConsensusMetrics         // used to track consensus metrics
	state                   protocol.State                  // used to access the  protocol state
	me                      module.Local                    // used to access local node information
	requester               module.Requester                // used to request missing execution receipts by block ID
	resultsDB               storage.ExecutionResults        // used to check previous results are known
	headersDB               storage.Headers                 // used to check sealed headers
	indexDB                 storage.Index                   // used to check payloads for results
	incorporatedResults     mempool.IncorporatedResults     // holds incorporated results in memory
	receipts                mempool.Receipts                // holds execution receipts in memory
	approvals               mempool.Approvals               // holds result approvals in memory
	seals                   mempool.IncorporatedResultSeals // holds the seals that were produced by the matching engine
	missing                 map[flow.Identifier]uint        // track how often a block was missing
	assigner                module.ChunkAssigner            // chunk assignment object
	checkingSealing         *atomic.Bool                    // used to rate limit the checksealing call
	requestReceiptThreshold uint                            // how many blocks between sealed/finalized before we request execution receipts
	maxUnsealedResults      int                             // how many unsealed results to check when check sealing
	requireApprovals        bool                            // flag to disable verifying chunk approvals
}

// New creates a new collection propagation engine.
func New(
	log zerolog.Logger,
	engineMetrics module.EngineMetrics,
	tracer module.Tracer,
	mempool module.MempoolMetrics,
	conMetrics module.ConsensusMetrics,
	net module.Network,
	state protocol.State,
	me module.Local,
	requester module.Requester,
	resultsDB storage.ExecutionResults,
	headersDB storage.Headers,
	indexDB storage.Index,
	incorporatedResults mempool.IncorporatedResults,
	receipts mempool.Receipts,
	approvals mempool.Approvals,
	seals mempool.IncorporatedResultSeals,
	assigner module.ChunkAssigner,
	requireApprovals bool,
) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:                    engine.NewUnit(),
		log:                     log.With().Str("engine", "matching").Logger(),
		engineMetrics:           engineMetrics,
		tracer:                  tracer,
		mempool:                 mempool,
		metrics:                 conMetrics,
		state:                   state,
		me:                      me,
		requester:               requester,
		resultsDB:               resultsDB,
		headersDB:               headersDB,
		indexDB:                 indexDB,
		incorporatedResults:     incorporatedResults,
		receipts:                receipts,
		approvals:               approvals,
		seals:                   seals,
		missing:                 make(map[flow.Identifier]uint),
		checkingSealing:         atomic.NewBool(false),
		requestReceiptThreshold: 10,
		maxUnsealedResults:      200,
		assigner:                assigner,
		requireApprovals:        requireApprovals,
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

// HandleReceipt pipes explicitely requested receipts to the process function.
// Receipts can come from this function or the receipt provider setup in the
// engine constructor.
func (e *Engine) HandleReceipt(originID flow.Identifier, receipt flow.Entity) {

	e.log.Debug().Msg("received receipt from requester engine")

	err := e.process(originID, receipt)
	if err != nil {
		e.log.Error().Err(err).Msg("could not process receipt")
	}
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
		e.unit.Lock()
		defer e.unit.Unlock()
		defer e.engineMetrics.MessageHandled(metrics.EngineMatching, metrics.MessageResultApproval)
		return e.onApproval(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// onReceipt processes a new execution receipt.
func (e *Engine) onReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {
	log := e.log.With().
		Hex("origin_id", originID[:]).
		Hex("receipt_id", logging.Entity(receipt)).
		Hex("result_id", logging.Entity(receipt.ExecutionResult)).
		Hex("previous_id", receipt.ExecutionResult.PreviousResultID[:]).
		Hex("block_id", receipt.ExecutionResult.BlockID[:]).
		Hex("executor_id", receipt.ExecutorID[:]).
		Logger()

	resultFinalState, ok := receipt.ExecutionResult.FinalStateCommitment()
	if !ok {
		return fmt.Errorf("could not get final state: no chunks found")
	}

	log = log.With().Hex("final_state", resultFinalState).Logger()

	log.Info().Msg("execution receipt received")

	// check the execution receipt is sent by its executor
	if receipt.ExecutorID != originID {
		return engine.NewInvalidInputErrorf("invalid origin for receipt (executor: %x, origin: %x)", receipt.ExecutorID, originID)
	}

	// if the receipt is for an unknown block, skip it. It will be re-requested
	// later.
	header, err := e.headersDB.ByBlockID(receipt.ExecutionResult.BlockID)
	if err != nil {
		log.Debug().Msg("discarding receipt for unknown block")
		return nil
	}

	lastSealed, err := e.headersDB.GetLastSealed()
	if err != nil {
		return err
	}

	if header.Height <= lastSealed.Height {
		log.Debug().Msg("discarding receipt for sealed block")
		return nil
	}

	// get the identity of the origin node, so we can check if it's a valid
	// source for a execution receipt (usually execution nodes)
	identity, err := e.state.AtBlockID(receipt.ExecutionResult.BlockID).Identity(originID)
	if err != nil {
		if protocol.IsIdentityNotFound(err) {
			return engine.NewInvalidInputErrorf("could not get executor identity: %w", err)
		}

		// unknown exception
		return fmt.Errorf("could not get executor identity: %w", err)
	}

	// check that the origin is an execution node
	if identity.Role != flow.RoleExecution {
		return engine.NewInvalidInputErrorf("invalid executor node role (%s)", identity.Role)
	}

	// check if the identity has a stake
	if identity.Stake == 0 {
		return engine.NewInvalidInputErrorf("executor has zero stake (%x)", identity.NodeID)
	}

	// add the receipt to the mempool so that the builder can incorporate it in
	// a block payload
	added := e.receipts.Add(receipt)
	if !added {
		log.Debug().Msg("skipping receipt already in mempool")
		return nil
	}

	log.Info().Msg("execution receipt added to mempool")
	e.mempool.MempoolEntries(metrics.ResourceReceipt, e.receipts.Size())

	// store the result belonging to the receipt in the memory pool
	//
	// ATTENTION:
	//
	// In phase 2, we artificially create IncorporatedResults from incoming
	// receipts and set the IncorporatedBlockID to the result's block ID.
	//
	// In phase 3, the incorporated results mempool will be populated by the
	// finalizer when blocks are added to the chain, and the IncorporatedBlockID
	// will be the ID of the first block on its fork that contains a receipt
	// committing to this result.
	added, err = e.incorporatedResults.Add(
		flow.NewIncorporatedResult(
			receipt.ExecutionResult.BlockID,
			&receipt.ExecutionResult,
		),
	)
	if err != nil {
		e.log.Err(err).Msg("error inserting incorporated result in mempool")
	}
	if !added {
		e.log.Debug().Msg("skipping result already in mempool")
		return nil
	}

	e.mempool.MempoolEntries(metrics.ResourceResult, e.incorporatedResults.Size())
	e.log.Info().Msg("execution result added to mempool")

	// kick off a check for potential seal formation
	e.unit.Launch(e.checkSealing)

	return nil
}

// onApproval processes a new result approval.
func (e *Engine) onApproval(originID flow.Identifier, approval *flow.ResultApproval) error {

	log := e.log.With().
		Hex("approval_id", logging.Entity(approval)).
		Hex("block_id", approval.Body.BlockID[:]).
		Hex("result_id", approval.Body.ExecutionResultID[:]).
		Logger()

	log.Info().Msg("result approval received")

	// check approver matches the origin ID
	if approval.Body.ApproverID != originID {
		return engine.NewInvalidInputErrorf("invalid origin for approval: %x", originID)
	}

	// if the approval is for an unknown block, cache it. It will be picked up
	// later when the finalizer processes new blocks.
	header, err := e.headersDB.ByBlockID(approval.Body.BlockID)
	if err != nil {
		_, _ = e.approvals.Add(approval)
		return nil
	}

	lastSealed, err := e.headersDB.GetLastSealed()
	if err != nil {
		return err
	}

	if header.Height <= lastSealed.Height {
		log.Debug().Msg("discarding receipt for sealed block")
		return nil
	}

	// Check if the approver was a staked verifier at that block. Don't error
	// out if the block is not known yet, because this method will be called
	// again for some house cleaning when we try to match approvals to chunks.
	err = e.checkApproverIsStakedVerifier(approval.Body.ApproverID, approval.Body.BlockID)
	if err != nil {
		// don't error out if the block was not found yet
		if !errors.Is(err, ErrBlockNotFound) {
			return err
		}
	}

	// TODO: check the approval's cryptographic integrity

	// store in the memory pool (it won't be added if it is already in there).
	added, err := e.approvals.Add(approval)
	if err != nil {
		return err
	}

	if !added {
		e.log.Debug().Msg("skipping approval already in mempool")
		return nil
	}

	e.mempool.MempoolEntries(metrics.ResourceApproval, e.approvals.Size())

	// kick off a check for potential seal formation
	e.unit.Launch(e.checkSealing)

	return nil
}

// checkSealing checks if there is anything worth sealing at the moment.
func (e *Engine) checkSealing() {
	// rate limit the check sealing
	if e.checkingSealing.Load() {
		return
	}

	e.unit.Lock()
	defer e.unit.Unlock()

	// only check sealing when no one else is checking
	canCheck := e.checkingSealing.CAS(false, true)
	if !canCheck {
		return
	}

	defer e.checkingSealing.Store(false)

	start := time.Now()

	// get all results that have collected enough approvals on a per-chunk basis
	sealableResults, err := e.sealableResults()
	if err != nil {
		e.log.Error().Err(err).Msg("could not get sealable execution results")
		return
	}

	// skip if no results can be sealed yet
	if len(sealableResults) == 0 {
		return
	}

	// don't overflow the seal mempool
	space := e.seals.Limit() - e.seals.Size()
	if len(sealableResults) > int(space) {
		e.log.Debug().
			Int("space", int(space)).
			Int("results", len(sealableResults)).
			Msg("cut and return the first x results")
		sealableResults = sealableResults[:space]
	}

	e.log.Info().Int("num_results", len(sealableResults)).Msg("identified sealable execution results")

	// Start spans for tracing within the parent spans trace.CONProcessBlock and
	// trace.CONProcessCollection
	for _, incorporatedResult := range sealableResults {
		// For each execution result, we load the trace.CONProcessBlock span for the executed block. If we find it, we
		// start a child span that will run until this function returns.
		if span, ok := e.tracer.GetSpan(incorporatedResult.Result.BlockID, trace.CONProcessBlock); ok {
			childSpan := e.tracer.StartSpanFromParent(span, trace.CONMatchCheckSealing, opentracing.StartTime(
				start))
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
				childSpan := e.tracer.StartSpanFromParent(span, trace.CONMatchCheckSealing, opentracing.StartTime(
					start))
				defer childSpan.Finish()
			}
		}
	}

	// seal the matched results
	var sealedResultIDs []flow.Identifier
	var sealedBlockIDs []flow.Identifier
	for _, incorporatedResult := range sealableResults {

		log := e.log.With().
			Hex("result_id", logging.Entity(incorporatedResult)).
			Hex("previous_id", incorporatedResult.Result.PreviousResultID[:]).
			Hex("block_id", incorporatedResult.Result.BlockID[:]).
			Hex("incorporated_block_id", incorporatedResult.IncorporatedBlockID[:]).
			Logger()

		err := e.sealResult(incorporatedResult)
		if err != nil {
			log.Error().Err(err).Msg("could not seal result")
			continue
		}

		// mark the result cleared for mempool cleanup
		sealedResultIDs = append(sealedResultIDs, incorporatedResult.ID())
		sealedBlockIDs = append(sealedBlockIDs, incorporatedResult.Result.BlockID)
	}

	e.log.Debug().Int("sealed", len(sealedResultIDs)).Msg("sealed execution results")

	// clear the memory pools
	e.clearPools(sealedResultIDs)

	// finish tracing spans
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

	// request execution receipts for unsealed finalized blocks
	err = e.requestPending()
	if err != nil {
		e.log.Error().Err(err).Msg("could not request pending block results")
	}

	// record duration of check sealing
	e.metrics.CheckSealingDuration(time.Since(start))
}

// sealableResults returns the IncorporatedResults from the mempool that have
// collected enough approvals on a per-chunk basis, as defined by the matchChunk
// function. It also filters out results that have an incorrect sub-graph.
func (e *Engine) sealableResults() ([]*flow.IncorporatedResult, error) {
	var results []*flow.IncorporatedResult

	// go through the results mempool and check which ones we have collected
	// enough approvals for
RES_LOOP:
	for _, incorporatedResult := range e.incorporatedResults.All() {

		// TODO: In phase 3, when incorporated results are produced by the
		// finalizer, the result's block and previous result should be found in
		// storage. Any error in retrieving them is unexpected.

		// if we have not received the block yet, we will just keep rechecking
		// until the block has been received or the result has been purged.
		block, err := e.headersDB.ByBlockID(incorporatedResult.Result.BlockID)
		if errors.Is(err, storage.ErrNotFound) {
			log.Debug().Msg("skipping result with unknown block")
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not retrieve block: %w", err)
		}

		// look for previous result in mempool and storage (it could have been
		// inserted in storage as part of a block payload).
		previousID := incorporatedResult.Result.PreviousResultID
		previous, _ := e.incorporatedResults.ByResultID(previousID)
		if previous == nil {
			var err error
			previous, err = e.resultsDB.ByID(previousID)
			if errors.Is(err, storage.ErrNotFound) {
				log.Debug().Msg("skipping sealable result with unknown previous result")
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("could not get previous result: %w", err)
			}
		}

		// check sub-graph
		if block.ParentID != previous.BlockID {
			_ = e.incorporatedResults.Rem(incorporatedResult)
			log.Warn().
				Str("block_parent_id", block.ParentID.String()).
				Str("previous_result_block_id", previous.BlockID.String()).
				Msg("removing result with invalid sub-graph")
			continue
		}

		// we create one chunk per collection (at least for now), plus the
		// system chunk. so we can check if the chunk number matches with the
		// number of guarantees plus one; this will ensure the execution receipt
		// cannot lie about having less chunks and having the remaining ones
		// approved
		requiredChunks := 0

		index, err := e.indexDB.ByBlockID(incorporatedResult.Result.BlockID)
		if err != nil {
			// the block could have no payload
			if !errors.Is(err, storage.ErrNotFound) {
				return nil, err
			}
		} else {
			requiredChunks = len(index.CollectionIDs) + 1
		}

		if incorporatedResult.Result.Chunks.Len() != requiredChunks {
			_ = e.incorporatedResults.Rem(incorporatedResult)
			log.Warn().
				Int("result_chunks", len(incorporatedResult.Result.Chunks)).
				Int("required_chunks", requiredChunks).
				Msg("removing result with invalid number of chunks")
			continue
		}

		// check that each chunk collected enough approvals
		allChunksMatched := true

		// the chunk assigment is based on the first block in its fork which
		// contains a receipt that commits to this result.
		assignment, err := e.assigner.Assign(incorporatedResult.Result, incorporatedResult.IncorporatedBlockID)
		if state.IsNoValidChildBlockError(err) {
			continue
		}
		if err != nil {
			log.Warn().Msgf("could not compute chunk assignment: %v", err)
			continue
		}

		// check that each chunk collects enough approvals
		for i := 0; i < assignment.Len(); i++ {
			// arriving at a failure condition here means that the execution
			// result is invalid; we should skip it and move on to the next
			// execution result.

			// get chunk at position i
			chunk, ok := incorporatedResult.Result.Chunks.ByIndex(uint64(i))
			if !ok {
				log.Warn().Msgf("chunk out of range requested: %d", i)
				continue RES_LOOP
			}

			// Check if chunk index matches its position. This ensures that the
			// result contains all chunks and no duplicates.
			if chunk.Index != uint64(i) {
				log.Warn().Msgf("chunk out of place: pos = %d, index = %d", i, chunk.Index)
				continue RES_LOOP
			}

			matched := e.matchChunk(incorporatedResult, chunk, assignment)
			if !matched {
				allChunksMatched = false
				break
			}
		}

		// add the result to the results that should be sealed
		if allChunksMatched {
			e.log.Info().Msg("adding result with sufficient verification")
			results = append(results, incorporatedResult)
		}
	}

	return results, nil
}

// matchChunk checks that the number of ResultApprovals collected by a chunk
// exceeds the required threshold. It also populates the IncorporatedResult's
// collection of approval signatures to avoid repeated work.
func (e *Engine) matchChunk(incorporatedResult *flow.IncorporatedResult, chunk *flow.Chunk, assignment *chunks.Assignment) bool {

	// get all the chunk approvals from mempool
	approvals := e.approvals.ByChunk(incorporatedResult.Result.ID(), chunk.Index)

	validApprovals := 0
	for approverID, approval := range approvals {
		// skip if the incorporated result already has a signature for that
		// chunk and verifier
		_, ok := incorporatedResult.GetSignature(chunk.Index, approverID)
		if ok {
			validApprovals++
			continue
		}

		// check if the approver is assigned to this chunk.
		ok = chmodule.IsValidVerifer(assignment, chunk, approverID)
		if !ok {
			// if the approval comes from a node that wasn't even a staked
			// verifier at that block, remove the approval from the mempool.
			err := e.checkApproverIsStakedVerifier(approverID, incorporatedResult.Result.BlockID)
			if err != nil {
				// don't remove the approval if the error indicates that the
				// block is not known yet.
				if !errors.Is(err, ErrBlockNotFound) {
					_, _ = e.approvals.RemApproval(approval)
				}
			}
			continue
		}

		// Add signature to incorporated result so that we don't have to check
		// it again.
		incorporatedResult.AddSignature(chunk.Index, approverID, approval.Body.AttestationSignature)

		validApprovals++
	}

	// skip counting approvals. We don't put this earlier in the function
	// because we still want to test that the above code doesn't panic.
	// TODO: this is only here temporarily to ease the migration to new chunk
	// based sealing.
	if !e.requireApprovals {
		return true
	}

	// TODO:
	//   * This is the happy path (requires just one approval per chunk).
	//   * Full protocol should be +2/3 of all currently staked verifiers.
	return validApprovals > 0
}

// checkApproverIsStakedVerifier checks if the approver was a valid staked
// verifier at a given block, and returns an error if it wasn't, or if the block
// is not known yet. If the block is not known yet, it returns a
// ErrBlockNotFound sentinel error.
func (e *Engine) checkApproverIsStakedVerifier(approverID flow.Identifier, blockID flow.Identifier) error {

	// if the approval is for an unknown block, cache it. It will be picked up
	// later when the finalizer processes new blocks.
	_, err := e.headersDB.ByBlockID(blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrBlockNotFound
		}
		return err
	}

	// get the identity of the origin node
	identity, err := e.state.AtBlockID(blockID).Identity(approverID)
	if err != nil {
		if protocol.IsIdentityNotFound(err) {
			return engine.NewInvalidInputErrorf("could not get approver identity: %w", err)
		}

		// unknown exception
		return fmt.Errorf("could not get approver identity: %w", err)
	}

	// check that the origin is a verification node
	if identity.Role != flow.RoleVerification {
		return engine.NewInvalidInputErrorf("invalid approver node role (%s)", identity.Role)
	}

	// check if the identity has a stake
	if identity.Stake == 0 {
		return engine.NewInvalidInputErrorf("verifier has zero stake (%x)", identity.NodeID)
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
		return fmt.Errorf("could not get final state: no chunks found")
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
	_ = e.seals.Add(&flow.IncorporatedResultSeal{
		IncorporatedResult: incorporatedResult,
		Seal:               seal,
	})

	return nil
}

// clearPools clears the memory pools of all entities related to blocks that are
// already sealed. If we don't know the block, we purge the entities once we
// have called checkSealing 1000 times without seeing the block (it's probably
// no longer a valid extension of the state anyway).
func (e *Engine) clearPools(sealedIDs []flow.Identifier) {

	clear := make(map[flow.Identifier]bool)
	for _, sealedID := range sealedIDs {
		clear[sealedID] = true
	}

	sealed, err := e.state.Sealed().Head()
	if err != nil {
		e.log.Error().Err(err).Msg("could not get sealed head")
		return
	}

	// build a helper function that determines if an entity should be cleared
	// if it references the block with the given ID
	missingIDs := make(map[flow.Identifier]bool) // count each missing block only once
	shouldClear := func(blockID flow.Identifier) bool {
		if e.missing[blockID] >= 1000 {
			return true // clear if block is missing for 100 seals already
		}
		header, err := e.headersDB.ByBlockID(blockID)
		if errors.Is(err, storage.ErrNotFound) {
			missingIDs[blockID] = true
			return false // keep if the block is missing, but count times missing
		}
		if err != nil {
			e.log.Error().Err(err).Msg("could not check block expiry")
			return false // keep if we can't check the DB for the referenced block
		}
		if header.Height <= sealed.Height {
			return true // clear if sealed block is same or higher than referenced block
		}
		return false
	}

	// for each memory pool, clear if the related block is no longer relevant or
	// if the seal was already built for it (except for seals themselves)
	for _, result := range e.incorporatedResults.All() {
		if clear[result.ID()] || shouldClear(result.Result.BlockID) {
			_ = e.incorporatedResults.Rem(result)
		}
	}
	for _, receipt := range e.receipts.All() {
		if clear[receipt.ExecutionResult.ID()] || shouldClear(receipt.ExecutionResult.BlockID) {
			_ = e.receipts.Rem(receipt.ID())
		}
	}
	for _, approval := range e.approvals.All() {
		if clear[approval.Body.ExecutionResultID] || shouldClear(approval.Body.BlockID) {
			// delete all the approvals for the corresponding chunk
			_, _ = e.approvals.RemChunk(approval.Body.ExecutionResultID, approval.Body.ChunkIndex)
		}
	}
	for _, seal := range e.seals.All() {
		if shouldClear(seal.Seal.BlockID) {
			_ = e.seals.Rem(seal.ID())
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
}

// requestPending requests the execution receipts of unsealed finalized blocks.
func (e *Engine) requestPending() error {

	// last sealed block
	sealed, err := e.state.Sealed().Head()
	if err != nil {
		return fmt.Errorf("could not get sealed height: %w", err)
	}

	// last finalized block
	final, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get finalized height: %w", err)
	}

	// only request if number of unsealed finalized blocks exceeds the threshold
	if uint(final.Height-sealed.Height) < e.requestReceiptThreshold {
		return nil
	}

	// order the missing blocks by height from low to high such that when
	// passing them to the missing block requester, they can be requested in the
	// right order. The right order gives the priority to the execution result
	// of lower height blocks to be requested first, since a gap in the sealing
	// heights would stop the sealing.
	missingBlocksOrderedByHeight := make([]flow.Identifier, 0, e.maxUnsealedResults)

	// traverse each unsealed and finalized block with height from low to high,
	// if the result is missing, then add the blockID to a missing block list in
	// order to request them.
	for height := sealed.Height; height < final.Height; height++ {
		// add at most <maxUnsealedResults> number of results
		if len(missingBlocksOrderedByHeight) >= int(e.maxUnsealedResults) {
			break
		}

		// get the block header at this height
		header, err := e.headersDB.ByHeight(height)
		if err != nil {
			// should never happen
			return fmt.Errorf("could not get header (height=%d): %w", height, err)
		}

		blockID := header.ID()

		// Check if we have a receipt for this block
		// TODO, implement an index
		notFound := true
		for _, r := range e.receipts.All() {
			if r.ExecutionResult.BlockID == blockID {
				notFound = false
				break
			}
		}
		if notFound {
			missingBlocksOrderedByHeight = append(missingBlocksOrderedByHeight, blockID)
		}
	}

	e.log.Info().
		Uint64("final", final.Height).
		Uint64("sealed", sealed.Height).
		Uint("request_receipt_threshold", e.requestReceiptThreshold).
		Int("missing", len(missingBlocksOrderedByHeight)).
		Msg("check missing receipts")

	requestedCount := 0
	for _, blockID := range missingBlocksOrderedByHeight {
		e.requester.EntityByID(blockID, filter.Any)
		requestedCount++
	}

	e.log.Info().
		Int("count", requestedCount).
		Msg("requested missing results")

	return nil
}
