// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package matching

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

var (
	errUnknownBlock    = errors.New("result block unknown")
	errUnknownPrevious = errors.New("previous result unknown")
)

// Engine is the matching engine, which matches execution receipts with result
// approvals to create block seals.
type Engine struct {
	unit      *engine.Unit             // used to control startup/shutdown
	log       zerolog.Logger           // used to log relevant actions with context
	metrics   module.EngineMetrics     // used to track sent and received messages
	mempool   module.MempoolMetrics    // used to track mempool size
	state     protocol.State           // used to access the  protocol state
	me        module.Local             // used to access local node information
	resultsDB storage.ExecutionResults // used to permanently store results
	sealsDB   storage.Seals            // used to check existing seals
	headersDB storage.Headers          // used to check sealed headers
	results   mempool.Results          // holds execution results in memory
	receipts  mempool.Receipts         // holds execution receipts in memory
	approvals mempool.Approvals        // holds result approvals in memory
	seals     mempool.Seals            // holds block seals in memory
	missing   map[flow.Identifier]uint // track how often a block was missing
}

// New creates a new collection propagation engine.
func New(log zerolog.Logger, collector module.EngineMetrics, mempool module.MempoolMetrics, net module.Network, state protocol.State, me module.Local, resultsDB storage.ExecutionResults, sealsDB storage.Seals, headersDB storage.Headers, results mempool.Results, receipts mempool.Receipts, approvals mempool.Approvals, seals mempool.Seals) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:      engine.NewUnit(),
		log:       log.With().Str("engine", "matching").Logger(),
		metrics:   collector,
		mempool:   mempool,
		state:     state,
		me:        me,
		resultsDB: resultsDB,
		sealsDB:   sealsDB,
		headersDB: headersDB,
		results:   results,
		receipts:  receipts,
		approvals: approvals,
		seals:     seals,
		missing:   make(map[flow.Identifier]uint),
	}

	e.mempool.MempoolEntries(metrics.ResourceResult, e.results.Size())
	e.mempool.MempoolEntries(metrics.ResourceReceipt, e.receipts.Size())
	e.mempool.MempoolEntries(metrics.ResourceApproval, e.approvals.Size())
	e.mempool.MempoolEntries(metrics.ResourceSeal, e.seals.Size())

	// register engine with the receipt provider
	_, err := net.Register(engine.ExecutionReceiptProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for results: %w", err)
	}

	// register engine with the approval provider
	_, err = net.Register(engine.ApprovalProvider, e)
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

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	e.unit.Lock()
	defer e.unit.Unlock()

	switch ev := event.(type) {
	case *flow.ExecutionReceipt:
		e.metrics.MessageReceived(metrics.EngineMatching, metrics.MessageExecutionReceipt)
		return e.onReceipt(originID, ev)
	case *flow.ResultApproval:
		e.metrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
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
		return fmt.Errorf("invalid origin for receipt (executor: %x, origin: %x)", receipt.ExecutorID, originID)
	}

	// get the identity of the origin node, so we can check if it's a valid
	// source for a execution receipt (usually execution nodes)
	identity, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("could not get executor identity: %w", err)
	}

	// check that the origin is an execution node
	if identity.Role != flow.RoleExecution {
		return fmt.Errorf("invalid executor node role (%s)", identity.Role)
	}

	// check if the identity has a stake
	if identity.Stake == 0 {
		return fmt.Errorf("executor has zero stake (%x)", identity.NodeID)
	}

	// check if the result of this receipt is already in the dB
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
		return fmt.Errorf("invalid origin for approval: %x", originID)
	}

	// get the identity of the origin node, so we can check if it's a valid
	// source for a result approval (usually verification node)
	identity, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("could not get approval identity: %w", err)
	}

	// check that the origin is a verification node
	if identity.Role != flow.RoleVerification {
		return fmt.Errorf("invalid approver node role (%s)", identity.Role)
	}

	// check if the identity has a stake
	if identity.Stake == 0 {
		return fmt.Errorf("verifier has zero stake (%x)", identity.NodeID)
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
	e.unit.Lock()
	defer e.unit.Unlock()

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

	e.log.Info().Int("num_results", len(results)).Msg("identified sealable execution results")

	// process the results results
	var sealedIDs []flow.Identifier
	for _, result := range results {

		log := e.log.With().
			Hex("result_id", logging.Entity(result)).
			Hex("previous_id", result.PreviousResultID[:]).
			Hex("block_id", result.BlockID[:]).
			Logger()

		err := e.sealResult(result)
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
		sealedIDs = append(sealedIDs, result.ID())

		log.Info().Msg("sealed execution result")
	}

	// clear the memory pools
	e.clearPools(sealedIDs)
}

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
	sealed, err := e.state.Sealed().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get sealed height: %w", err)
	}
	final, err := e.state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized height: %w", err)
	}

	for height := sealed.Height; height < final.Height; height++ {
		//TODO suggested fix for #3999
		// get the execution result for the block at this height
		// if no seal exists for this ER in the DB, nor in the mempool:
		//   create a new seal and add it to mempool

		// TODO hard-coded
		// stop searching if we would overflow the seal mempool
		if int(e.seals.Size())+len(results) >= 1000 {
			return results, nil
		}

		// get the block header at this height
		header, err := e.headersDB.ByHeight(height)
		if err != nil {
			// should never happen
			return nil, fmt.Errorf("could not get header (height=%d): %w", height, err)
		}

		// get the execution result for the block at this height
		result, err := e.resultsDB.ByBlockID(header.ID())
		if err != nil {
			break
		}
		results = append(results, result)
	}

	// get all available approvals once
	approvals := e.approvals.All()

	// go through all results and check which ones we have enough approvals for
	for _, result := range e.results.All() {

		// TODO hard-coded
		// stop searching if we would overflow the seal mempool
		if int(e.seals.Size())+len(results) >= 1000 {
			return results, nil
		}

		// get the node IDs for all approvers of this result
		// TODO: check for duplicate approver
		var approverIDs []flow.Identifier
		resultID := result.ID()
		for _, approval := range approvals {
			if approval.Body.ExecutionResultID == resultID {
				approverIDs = append(approverIDs, approval.Body.ApproverID)
			}
		}

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

	return results, nil
}

func (e *Engine) sealResult(result *flow.ExecutionResult) error {

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
	if err != nil {
		return fmt.Errorf("could not store sealing result: %w", err)
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
	if err != nil {
		return fmt.Errorf("seal (id=%x, block_id=%x) already stored: %w", sealID, seal.BlockID, err)
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
