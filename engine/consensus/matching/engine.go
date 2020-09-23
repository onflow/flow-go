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

var (
	errUnknownBlock    = errors.New("result block unknown")
	errUnknownPrevious = errors.New("previous result unknown")
)

// Engine is the matching engine, which matches execution receipts with result
// approvals to create block seals.
type Engine struct {
	unit            *engine.Unit         // used to control startup/shutdown
	log             zerolog.Logger       // used to log relevant actions with context
	metrics         module.EngineMetrics // used to track sent and received messages
	tracer          module.Tracer
	mempool         module.MempoolMetrics    // used to track mempool size
	state           protocol.State           // used to access the  protocol state
	me              module.Local             // used to access local node information
	requester       module.Requester         // used to request missing execution receipts by block ID
	resultsDB       storage.ExecutionResults // used to permanently store results
	sealsDB         storage.Seals            // used to check existing seals
	headersDB       storage.Headers          // used to check sealed headers
	indexDB         storage.Index
	results         mempool.Results          // holds execution results in memory
	receipts        mempool.Receipts         // holds execution receipts in memory
	approvals       mempool.Approvals        // holds result approvals in memory
	seals           mempool.Seals            // holds block seals in memory
	missing         map[flow.Identifier]uint // track how often a block was missing
	checkingSealing *atomic.Bool             // used to rate limit the checksealing call

	// how many blocks between sealed/finalized before we request execcution receipts
	requestReceiptThreshold uint
	// how many unsealed results to check when check sealing
	maxUnsealedResults int
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
	resultsDB storage.ExecutionResults,
	sealsDB storage.Seals,
	headersDB storage.Headers,
	indexDB storage.Index,
	results mempool.Results,
	receipts mempool.Receipts,
	approvals mempool.Approvals,
	seals mempool.Seals,
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
		resultsDB:               resultsDB,
		sealsDB:                 sealsDB,
		headersDB:               headersDB,
		indexDB:                 indexDB,
		results:                 results,
		receipts:                receipts,
		approvals:               approvals,
		seals:                   seals,
		missing:                 make(map[flow.Identifier]uint),
		checkingSealing:         atomic.NewBool(false),
		requestReceiptThreshold: 10,
		maxUnsealedResults:      200,
	}

	e.mempool.MempoolEntries(metrics.ResourceResult, e.results.Size())
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

// HandleReceipts handles receipts we have explicitly requested by block ID.
func (e *Engine) HandleReceipt(originID flow.Identifier, receipt flow.Entity) {

	e.log.Debug().Msg("received receipt from requester engine")

	err := e.process(originID, receipt)
	if err != nil {
		e.log.Error().Err(err).Msg("could not process receipt")
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

	// check if the result of this receipt is already in the DB
	result := &receipt.ExecutionResult
	_, err = e.resultsDB.ByID(result.ID())
	if err == nil {
		log.Debug().Msg("discarding receipt for sealed result")
		return nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not check result: %w", err)
	}

	// store the receipt in the memory pool
	added := e.receipts.Add(receipt)
	if !added {
		log.Debug().Msg("skipping receipt already in mempool")
		return nil
	}

	log.Info().Msg("execution receipt added to mempool")

	e.mempool.MempoolEntries(metrics.ResourceReceipt, e.receipts.Size())

	// store the result belonging to the receipt in the memory pool
	added = e.results.Add(result)
	if !added {
		log.Debug().Msg("skipping result already in mempool")
		return nil
	}

	e.mempool.MempoolEntries(metrics.ResourceResult, e.results.Size())

	log.Info().Msg("execution result added to mempool")

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

	// check if the result of this approval is already in the dB
	_, err = e.resultsDB.ByID(approval.Body.ExecutionResultID)
	if err == nil {
		log.Debug().Msg("discarding approval for sealed result")
		return nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not check result: %w", err)
	}

	// store in the memory pool
	added := e.approvals.Add(approval)
	if !added {
		log.Debug().Msg("skipping approval already in mempool")
		return nil
	}

	e.mempool.MempoolEntries(metrics.ResourceApproval, e.approvals.Size())

	log.Info().Msg("result approval added to mempool")

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

	// get all results that are sealable
	results, err := e.sealableResults()
	if err != nil {
		e.log.Error().Err(err).Msg("could not get sealable execution results")
		return
	}

	// skip if no results can be sealed yet
	if len(results) == 0 {
		return
	}

	// don't overflow the seal mempool
	space := e.seals.Limit() - e.seals.Size()
	if len(results) > int(space) {
		e.log.Debug().Int("space", int(space)).Int("results", len(results)).Msg("cut and return the first x results")
		results = results[:space]
	}

	e.log.Info().Int("num_results", len(results)).Msg("identified sealable execution results")

	// Start spans for tracing within the parent spans trace.CONProcessBlock and trace.CONProcessCollection
	for _, result := range results {
		// For each execution result, we load the trace.CONProcessBlock span for the executed block. If we find it, we
		// start a child span that will run until this function returns.
		if span, ok := e.tracer.GetSpan(result.BlockID, trace.CONProcessBlock); ok {
			childSpan := e.tracer.StartSpanFromParent(span, trace.CONMatchCheckSealing, opentracing.StartTime(
				start))
			defer childSpan.Finish()
		}

		// For each execution result, we load all the collection that are in the executed block.
		index, err := e.indexDB.ByBlockID(result.BlockID)
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

	// process the results

	final, err := e.state.Final().Head()
	if err != nil {
		e.log.Error().Err(err).Msg("could not get finalized header")
		return
	}

	var sealedResultIDs []flow.Identifier
	var sealedBlockIDs []flow.Identifier
	for _, result := range results {

		log := e.log.With().
			Hex("result_id", logging.Entity(result)).
			Hex("previous_id", result.PreviousResultID[:]).
			Hex("block_id", result.BlockID[:]).
			Logger()

		err := e.sealResult(result, final)
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
		sealedBlockIDs = append(sealedBlockIDs, result.BlockID)

	}

	log.Debug().Int("sealed", len(sealedResultIDs)).Msg("sealed execution results")

	// clear the memory pools
	e.clearPools(sealedResultIDs)

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
}

// sealableResults traverse through unsealed and finalized blocks, and request their results
// if is missing and return all the results that are matched with enough approvals.
func (e *Engine) sealableResults() ([]*flow.ExecutionResult, error) {

	var results []*flow.ExecutionResult

	// get all approvers so we have the vote threshold
	verifiers, err := e.state.Final().Identities(filter.And(
		filter.HasStake(true),
		filter.HasRole(flow.RoleVerification),
	))
	if err != nil {
		return nil, fmt.Errorf("could not get verifiers: %w", err)
	}
	threshold := verifiers.TotalStake() / 3 * 2

	// check for stored execution results that don't have a corresponding seal
	// last sealed block
	sealed, err := e.state.Sealed().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get sealed height: %w", err)
	}
	final, err := e.state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized height: %w", err)
	}

	// keep track of any missing execution results for finalized blocks
	missingByBlockID := make(map[flow.Identifier]struct{})
	// order the missing blocks by height from low to high
	// such that when passing them to the missing block requester,
	// they can be requested in the right order.
	// the right order gives the priority to the execution result of lower
	// height blocks to be requested first, since a gap in the sealing
	// heights would stop the sealing.
	missingBlocksOrderedByHeight := make([]flow.Identifier, 0, e.maxUnsealedResults)

	// traverse each unsealed and finalized block with height from low to high,
	// if the result has been received, then add it to the list of results,
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
			return nil, fmt.Errorf("could not get header (height=%d): %w", height, err)
		}

		blockID := header.ID()
		// get the execution result for the block at this height
		result, err := e.resultsDB.ByBlockID(blockID)
		if errors.Is(err, storage.ErrNotFound) {
			missingByBlockID[blockID] = struct{}{}
			missingBlocksOrderedByHeight = append(missingBlocksOrderedByHeight, blockID)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not get execution result (block_id=%x): %w", blockID, err)
		}
		results = append(results, result)

	}

	// get all available approvals once
	approvals := e.approvals.All()

	// results can be stored in either result DB (results received enough approvals) or
	// result mempool (results have not received enough approvals).
	// go through all pending results in the result mempool and check which ones
	// we have enough approvals for, and add them to the matched result list.
	for _, result := range e.results.All() {
		// get the node IDs for all approvers of this result
		// TODO: check for duplicate approver
		var approverIDs []flow.Identifier
		resultID := result.ID()
		for _, approval := range approvals {
			if approval.Body.ExecutionResultID == resultID {
				approverIDs = append(approverIDs, approval.Body.ApproverID)
			}
		}

		// NOTE: we could've avoided adding these results to the missingByBlockID
		// in the first place if the results mempool allows us to query result by blockID.
		// The workaround here is to delete it.
		delete(missingByBlockID, result.BlockID)

		// get all of the approver identities and check threshold
		approvers := verifiers.Filter(filter.And(
			filter.HasRole(flow.RoleVerification),
			filter.HasStake(true),
			filter.HasNodeID(approverIDs...),
		))
		voted := approvers.TotalStake()
		if voted <= threshold { // nolint: emptybranch
			// NOTE: we are currently generating a seal for every result, regardless of approvals
			// e.log.Debug().Msg("ignoring result with insufficient verification")
			// continue
		}

		// add the result to the results that should be sealed
		results = append(results, result)
	}

	e.log.Info().
		Uint64("final", final.Height).
		Uint64("sealed", sealed.Height).
		Uint("threshold", e.requestReceiptThreshold).
		Int("missing", len(missingByBlockID)).
		Int("missingCount", len(missingBlocksOrderedByHeight)).
		Msg("check missing receipts")

	// request missing execution results, if sealed height is low enough
	if uint(final.Height-sealed.Height) >= e.requestReceiptThreshold {
		requestedCount := 0
		for _, blockID := range missingBlocksOrderedByHeight {
			if _, ok := missingByBlockID[blockID]; ok {
				e.requester.EntityByID(blockID, filter.Any)
				requestedCount++
			}
		}
		e.log.Info().
			Int("count", requestedCount).
			Msg("requested missing results")
	}

	return results, nil
}

func (e *Engine) sealResult(result *flow.ExecutionResult, final *flow.Header) error {

	// check if we know the block the result pertains to
	_, err := e.headersDB.ByBlockID(result.BlockID)
	if errors.Is(err, storage.ErrNotFound) {
		return errUnknownBlock
	}
	if err != nil {
		return fmt.Errorf("could not check sealed header: %w", err)
	}

	// get the previous result from our mempool or storage
	previousID := result.PreviousResultID
	previous, found := e.results.ByID(previousID)
	if !found {
		var err error
		previous, err = e.resultsDB.ByID(previousID)
		if errors.Is(err, storage.ErrNotFound) {
			return errUnknownPrevious
		}
		if err != nil {
			return fmt.Errorf("could not get previous result: %w", err)
		}
	}

	// store the result to make it persistent for later checks
	err = e.resultsDB.Store(result)
	if err != nil && !errors.Is(err, storage.ErrAlreadyExists) {
		return fmt.Errorf("could not store sealing result: %w", err)
	}
	err = e.resultsDB.Index(result.BlockID, result.ID())
	if err != nil && !errors.Is(err, storage.ErrAlreadyExists) {
		return fmt.Errorf("could not index sealing result: %w", err)
	}

	// generate & store seal
	seal := &flow.Seal{
		BlockID:      result.BlockID,
		ResultID:     result.ID(),
		InitialState: previous.FinalStateCommit,
		FinalState:   result.FinalStateCommit,
	}

	// don't add the seal if it's already been included in a proposal
	sealID := seal.ID()
	_, err = e.sealsDB.ByID(sealID)
	if err == nil {
		return nil
	}

	// we don't care whether the seal is already in the mempool
	_ = e.seals.Add(seal)

	return nil
}

func (e *Engine) clearPools(sealedIDs []flow.Identifier) {

	// NOTE: Below, we clear our memory pool of all entities that reference
	// blocks at or below the sealed head. If we don't know the block, we purge
	// the entities once we have sealed a further 100 blocks without seeing the
	// block (it's probably no longer a valid extension of the state anyway).

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
	for _, result := range e.results.All() {
		if clear[result.ID()] || shouldClear(result.BlockID) {
			_ = e.results.Rem(result.ID())
		}
	}
	for _, receipt := range e.receipts.All() {
		if clear[receipt.ExecutionResult.ID()] || shouldClear(receipt.ExecutionResult.BlockID) {
			_ = e.receipts.Rem(receipt.ID())
		}
	}
	for _, approval := range e.approvals.All() {
		if clear[approval.Body.ExecutionResultID] || shouldClear(approval.Body.BlockID) {
			_ = e.approvals.Rem(approval.ID())
		}
	}
	for _, seal := range e.seals.All() {
		if shouldClear(seal.BlockID) {
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

	e.mempool.MempoolEntries(metrics.ResourceResult, e.results.Size())
	e.mempool.MempoolEntries(metrics.ResourceReceipt, e.receipts.Size())
	e.mempool.MempoolEntries(metrics.ResourceApproval, e.approvals.Size())
	e.mempool.MempoolEntries(metrics.ResourceSeal, e.seals.Size())
}
