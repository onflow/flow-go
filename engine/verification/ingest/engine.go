package ingest

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/engine/verification/utils"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
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
	chunksConduit      network.Conduit
	me                 module.Local
	state              protocol.State
	verifierEng        network.Engine   // for submitting ERs that are ready to be verified
	authReceipts       mempool.Receipts // keeps receipts with authenticated origin IDs
	pendingReceipts    mempool.Receipts // keeps receipts pending for their originID to be authenticated
	authCollections       mempool.Collections    // keeps collections with authenticated origin IDs
	pendingCollections    mempool.Collections    // keeps collections pending for their origin IDs to be authenticated
	pendingChunkDataPacks mempool.ChunkDataPacks // keeps chunk data packs pending for their origin ID to be authenticated
	authChunkDataPacks    mempool.ChunkDataPacks // keeps chunk data packs with authenticated origin IDs
	blockStorage       storage.Blocks
	checkChunksLock    sync.Mutex           // protects the checkPendingChunks method to prevent double-verifying
	assigner           module.ChunkAssigner // used to determine chunks this node needs to verify
	chunkStates           mempool.ChunkStates
}

// New creates and returns a new instance of the ingest engine.
func New(
	log zerolog.Logger,
	net module.Network,
	state protocol.State,
	me module.Local,
	verifierEng network.Engine,
	authReceipts mempool.Receipts,
	pendingReceipts mempool.Receipts,
	collections mempool.Collections,
	chunkStates mempool.ChunkStates,
	chunkDataPacks mempool.ChunkDataPacks,
	blockStorage storage.Blocks,
	assigner module.ChunkAssigner,
) (*Engine, error) {

	e := &Engine{
		unit:            engine.NewUnit(),
		log:             log,
		state:           state,
		me:              me,
		authReceipts:    authReceipts,
		pendingReceipts: pendingReceipts,
		verifierEng:     verifierEng,
		authCollections:    collections,
		chunkStates:     chunkStates,
		authChunkDataPacks: chunkDataPacks,
		blockStorage:       blockStorage,
		assigner:           assigner,
	}

	var err error
	e.collectionsConduit, err = net.Register(engine.CollectionProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on collection provider channel: %w", err)
	}

	// for chunk states and chunk data packs.
	e.stateConduit, err = net.Register(engine.ExecutionStateProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on execution state provider channel: %w", err)
	}

	e.chunksConduit, err = net.Register(engine.ChunkDataPackProvider, e)
	if err != nil {
		return nil, errors.Wrap(err, "could not register chunk data pack provider engine")
	}

	_, err = net.Register(engine.ExecutionReceiptProvider, e)
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
	case *messages.ExecutionStateResponse:
		return e.handleExecutionStateResponse(originID, resource)
	case *messages.ChunkDataPackResponse:
		return e.handleChunkDataPack(originID, &resource.Data)
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
	// extracts list of verifier nodes id
	origin, err := e.state.AtBlockID(receipt.ExecutionResult.BlockID).Identity(originID)
	if err != nil {
		// TODO: potential attack on authenticity
		// stores ER in pending receipts till a block arrives authenticating this
		err = e.pendingReceipts.Add(receipt)
		if err != nil && err != mempool.ErrEntityAlreadyExists {
			return fmt.Errorf("could not store execution receipt in pending pool: %w", err)
		}

	} else {
		// execution results are only valid from execution nodes
		if origin.Role != flow.RoleExecution {
			// TODO: potential attack on integrity
			return fmt.Errorf("invalid role for generating an execution receipt, id: %s, role: %s", origin.NodeID, origin.Role)
		}

		// store the execution receipt in the store of the engine
		// this will fail if the receipt already exists in the store
		err = e.authReceipts.Add(receipt)
		if err != nil && err != mempool.ErrEntityAlreadyExists {
			return fmt.Errorf("could not store execution receipt: %w", err)
		}

	}

	e.checkPendingChunks()

	return nil
}

// handleChunkDataPack receives a chunk data pack and stores that in the mempool
func (e *Engine) handleChunkDataPack(originID flow.Identifier, chunkDataPack *flow.ChunkDataPack) error {
	e.log.Info().
		Hex("origin_id", logging.ID(originID)).
		Hex("chunk_data_pack_id", logging.Entity(chunkDataPack)).
		Msg("chunk data pack received")

	// TODO tracking request feature
	// https://github.com/dapperlabs/flow-go/issues/2970
	origin, err := e.state.Final().Identity(originID)
	if err != nil {
		// TODO: potential attack on authenticity
		return fmt.Errorf("invalid origin id (%s): %w", originID[:], err)
	}

	// chunk data pack should only be sent by an execution node
	if origin.Role != flow.RoleExecution {
		// TODO: potential attack on integrity
		return fmt.Errorf("invalid role for generating an execution receipt, id: %s, role: %s", origin.NodeID, origin.Role)
	}

	// store the chunk data pack in the store of the engine
	// this will fail if the receipt already exists in the store
	err = e.authChunkDataPacks.Add(chunkDataPack)
	if err != nil {
		return fmt.Errorf("could not store execution receipt: %w", err)
	}

	e.checkPendingChunks()

	return nil
}

// handleBlock handles an incoming block.
func (e *Engine) handleBlock(block *flow.Block) error {

	e.log.Info().
		Hex("block_id", logging.Entity(block)).
		Uint64("block_view", block.View).
		Msg("received block")

	err := e.blockStorage.Store(block)
	if err != nil {
		return fmt.Errorf("could not store block: %w", err)
	}

	blockID := block.ID()
	for _, receipt := range e.pendingReceipts.All() {
		if receipt.ExecutionResult.BlockID == blockID {
			// adds receipt to authenticated receipts pool
			err := e.authReceipts.Add(receipt)
			if err != nil {
				return fmt.Errorf("could not move receipt to authenticated receipts pool: %w", err)
			}

			// removes receipt from pending receipts pool
			e.pendingReceipts.Rem(receipt.ID())
		}
	}

	e.checkPendingChunks()

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

	// extracts list of verifier nodes id
	//
	// TODO tracking request feature
	// https://github.com/dapperlabs/flow-go/issues/2970
	origin, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("invalid origin id (%s): %w", origin, err)
	}

	if origin.Role != flow.RoleCollection {
		return fmt.Errorf("invalid role for receiving collection: %s", origin.Role)
	}

	err = e.authCollections.Add(coll)
	if err != nil {
		return fmt.Errorf("could not add collection to mempool: %w", err)
	}

	e.checkPendingChunks()

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

	// extracts list of verifier nodes id
	//
	// TODO tracking request feature
	// https://github.com/dapperlabs/flow-go/issues/2970
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

	e.checkPendingChunks()

	return nil
}

// requestCollection submits a request for the given collection to collection nodes.
func (e *Engine) requestCollection(collID flow.Identifier) error {

	// extracts list of verifier nodes id
	//
	collNodes, err := e.state.Final().Identities(filter.HasRole(flow.RoleCollection))
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

// requestExecutionState submits a request for the state required by the
// given chunk to execution nodes.
func (e *Engine) requestExecutionState(chunkID flow.Identifier) error {

	// extracts list of verifier nodes id
	//
	exeNodes, err := e.state.Final().Identities(filter.HasRole(flow.RoleExecution))
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

// requestChunkDataPack submits a request for the given chunk ID to the execution nodes.
func (e *Engine) requestChunkDataPack(chunkID flow.Identifier) error {
	// extracts list of verifier nodes id
	//
	execNodes, err := e.state.Final().Identities(filter.HasRole(flow.RoleExecution))
	if err != nil {
		return fmt.Errorf("could not load execution nodes identities: %w", err)
	}

	req := &messages.ChunkDataPackRequest{
		ChunkID: chunkID,
	}

	// TODO we should only submit to execution node that generated execution receipt
	err = e.chunksConduit.Submit(req, execNodes.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not submit request for collection (id=%s): %w", chunkID, err)
	}

	return nil
}

// getBlockForReceipt checks the block referenced by the given receipt. If the
// block is available locally, returns true and the block. Otherwise, returns
// false and requests the block.
// TODO does not yet request block
func (e *Engine) getBlockForReceipt(receipt *flow.ExecutionReceipt) (*flow.Block, bool) {
	// ensure we have the block corresponding to this execution
	block, err := e.blockStorage.ByID(receipt.ExecutionResult.BlockID)
	if err != nil {
		// TODO should request the block here. For now, we require that we
		// have received the block at this point as there is no way to request it
		return nil, false
	}

	return block, true
}

// getChunkStateForReceipt checks the chunk state depended on by the given
// execution receipt. If the chunk state is available locally, returns true
// as well as the chunk data itself.
// Otherwise, returns false and requests the chunk state
func (e *Engine) getChunkStateForReceipt(receipt *flow.ExecutionReceipt, chunkID flow.Identifier) (*flow.ChunkState, bool) {

	log := e.log.With().
		Hex("block_id", logging.ID(receipt.ExecutionResult.BlockID)).
		Hex("chunk_id", logging.ID(chunkID)).
		Hex("receipt_id", logging.Entity(receipt)).
		Logger()

	if !e.chunkStates.Has(chunkID) {
		// the chunk state is missing, the chunk cannot yet be verified
		// TODO rate limit these requests
		err := e.requestExecutionState(chunkID)
		if err != nil {
			log.Error().
				Err(err).
				Hex("chunk_id", logging.ID(chunkID)).
				Msg("could not request chunk state")
		}
		return nil, false
	}

	// chunk state exists and retrieved and returned
	chunkState, err := e.chunkStates.ByID(chunkID)
	if err != nil {
		// couldn't get chunk state from mempool, the chunk cannot yet be verified
		log.Error().
			Err(err).
			Hex("chunk_id", logging.ID(chunkID)).
			Msg("could not get chunk")
		return nil, false
	}
	return chunkState, true
}

// getChunkDataPackForReceipt checks the chunk data pack associated with a chunk ID and
// execution receipt. If the chunk data pack is available locally, returns true
// as well as the chunk data pack itself.
// Otherwise, returns false and requests the chunk data pack
func (e *Engine) getChunkDataPackForReceipt(receipt *flow.ExecutionReceipt, chunkID flow.Identifier) (*flow.ChunkDataPack, bool) {

	log := e.log.With().
		Hex("block_id", logging.ID(receipt.ExecutionResult.BlockID)).
		Hex("chunk_id", logging.ID(chunkID)).
		Hex("receipt_id", logging.Entity(receipt)).
		Logger()

	if !e.authChunkDataPacks.Has(chunkID) {
		// the chunk data pack is missing, the chunk cannot yet be verified
		// TODO rate limit these requests
		err := e.requestChunkDataPack(chunkID)
		if err != nil {
			log.Error().
				Err(err).
				Hex("chunk_id", logging.ID(chunkID)).
				Msg("could not request chunk data pack")
		}
		return nil, false
	}

	// chunk data pack exists and retrieved and returned
	chunkDataPack, err := e.authChunkDataPacks.ByChunkID(chunkID)
	if err != nil {
		// couldn't get chunk state from mempool, the chunk cannot yet be verified
		log.Error().
			Err(err).
			Hex("chunk_id", logging.ID(chunkID)).
			Msg("could not get chunk data pack")
		return nil, false
	}
	return chunkDataPack, true
}

// getCollectionForChunk checks the collection depended on the
// given execution receipt and chunk. Returns true if the collections is available
// locally. If the collections is not available locally, it is requested.
func (e *Engine) getCollectionForChunk(block *flow.Block, receipt *flow.ExecutionReceipt, chunk *flow.Chunk) (*flow.Collection, bool) {

	log := e.log.With().
		Hex("block_id", logging.ID(block.ID())).
		Hex("receipt_id", logging.Entity(receipt)).
		Logger()

	// whether all collections for the receipt are available locally

	collIndex := int(chunk.CollectionIndex)

	// ensure the collection index specified by the ER is valid
	if len(block.Guarantees) <= collIndex {
		log.Error().
			Int("collection_index", collIndex).
			Msg("could not get collections - invalid collection index")

		// TODO this means the block or receipt is invalid, for now fail fast
		return nil, false
	}

	// request the collection if we don't already have it
	collID := block.Guarantees[collIndex].ID()

	if !e.authCollections.Has(collID) {
		// a collection is missing, the receipt cannot yet be verified
		// TODO rate limit these requests
		err := e.requestCollection(collID)
		if err != nil && err != mempool.ErrEntityAlreadyExists {
			log.Error().
				Err(err).
				Hex("collection_id", logging.ID(collID)).
				Msg("could not request collection")
		}
		return nil, false
	}

	coll, err := e.authCollections.ByID(collID)
	if err != nil {
		// couldn't get the collection from mempool, the receipt cannot be verified
		log.Error().
			Err(err).
			Hex("collection_id", logging.ID(collID)).
			Msg("could not get collection")
		return nil, false
	}

	return coll, true
}

// checkPendingChunks checks all pending chunks of receipts in the mempool and verifies
// any that are ready for verification.
//
// NOTE: this method is protected by mutex to prevent double-verifying ERs.
func (e *Engine) checkPendingChunks() {
	e.checkChunksLock.Lock()
	defer e.checkChunksLock.Unlock()

	// asks for missing blocks
	pending := e.pendingReceipts.All()
	for _, receipt := range pending {
		_, blockReady := e.getBlockForReceipt(receipt)
		if !blockReady {
			continue
		}
		err := e.authReceipts.Add(receipt)
		if err != nil && err != mempool.ErrEntityAlreadyExists {
			e.log.Error().Err(err).
				Hex("receipt_id", logging.ID(receipt.ID())).
				Msg("could not add receipt to the authenticated receipts pool")
		}

		e.pendingReceipts.Rem(receipt.ID())
	}

	receipts := e.authReceipts.All()

	for _, receipt := range receipts {
		block, blockReady := e.getBlockForReceipt(receipt)
		// we can't get collections without the block
		if !blockReady {
			continue
		}

		mychunks, err := e.myChunks(&receipt.ExecutionResult)
		// extracts list of chunks assigned to this Verification node
		if err != nil {
			e.log.Error().
				Err(err).
				Hex("result_id", logging.Entity(receipt.ExecutionResult)).
				Msg("could not fetch the assigned chunks")
			continue
		}

		for _, chunk := range mychunks {
			chunkState, chunkStateReady := e.getChunkStateForReceipt(receipt, chunk.ID())
			if !chunkStateReady {
				// can not verify a chunk without its state, moves to the next chunk
				continue
			}

			// TODO replace chunk state with chunk data pack
			_, chunkDataPackReady := e.getChunkDataPackForReceipt(receipt, chunk.ID())
			if !chunkDataPackReady {
				// can not verify a chunk without its chunk data, moves to the next chunk
				continue
			}

			// retrieves collection corresponding to the chunk
			collection, collectionReady := e.getCollectionForChunk(block, receipt, chunk)
			if !collectionReady {
				// can not verify a chunk without its collection, moves to the next chunk
				continue
			}

			index := chunk.Index
			var endState flow.StateCommitment
			if int(index) == len(receipt.ExecutionResult.Chunks)-1 {
				// last chunk in receipt takes final state commitment
				endState = receipt.ExecutionResult.FinalStateCommit
			} else {
				// any chunk except last takes the subsequent chunk's start state
				endState = receipt.ExecutionResult.Chunks[index+1].StartState
			}

			// creates a verifiable chunk for assigned chunk
			vchunk := &verification.VerifiableChunk{
				ChunkIndex: chunk.Index,
				Receipt:    receipt,
				EndState:   endState,
				Block:      block,
				Collection: collection,
				ChunkState: chunkState,
			}

			// verify the receipt
			err := e.verifierEng.ProcessLocal(vchunk)
			if err != nil {
				e.log.Error().
					Err(err).
					Hex("result_id", logging.Entity(receipt.ExecutionResult)).
					Hex("chunk_id", logging.ID(chunk.ID())).
					Msg("could not pass chunk to verifier engine")
				continue
			}

			// TODO add a tracker to clean the memory up from resources that have already been verified
			// https://github.com/dapperlabs/flow-go/issues/2750
			// clean up would be something like this:
			// cleans the execution receipt from receipts mempool
			// cleans block form blocks mempool
			// cleans all chunk states from chunkStates mempool
			// for now, we clean the mempool from only the collection, as otherwise
			// ingest engine sends the chunk corresponding to this collection everytime
			// this function gets called, since it has everything that is needed to verify this
			e.authCollections.Rem(collection.ID())
		}
	}
}

// myChunks returns the list of chunks in the chunk list that this verifier node
// is assigned to
func (e *Engine) myChunks(res *flow.ExecutionResult) (flow.ChunkList, error) {

	// extracts list of verifier nodes id
	//
	// TODO state extraction should be done based on block references
	// https://github.com/dapperlabs/flow-go/issues/2787
	verifierNodes, err := e.state.Final().
		Identities(filter.HasRole(flow.RoleVerification))
	if err != nil {
		return nil, fmt.Errorf("could not load verifier node IDs: %w", err)
	}

	rng, err := utils.NewChunkAssignmentRNG(res)
	if err != nil {
		return nil, fmt.Errorf("could not generate random generator: %w", err)
	}
	// TODO pull up caching of chunk assignments to here
	a, err := e.assigner.Assign(verifierNodes, res.Chunks, rng)
	if err != nil {
		return nil, fmt.Errorf("could not create chunk assignment %w", err)
	}

	// indices of chunks assigned to this node
	chunkIndices := a.ByNodeID(e.me.NodeID())

	// mine keeps the list of chunks assigned to this node
	mine := make(flow.ChunkList, 0, len(chunkIndices))
	for _, index := range chunkIndices {
		mine = append(mine, res.Chunks.ByIndex(index))
	}

	return mine, nil
}
