// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package matching

import (
	"errors"
	"fmt"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	chmodule "github.com/dapperlabs/flow-go/module/chunks"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	storerr "github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

var (
	errUnknownBlock    = errors.New("result block unknown")
	errUnknownPrevious = errors.New("previous result unknown")
	errInvalidChunks   = errors.New("invalid chunk number")
)

// Engine is the Matching engine, which builds seals by matching receipts (aka
// ExecutionReceipt, from execution nodes) and approvals (aka ResultApproval,
// from verification nodes), and saves the seals into seals mempool for adding
// into a new block.
type Engine struct {
	unit                    *engine.Unit                    // used to control startup/shutdown
	log                     zerolog.Logger                  // used to log relevant actions with context
	metrics                 module.EngineMetrics            // used to track sent and received messages
	tracer                  module.Tracer                   // used to trace execution
	mempool                 module.MempoolMetrics           // used to track mempool size
	state                   protocol.State                  // used to access the  protocol state
	me                      module.Local                    // used to access local node information
	requester               module.Requester                // used to request missing execution receipts by block ID
	headersDB               storage.Headers                 // used to check sealed headers
	indexDB                 storage.Index                   // used to check payloads for results
	sealsDB                 storage.Seals                   // used to check sealed blocks
	resultsDB               storage.ExecutionResults        // used to check prevous results are known
	incorporatedResults     mempool.IncorporatedResults     // holds execution results in memory
	receipts                mempool.Receipts                // holds execution receipts in memory
	approvals               mempool.Approvals               // holds result approvals in memory
	seals                   mempool.IncorporatedResultSeals // holds block seals in memory
	missing                 map[flow.Identifier]uint        // track how often a block was missing
	assigner                module.ChunkAssigner            // chunk assignment object
	checkingSealing         *atomic.Bool                    // used to rate limit the checksealing call
	requestReceiptThreshold uint                            // how many blocks between sealed/finalized before we request execution receipts
	maxUnsealedResults      int                             // how many unsealed results to check when check sealing
}

// New creates a new collection propagation engine.
func New(
	log zerolog.Logger,
	collector module.EngineMetrics,
	tracer module.Tracer,
	mempool module.MempoolMetrics,
	net module.Network,
	state protocol.State,
	me module.Local,
	requester module.Requester,
	headersDB storage.Headers,
	indexDB storage.Index,
	sealsDB storage.Seals,
	resultsDB storage.ExecutionResults,
	incorporatedResults mempool.IncorporatedResults,
	receipts mempool.Receipts,
	approvals mempool.Approvals,
	seals mempool.IncorporatedResultSeals,
	assigner module.ChunkAssigner,
) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:                    engine.NewUnit(),
		log:                     log.With().Str("engine", "matching").Logger(),
		metrics:                 collector,
		tracer:                  tracer,
		mempool:                 mempool,
		state:                   state,
		me:                      me,
		requester:               requester,
		headersDB:               headersDB,
		indexDB:                 indexDB,
		resultsDB:               resultsDB,
		sealsDB:                 sealsDB,
		incorporatedResults:     incorporatedResults,
		receipts:                receipts,
		approvals:               approvals,
		seals:                   seals,
		missing:                 make(map[flow.Identifier]uint),
		checkingSealing:         atomic.NewBool(false),
		requestReceiptThreshold: 10,
		maxUnsealedResults:      200,
		assigner:                assigner,
	}

	e.mempool.MempoolEntries(metrics.ResourceResult, e.incorporatedResults.Size())
	e.mempool.MempoolEntries(metrics.ResourceReceipt, e.receipts.Size())
	e.mempool.MempoolEntries(metrics.ResourceApproval, e.approvals.Size())
	e.mempool.MempoolEntries(metrics.ResourceSeal, e.seals.Size())

	// register engine with the receipt provider
	_, err := net.Register(engine.ReceiveReceipts, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for receipts: %w", err)
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
		e.metrics.MessageReceived(metrics.EngineMatching, metrics.MessageExecutionReceipt)
		e.unit.Lock()
		defer e.unit.Unlock()
		defer e.metrics.MessageHandled(metrics.EngineMatching, metrics.MessageExecutionReceipt)
		return e.onReceipt(originID, ev)
	case *flow.ResultApproval:
		e.metrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		e.unit.Lock()
		defer e.unit.Unlock()
		defer e.metrics.MessageHandled(metrics.EngineMatching, metrics.MessageResultApproval)
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
		Hex("final_state", receipt.ExecutionResult.FinalStateCommit).
		Logger()

	log.Info().Msg("execution receipt received")

	// check the execution receipt is sent by its executor
	if receipt.ExecutorID != originID {
		return engine.NewInvalidInputErrorf("invalid origin for receipt (executor: %x, origin: %x)", receipt.ExecutorID, originID)
	}

	// get the identity of the origin node, so we can check if it's a valid
	// source for a execution receipt (usually execution nodes)
	identity, err := e.state.Final().Identity(originID)
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

	// check if the corresponding block has already been sealed
	_, err = e.sealsDB.ByBlockID(receipt.ExecutionResult.BlockID)
	if err == nil {
		log.Debug().Msg("skipping receipt for sealed block")
		return nil
	}
	if !errors.Is(err, storerr.ErrNotFound) {
		return fmt.Errorf("could not retrieve seal: %w", err)
	}

	// store the receipt in the memory pool
	added := e.receipts.Add(receipt)
	if !added {
		log.Debug().Msg("skipping receipt already in mempool")
		return nil
	}

	log.Info().Msg("execution receipt added to mempool")

	e.mempool.MempoolEntries(metrics.ResourceReceipt, e.receipts.Size())

	// kick off a check for potential seal formation
	go e.checkSealing()

	return nil
}

// onApproval processes a new result approval.
func (e *Engine) onApproval(originID flow.Identifier, approval *flow.ResultApproval) error {

	log := e.log.With().
		Hex("approval_id", logging.Entity(approval)).
		Hex("result_id", approval.Body.ExecutionResultID[:]).
		Logger()

	log.Info().Msg("result approval received")

	// check approver matches the origin ID
	if approval.Body.ApproverID != originID {
		return engine.NewInvalidInputErrorf("invalid origin for approval: %x", originID)
	}

	// get the identity of the origin node, so we can check if it's a valid
	// source for a result approval (usually verification node)
	identity, err := e.state.Final().Identity(originID)
	if err != nil {
		if protocol.IsIdentityNotFound(err) {
			return engine.NewInvalidInputErrorf("could not get approval identity: %w", err)
		}

		// unknown exception
		return fmt.Errorf("could not get executor identity: %w", err)
	}

	// check that the origin is a verification node
	if identity.Role != flow.RoleVerification {
		return engine.NewInvalidInputErrorf("invalid approver node role (%s)", identity.Role)
	}

	// check if the identity has a stake
	if identity.Stake == 0 {
		return engine.NewInvalidInputErrorf("verifier has zero stake (%x)", identity.NodeID)
	}

	// check if the corresponding block has already been sealed
	_, err = e.sealsDB.ByBlockID(approval.Body.BlockID)
	if err == nil {
		log.Debug().Msg("skipping approval for sealed block")
		return nil

	}
	if !errors.Is(err, storerr.ErrNotFound) {
		return fmt.Errorf("could not retrieve seal: %w", err)
	}

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
	go e.checkSealing()

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
	matchedResults, err := e.matchedResults()
	if err != nil {
		e.log.Error().Err(err).Msg("could not get sealable execution results")
		return
	}

	// skip if no results can be sealed yet
	if len(matchedResults) == 0 {
		return
	}

	// don't overflow the seal mempool
	space := e.seals.Limit() - e.seals.Size()
	if len(matchedResults) > int(space) {
		e.log.Debug().
			Int("space", int(space)).
			Int("results", len(matchedResults)).
			Msg("cut and return the first x results")
		matchedResults = matchedResults[:space]
	}

	e.log.Info().Int("num_results", len(matchedResults)).Msg("identified sealable execution results")

	// Start spans for tracing within the parent spans trace.CONProcessBlock and
	// trace.CONProcessCollection
	for _, result := range matchedResults {
		// For each execution result, we load the trace.CONProcessBlock span for the executed block. If we find it, we
		// start a child span that will run until this function returns.
		if span, ok := e.tracer.GetSpan(result.Result.BlockID, trace.CONProcessBlock); ok {
			childSpan := e.tracer.StartSpanFromParent(span, trace.CONMatchCheckSealing, opentracing.StartTime(
				start))
			defer childSpan.Finish()
		}

		// For each execution result, we load all the collection that are in the executed block.
		index, err := e.indexDB.ByBlockID(result.Result.BlockID)
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
	for _, result := range matchedResults {

		log := e.log.With().
			Hex("result_id", logging.Entity(result.Result)).
			Hex("previous_id", result.Result.PreviousResultID[:]).
			Hex("block_id", result.Result.BlockID[:]).
			Logger()

		err := e.sealResult(result)
		if err == errInvalidChunks {
			_ = e.incorporatedResults.Rem(result.ID())
			log.Warn().Msg("removing matched result with invalid number of chunks")
			continue
		}
		if err == errUnknownBlock {
			log.Debug().Msg("skipping sealable result with unknown sealed block")
			continue
		}
		if err == errUnknownPrevious {
			log.Debug().Msg("skipping sealable result with unknown previous result")
			continue
		}
		if err != nil {
			log.Error().Err(err).Msg("could not seal result")
			continue
		}

		// mark the result cleared for mempool cleanup
		sealedResultIDs = append(sealedResultIDs, result.ID())
		sealedBlockIDs = append(sealedBlockIDs, result.Result.BlockID)
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
}

// matchedResults returns the ExecutionResults from the mempool that have
// collected enough approvals on a per-chunk basis, as defined by the matchChunk
// function.
func (e *Engine) matchedResults() ([]*flow.IncorporatedResult, error) {
	var results []*flow.IncorporatedResult

	// go through the results mempool and check which ones we have collected
	// enough approvals
	for _, result := range e.incorporatedResults.All() {
		// check that each chunk collected enough approvals
		allChunksMatched := true

		// The assignment is calculated based on the first block in its fork
		// which references an Execution Receipt for this result.
		assignment, err := e.assigner.Assign(result.Result, result.IncorporatedBlockID)
		if err != nil {
			return nil, fmt.Errorf("could assign verifiers: %w", err)
		}

		// check that each chunk collects enough approvals
		for _, chunk := range result.Result.Chunks {
			matched := e.matchChunk(result.Result.ID(), chunk, assignment)
			if !matched {
				allChunksMatched = false
				break
			}
		}

		// add the result to the results that should be sealed
		if allChunksMatched {
			e.log.Info().Msg("adding result with sufficient verification")
			results = append(results, result)
		}
	}

	return results, nil
}

// matchChunk checks that the number of ResultApprovals collected by a chunk
// exceeds the required threshold.
func (e *Engine) matchChunk(resultID flow.Identifier, chunk *flow.Chunk, assignment *chunks.Assignment) bool {

	// get all the chunk approvals from mempool
	approvals, ok := e.approvals.ByChunk(resultID, chunk.Index)
	if !ok {
		return false
	}

	// only keep approvals from assigned verifiers
	var validApprovers flow.IdentifierList
	for approverID := range approvals {
		ok := chmodule.IsValidVerifer(assignment, chunk, approverID)
		if ok {
			validApprovers = append(validApprovers, approverID)
		}
	}

	// TODO:
	//  * This is the happy path (requires just one approval per chunk). Should
	//    be +2/3 of all currently staked verifiers.
	return len(validApprovers) > 0
}

// sealResult creates a seal for the incorporated result and adds it to the
// seals mempool.
func (e *Engine) sealResult(result *flow.IncorporatedResult) error {

	// we create one chunk per collection (at least for now), so we can check if
	// the chunk number matches with the number of guarantees; this will ensure
	// the execution receipt can not lie about having less chunks and having the
	// remaining ones approved
	index, err := e.indexDB.ByBlockID(result.Result.BlockID)
	if errors.Is(err, storage.ErrNotFound) {
		// if we have not received the block yet, we will just keep
		// rechecking until the block has been received or the result has
		// been purged
		return errUnknownBlock
	}
	if err != nil {
		return fmt.Errorf("could not retrieve payload index: %w", err)
	}
	if len(result.Result.Chunks) != len(index.CollectionIDs) {
		return errInvalidChunks
	}

	// ensure that previous result is known. The builder will ensure that the
	// seals are correctly chained.
	previousID := result.Result.PreviousResultID
	previous, err := e.resultsDB.ByID(previousID)
	if err != nil {
		return errUnknownPrevious
	}

	// generate and submit the seal to the mempool
	seal := &flow.Seal{
		BlockID:      result.Result.BlockID,
		ResultID:     result.ID(),
		InitialState: previous.FinalStateCommit,
		FinalState:   result.Result.FinalStateCommit,
	}

	// we don't care whether the seal is already in the mempool
	_ = e.seals.Add(&flow.IncorporatedResultSeal{
		IncorporatedResult: result,
		Seal:               seal,
	})

	// store the result to make it persistent for later checks
	err = e.resultsDB.Store(result.Result)
	if err != nil && !errors.Is(err, storage.ErrAlreadyExists) {
		return fmt.Errorf("could not store sealing result: %w", err)
	}

	return nil
}

// clearPools clears the memory pools of all entities related to blocks that are
// already sealed. If we don't know the block, we purge the entities once we
// have sealed a further 100 blocks without seeing the block (it's probably no
// longer a valid extension of the state anyway).
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
		if e.missing[blockID] >= 100 {
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
		if clear[result.Result.ID()] || shouldClear(result.Result.BlockID) {
			_ = e.incorporatedResults.Rem(result.ID())
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
			e.approvals.Rem(approval.Body.ExecutionResultID, approval.Body.ChunkIndex)
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
		if count >= 100 || err == nil {
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

		// check if we have any incorporated execution results for the block at
		// this height
		items := e.incorporatedResults.ByIncorporatedBlockID(blockID)
		if len(items) == 0 {
			missingBlocksOrderedByHeight = append(missingBlocksOrderedByHeight, blockID)
		}
	}

	e.log.Info().
		Uint64("final", final.Height).
		Uint64("sealed", sealed.Height).
		Uint("request_receipt_threshold", e.requestReceiptThreshold).
		Int("missing", len(missingBlocksOrderedByHeight)).
		Msg("check missing receipts")

	// request missing execution results, if sealed height is low enough
	if uint(final.Height-sealed.Height) >= e.requestReceiptThreshold {
		requestedCount := 0
		for _, blockID := range missingBlocksOrderedByHeight {
			e.requester.EntityByID(blockID, filter.Any)
		}
		e.log.Info().
			Int("count", requestedCount).
			Msg("requested missing results")
	}

	return nil
}
