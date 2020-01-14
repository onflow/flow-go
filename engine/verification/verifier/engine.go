package verifier

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine implements the verifier engine of the verification node,
// responsible for reception of a execution receipt, verifying that, and
// emitting its corresponding result approval to the entire system.
type Engine struct {
	unit               *engine.Unit        // used to control startup/shutdown
	log                zerolog.Logger      // used to log relevant actions
	collectionsConduit network.Conduit     // used to get collections from collection nodes
	receiptsConduit    network.Conduit     // used to get execution receipts from execution nodes
	approvalsConduit   network.Conduit     // used to propagate result approvals
	me                 module.Local        // used to access local node information
	state              protocol.State      // used to access the  protocol state
	receipts           mempool.Receipts    // used to store execution receipts in memory
	blocks             mempool.Blocks      // used to store blocks in memory
	collections        mempool.Collections // used to store collections in memory
}

// New creates and returns a new instance of a verifier engine.
func New(
	log zerolog.Logger,
	net module.Network,
	state protocol.State,
	me module.Local,
	receipts mempool.Receipts,
	blocks mempool.Blocks,
	collections mempool.Collections,
) (*Engine, error) {
	e := &Engine{
		unit:        engine.NewUnit(),
		log:         log,
		state:       state,
		me:          me,
		receipts:    receipts,
		blocks:      blocks,
		collections: collections,
	}

	var err error
	e.collectionsConduit, err = net.Register(engine.CollectionProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on collection provider channel: %w", err)
	}

	e.receiptsConduit, err = net.Register(engine.ResultProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on execution provider channel: %w", err)
	}

	e.approvalsConduit, err = net.Register(engine.ApprovalProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on approval provider channel: %w", err)
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
	case *flow.Block:
		return e.handleBlock(resource)
	case *flow.ExecutionReceipt:
		return e.handleExecutionReceipt(originID, resource)
	case *flow.Collection:
		return e.handleCollection(originID, resource)
	case *messages.CollectionResponse:
		return e.handleCollection(originID, &resource.Collection)
	default:
		return errors.Errorf("invalid event type (%T)", event)
	}
}

// handleExecutionReceipt receives an execution receipt (exrcpt), verifies that and emits
// a result approval upon successful verification
func (e *Engine) handleExecutionReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {

	e.log.Info().
		Hex("origin_id", logging.ID(originID)).
		Hex("receipt_id", logging.Entity(receipt)).
		Msg("execution receipt received")

	// TODO: correctness check for execution receipts

	// validating identity of the originID
	id, err := e.state.Final().Identity(originID)
	if err != nil {
		// TODO: potential attack on authenticity
		return fmt.Errorf("invalid origin id (%s): %w", originID[:], err)
	}

	// validating role of the originID
	// an execution receipt should be either coming from an execution node through the
	// Process method, or from the current verifier node itself through the Submit method
	if id.Role != flow.RoleExecution {
		// TODO: potential attack on integrity
		return fmt.Errorf("invalid role for generating an execution receipt, id: %s, role: %s", id.NodeID, id.Role)
	}

	// storing the execution receipt in the store of the engine
	// this will fail if the receipt already exists in the store
	err = e.receipts.Add(receipt)
	if err != nil {
		return fmt.Errorf("could not store execution receipt: %w", err)
	}

	// TODO start a verification job asyncronously.
	// For now, we do it synchronously and submit a RA before checking anything
	e.verify(originID, receipt)

	return nil
}

// handleBlock handles an incoming block.
func (e *Engine) handleBlock(block *flow.Block) error {

	e.log.Debug().
		Hex("block_id", logging.Entity(block)).
		Uint64("block_number", block.Number).
		Msg("received block")

	err := e.blocks.Add(block)
	if err != nil {
		return fmt.Errorf("could not store block: %w", err)
	}

	return nil
}

// handleCollection handles receipt of a new collection, either via push or after
// a request. It ensures the sender is valid and notifies the main verification
// process.
func (e *Engine) handleCollection(originID flow.Identifier, coll *flow.Collection) error {

	e.log.Info().
		Hex("origin_id", logging.ID(originID)).
		Hex("collection_id", logging.Entity(coll)).
		Msg("collection received")

	id, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("invalid origin id (%s): %w", id, err)
	}

	if id.Role != flow.RoleCollection {
		return fmt.Errorf("invalid role for receiving collection: %s", id.Role)
	}

	err = e.collections.Add(coll)
	if err != nil {
		return fmt.Errorf("could not add collection to mempool: %w", err)
	}

	// TODO notify of new collection

	return nil
}

// requestCollection submits a request for the given collection to collection nodes.
func (e *Engine) requestCollection(collID flow.Identifier) error {

	collNodes, err := e.state.Final().Identities(identity.HasRole(flow.RoleCollection))
	if err != nil {
		return fmt.Errorf("could not load collection node identities: %w", err)
	}

	req := &messages.CollectionRequest{
		CollectionID: collID,
	}

	// TODO we should only submit to cluster which owns the collection
	err = e.collectionsConduit.Submit(req, collNodes.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not submit request for collection (id=%s): %w", collID, err)
	}

	return nil
}

// verify is an internal component of the verifier engine.
// It receives an execution receipt and does the core verification process.
// The origin ID indicates the node which originally submitted the event to
// the peer-to-peer network.
// If the submission was successful it generates a result approval for the incoming ExecutionReceipt
// and submits that to the network
func (e *Engine) verify(originID flow.Identifier, receipt *flow.ExecutionReceipt) {

	result := receipt.ExecutionResult
	blockID := result.BlockID

	log := e.log.With().
		Hex("block_id", logging.ID(blockID)).
		Hex("result_id", logging.Entity(result)).
		Logger()

	// TODO temporary as log uses are commented
	_ = log

	// request block if not available
	// TODO handle this case
	//block, err := e.blocks.Get(blockID)
	//if err != nil {
	//	// TODO should request the block here
	//	log.Error().
	//		Err(err).
	//		Msg("could not get block")
	//	return
	//}

	// request collection if not available
	// TODO handle this case
	//for _, guarantee := range block.Guarantees {
	//	err := e.requestCollection(guarantee.ID())
	//	if err != nil {
	//		log.Error().
	//			Err(err).
	//			Msgf("could not request collection (id=%s): %w", logging.Entity(guarantee), err)
	//	}
	//}

	// TODO for now, just submit the result approval without checking anything
	// extracting list of consensus nodes' ids
	consIDs, err := e.state.Final().
		Identities(identity.HasRole(flow.RoleConsensus))
	if err != nil {
		// todo this error needs more advance handling after MVP
		e.log.Error().
			Str("error: ", err.Error()).
			Msg("could not load the consensus nodes ids")
		return
	}

	// emitting a result approval to all consensus nodes
	approval := &flow.ResultApproval{
		ResultApprovalBody: flow.ResultApprovalBody{
			ExecutionResultID:    receipt.ExecutionResult.ID(),
			AttestationSignature: nil,
			ChunkIndexList:       nil,
			Proof:                nil,
			Spocks:               nil,
		},
		VerifierSignature: nil,
	}

	// broadcasting result approval to all consensus nodes
	err = e.approvalsConduit.Submit(approval, consIDs.NodeIDs()...)
	if err != nil {
		e.log.Error().
			Err(err).
			Msg("could not submit result approval to consensus nodes")
	}
}
