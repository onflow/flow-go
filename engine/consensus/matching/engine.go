// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package matching

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine is the propagation engine, which makes sure that new collections are
// propagated to the other consensus nodes on the network.
type Engine struct {
	unit      *engine.Unit             // used to control startup/shutdown
	log       zerolog.Logger           // used to log relevant actions with context
	state     protocol.State           // used to access the  protocol state
	me        module.Local             // used to access local node information
	results   storage.ExecutionResults // used to permanently store results
	receipts  mempool.Receipts         // holds collection guarantees in memory
	approvals mempool.Approvals        // holds result approvals in memory
	seals     mempool.Seals            // holds block seals in memory
}

// New creates a new collection propagation engine.
func New(log zerolog.Logger, net module.Network, state protocol.State, me module.Local, results storage.ExecutionResults, receipts mempool.Receipts, approvals mempool.Approvals, seals mempool.Seals) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:      engine.NewUnit(),
		log:       log.With().Str("engine", "matching").Logger(),
		state:     state,
		me:        me,
		results:   results,
		receipts:  receipts,
		approvals: approvals,
		seals:     seals,
	}

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
	switch entity := event.(type) {
	case *flow.ExecutionReceipt:
		return e.onReceipt(originID, entity)
	case *flow.ResultApproval:
		return e.onApproval(originID, entity)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// onReceipt processes a new execution receipt.
func (e *Engine) onReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("block_id", logging.ID(receipt.ExecutionResult.BlockID)).
		Msg("execution receipt received")

	// check the execution receipt is sent by its executor
	if receipt.ExecutorID != originID {
		return fmt.Errorf("invalid origin for receipt: %x", originID)
	}

	// get the identity of the origin node, so we can check if it's a valid
	// source for a execution receipt (usually execution nodes)
	identity, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("could not get receipt identity: %w", err)
	}

	// check if the identity has a stake
	if identity.Stake == 0 {
		return fmt.Errorf("executor has zero stake (%x)", identity.NodeID)
	}

	// check that the origin is an execution node
	if identity.Role != flow.RoleExecution {
		return fmt.Errorf("invalid executor node role (%s)", identity.Role)
	}

	// store in the memory pool
	err = e.receipts.Add(receipt)
	if err != nil {
		return fmt.Errorf("could not store receipt: %w", err)
	}

	err = e.tryBuildSeal(receipt.ExecutionResult.BlockID)
	if err != nil {
		return fmt.Errorf("could not match seals: %w", err)
	}

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("receipt_id", logging.Entity(receipt)).
		Msg("execution receipt processed")

	return nil
}

// onApproval processes a new result approval.
func (e *Engine) onApproval(originID flow.Identifier, approval *flow.ResultApproval) error {

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("approval_id", logging.Entity(approval)).
		Msg("result approval received")

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

	// check if the node has a stake
	if identity.Stake == 0 {
		return fmt.Errorf("approvar has zero stake (%x)", identity.NodeID)
	}

	// check that the origin is a verification node
	if identity.Role != flow.RoleVerification {
		return fmt.Errorf("invalid approver node role (%s)", identity.Role)
	}

	// store in the memory pool
	err = e.approvals.Add(approval)
	if err != nil {
		return fmt.Errorf("could not store approval: %w", err)
	}

	// check if we can build a seal by first matching receipts
	// NOTE: this is not super efficient, so it makes sense to integrate a step
	// for when receipts have already been matched; however, the logic is simple
	// now, so let's wait until we actually see a problem with performance
	err = e.tryBuildSeal(approval.Body.BlockID)
	if err != nil {
		return fmt.Errorf("could not match seals: %w", err)
	}

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("approval_id", logging.Entity(approval)).
		Msg("result approval processed")

	return nil
}

// tryBuildSeal will match approvals for each result of the given block
// and check if any of the results receives a majority, creating a block seal
// if possible.
func (e *Engine) tryBuildSeal(blockID flow.Identifier) error {

	// get the list of approvers so we can tally their votes
	// get all execution node identities from the state
	approvers, err := e.state.AtBlockID(blockID).Identities(
		filter.HasStake,
		filter.HasRole(flow.RoleVerification),
	)
	if err != nil {
		return fmt.Errorf("could not get verifier identities: %w", err)
	}

	// make a list of all result IDs and each of their chunks
	receipts := e.receipts.ByBlockID(blockID)
	results := make(map[flow.Identifier]*flow.ExecutionResult)
	votes := make(map[flow.Identifier](map[uint64]uint64))
	for _, receipt := range receipts {
		results[receipt.ExecutionResult.ID()] = &receipt.ExecutionResult
		votes[receipt.ExecutionResult.ID()] = make(map[uint64]uint64)
	}

	// tally all the approvals for the given results and chunks
	for resultID := range votes {
		approvals := e.approvals.ByResultID(resultID)
		for _, approval := range approvals {
			approver, ok := approvers.ByNodeID(approval.Body.ApproverID)
			if !ok {
				e.log.Debug().Msg("skipping unknown approver")
				continue
			}
			chunkIndex := approval.Body.ChunkIndex
			votes[resultID][chunkIndex] += approver.Stake
		}
	}

	// check if any result reached the threshold on all chunks
	total := approvers.TotalStake()
	threshold := (total * 2) / 3 // TODO: precise rounding
ResultLoop:
	for resultID, chunkVotes := range votes {
		result := results[resultID]

		// check if there is any chunk that didn't reach the threshold
		for _, chunk := range result.Chunks {
			voted := chunkVotes[chunk.Index]
			if voted < threshold {
				continue ResultLoop
			}
		}

		// TODO: at this point we can also detect mismatching results
		// and punish them

		// if we reached here, we have a fully approved result
		// NOTE: this should only ever happen for one result per block
		return e.createSeal(result)
	}

	return nil
}

// createSeal creates the seal for the given result.
func (e *Engine) createSeal(result *flow.ExecutionResult) error {

	// get previous result to have starting state for seal
	previous, err := e.results.ByID(result.PreviousResultID)
	if err != nil {
		return fmt.Errorf("could not get previous result: %w", err)
	}

	// persist the result on disk
	err = e.results.Store(result)
	if err != nil {
		return fmt.Errorf("could not store result: %w", err)
	}

	// create and store the seal
	seal := flow.Seal{
		BlockID:       result.BlockID,
		PreviousState: previous.FinalStateCommit,
		FinalState:    result.FinalStateCommit,
	}
	err = e.seals.Add(&seal)
	if err != nil {
		return fmt.Errorf("could not cache seal: %w", err)
	}

	// clear up the caches
	e.receipts.DropForBlock(result.BlockID)
	e.approvals.DropForBlock(result.BlockID)

	// TODO: consider what receipts & approvals to accept going forward

	return nil
}
