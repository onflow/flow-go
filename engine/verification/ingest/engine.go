package ingest

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine implements the ingest engine of the verification node. It is
// responsible for receiving and handling new execution receipts. It requests
// all dependent resources for each execution receipt and relays a complete
// execution result to the verifier engine when all dependencies are ready.
type Engine struct {
	unit               *engine.Unit
	log                zerolog.Logger
	collectionsConduit network.Conduit
	stateConduit       network.Conduit
	me                 module.Local
	state              protocol.State
	verifierEng        network.Engine // for submitting ERs that are ready to be verified
	receipts           mempool.Receipts
	blocks             mempool.Blocks
	collections        mempool.Collections
	chunkStates        mempool.ChunkStates
}

// New creates and returns a new instance of the ingest engine.
func New(
	log zerolog.Logger,
	net module.Network,
	state protocol.State,
	me module.Local,
	verifierEng network.Engine,
	receipts mempool.Receipts,
	blocks mempool.Blocks,
	collections mempool.Collections,
	chunkStates mempool.ChunkStates,
) (*Engine, error) {

	e := &Engine{
		unit:        engine.NewUnit(),
		log:         log,
		state:       state,
		me:          me,
		receipts:    receipts,
		verifierEng: verifierEng,
		blocks:      blocks,
		collections: collections,
		chunkStates: chunkStates,
	}

	var err error
	e.collectionsConduit, err = net.Register(engine.CollectionProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on collection provider channel: %w", err)
	}

	e.collectionsConduit, err = net.Register(engine.StateProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on execution state provider channel: %w", err)
	}

	_, err = net.Register(engine.ReceiptProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on execution receipt provider channel: %w", err)
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

	id, err := e.state.Final().Identity(originID)
	if err != nil {
		// TODO: potential attack on authenticity
		return fmt.Errorf("invalid origin id (%s): %w", originID[:], err)
	}

	// execution results are only valid from execution nodes
	if id.Role != flow.RoleExecution {
		// TODO: potential attack on integrity
		return fmt.Errorf("invalid role for generating an execution receipt, id: %s, role: %s", id.NodeID, id.Role)
	}

	// store the execution receipt in the store of the engine
	// this will fail if the receipt already exists in the store
	err = e.receipts.Add(receipt)
	if err != nil {
		return fmt.Errorf("could not store execution receipt: %w", err)
	}

	e.checkPendingReceipts()

	return nil
}

// handleBlock handles an incoming block.
func (e *Engine) handleBlock(block *flow.Block) error {

	e.log.Info().
		Hex("block_id", logging.Entity(block)).
		Uint64("block_number", block.Number).
		Msg("received block")

	err := e.blocks.Add(block)
	if err != nil {
		return fmt.Errorf("could not store block: %w", err)
	}

	e.checkPendingReceipts()

	return nil
}

// handleCollection handles receipt of a new collection, either via push or
// after a request. It adds the collection to the mempool and checks for
// pending receipts that are ready for verification.
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

	e.checkPendingReceipts()

	return nil
}

// handleExecutionStateResponse handles responses to our requests for execution
// states for particular chunks. It adds the state to the mempool and checks for
// pending receipts that are ready for verification.
func (e *Engine) handleExecutionStateResponse(originID flow.Identifier, res *messages.ExecutionStateResponse) error {

	e.log.Info().
		Hex("origin_id", logging.ID(originID)).
		Hex("chunk_id", logging.ID(res.State.ChunkID)).
		Msg("execution state received")

	id, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("invalid origin id (%s): %w", id, err)
	}

	if id.Role != flow.RoleExecution {
		return fmt.Errorf("invalid role for receiving execution state: %s", id.Role)
	}

	err = e.chunkStates.Add(&res.State)
	if err != nil {
		return fmt.Errorf("could not add execution state (chunk_id=%s): %w", res.State.ChunkID, err)
	}

	e.checkPendingReceipts()

	return nil
}

// requestCollection submits a request for the given collection to collection nodes.
func (e *Engine) requestCollection(collID flow.Identifier) error {

	collNodes, err := e.state.Final().Identities(identity.HasRole(flow.RoleCollection))
	if err != nil {
		return fmt.Errorf("could not load collection node identities: %w", err)
	}

	req := &messages.CollectionRequest{
		ID: collID,
	}

	// TODO we should only submit to cluster which owns the collection
	err = e.collectionsConduit.Submit(req, collNodes.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not submit request for collection (id=%s): %w", collID, err)
	}

	return nil
}

func (e *Engine) requestState(chunkID flow.Identifier) error {

	exeNodes, err := e.state.Final().Identities(identity.HasRole(flow.RoleExecution))
	if err != nil {
		return fmt.Errorf("could not load execution node identities: %w", err)
	}

	req := &messages.ExecutionStateRequest{
		ChunkID: chunkID,
	}

	err = e.stateConduit.Submit(req, exeNodes.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not submit request for execution state (chunk_id=%s): %w", chunkID, err)
	}

	return nil
}

// checkReceiptCollections checks all collections depended on by the
// given execution receipt. Returns true if all collections are available
// locally. If the collections are not available locally, they are requested.
func (e *Engine) checkReceiptCollections(receipt *flow.ExecutionReceipt) bool {
	result := receipt.ExecutionResult
	blockID := result.BlockID

	log := e.log.With().
		Hex("block_id", logging.ID(blockID)).
		Hex("receipt_id", logging.Entity(receipt)).
		Logger()

	// ensure we have the block corresponding to this execution
	block, err := e.blocks.Get(blockID)
	if err != nil {
		// TODO should request the block here. For now, we require that we
		// have received the block at this point as there is no way to request it
		log.Error().
			Err(err).
			Msg("could not check collections - missing block")
		return false
	}

	chunks := result.Chunks.Items()

	// whether the receipt is ready for verification
	ready := true

	for _, chunk := range chunks {
		collIndex := int(chunk.CollectionIndex)

		// ensure the collection index specified by the ER is valid
		if len(block.Guarantees) <= collIndex {
			log.Error().
				Int("collection_index", collIndex).
				Msg("could not check collections - invalid collection index")
			continue
		}

		// request the collection if we don't already have it
		collID := block.Guarantees[collIndex].ID()
		if !e.collections.Has(collID) {
			// a collection is missing, the receipt cannot yet be verified
			ready = false

			// TODO rate limit these requests
			err := e.requestCollection(collID)
			if err != nil {
				log.Error().
					Err(err).
					Hex("collection_id", logging.ID(collID)).
					Msg("could not request collection")
				continue
			}
		}
	}

	return ready
}

// getCompleteExecutionResult creates a complete execution result for a receipt
// whose dependencies are all locally available.
func (e *Engine) getCompleteExecutionResult(receipt *flow.ExecutionReceipt) (*verification.CompleteExecutionResult, error) {
	// TODO
	return &verification.CompleteExecutionResult{
		Receipt: *receipt,
	}, nil
}

// checkPendingReceipts checks all pending receipts in the mempool and verifies
// any that are ready for verification.
func (e *Engine) checkPendingReceipts() {
	// TODO address race on this method - if called simultaneously, could double-verify
	receipts := e.receipts.All()

	for _, receipt := range receipts {
		ready := e.checkReceiptCollections(receipt)
		if ready {

			res, err := e.getCompleteExecutionResult(receipt)
			if err != nil {
				e.log.Error().
					Err(err).
					Hex("result_id", logging.Entity(receipt.ExecutionResult)).
					Msg("could not get complete execution result")
				continue
			}

			err = e.verifierEng.ProcessLocal(res)
			if err != nil {
				e.log.Error().
					Err(err).
					Hex("result_id", logging.Entity(receipt.ExecutionResult)).
					Msg("could not get complete execution result")
				continue
			}
			e.receipts.Rem(receipt.ID())
		}
	}
}
