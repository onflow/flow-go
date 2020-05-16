package ingest

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/engine/verification/utils"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	verificationmodel "github.com/dapperlabs/flow-go/model/verification"
	trackers "github.com/dapperlabs/flow-go/model/verification/tracker"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// IngestEngine implements the ingest engine of the verification node. It is
// responsible for receiving and handling new execution receipts. It requests
// all dependent resources for each execution receipt and relays a complete
// execution result to the verifier engine when all dependencies are ready.
type Engine struct {
	unit                  *engine.Unit
	log                   zerolog.Logger
	collectionsConduit    network.Conduit
	stateConduit          network.Conduit
	chunksConduit         network.Conduit
	me                    module.Local
	state                 protocol.State
	verifierEng           network.Engine                // for submitting ERs that are ready to be verified
	authReceipts          mempool.Receipts              // keeps receipts with authenticated origin IDs
	pendingReceipts       mempool.PendingReceipts       // keeps receipts pending for their originID to be authenticated
	authCollections       mempool.Collections           // keeps collections with authenticated origin IDs
	pendingCollections    mempool.PendingCollections    // keeps collections pending for their origin IDs to be authenticated
	collectionTrackers    mempool.CollectionTrackers    // keeps track of collection requests that this engine made
	chunkDataPacks        mempool.ChunkDataPacks        // keeps chunk data packs with authenticated origin IDs
	chunkDataPackTackers  mempool.ChunkDataPackTrackers // keeps track of chunk data pack requests that this engine made
	ingestedResultIDs     mempool.Identifiers           // keeps ids of ingested execution results
	ingestedChunkIDs      mempool.Identifiers           // keeps ids of ingested chunks
	ingestedCollectionIDs mempool.Identifiers           // keeps ids collections of ingested chunks
	headerStorage         storage.Headers               // used to check block existence to improve performance
	blockStorage          storage.Blocks                // used to retrieve blocks
	assigner              module.ChunkAssigner          // used to determine chunks this node needs to verify
	requestInterval       uint                          // determines time in milliseconds for retrying tracked requests
	failureThreshold      uint                          // determines number of retries for tracked requests before raising a challenge
}

// New creates and returns a new instance of the ingest engine.
func New(
	log zerolog.Logger,
	net module.Network,
	state protocol.State,
	me module.Local,
	verifierEng network.Engine,
	authReceipts mempool.Receipts,
	pendingReceipts mempool.PendingReceipts,
	authCollections mempool.Collections,
	pendingCollections mempool.PendingCollections,
	collectionTrackers mempool.CollectionTrackers,
	chunkDataPacks mempool.ChunkDataPacks,
	chunkDataPackTrackers mempool.ChunkDataPackTrackers,
	ingestedChunkIDs mempool.Identifiers,
	ingestedResultIDs mempool.Identifiers,
	ingestedCollectionIDs mempool.Identifiers,
	headerStorage storage.Headers,
	blockStorage storage.Blocks,
	assigner module.ChunkAssigner,
	requestIntervalMs uint,
	failureThreshold uint,
) (*Engine, error) {

	e := &Engine{
		unit:                  engine.NewUnit(),
		log:                   log,
		state:                 state,
		me:                    me,
		authReceipts:          authReceipts,
		pendingReceipts:       pendingReceipts,
		verifierEng:           verifierEng,
		authCollections:       authCollections,
		pendingCollections:    pendingCollections,
		collectionTrackers:    collectionTrackers,
		chunkDataPacks:        chunkDataPacks,
		chunkDataPackTackers:  chunkDataPackTrackers,
		ingestedChunkIDs:      ingestedChunkIDs,
		ingestedResultIDs:     ingestedResultIDs,
		ingestedCollectionIDs: ingestedCollectionIDs,
		headerStorage:         headerStorage,
		blockStorage:          blockStorage,
		assigner:              assigner,
		failureThreshold:      failureThreshold,
		requestInterval:       requestIntervalMs,
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
		return nil, fmt.Errorf("could not register chunk data pack provider engine: %w", err)
	}

	_, err = net.Register(engine.ExecutionReceiptProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on execution receipt provider channel: %w", err)
	}

	return e, nil
}

// Ready returns a channel that is closed when the verifier engine is ready.
func (e *Engine) Ready() <-chan struct{} {
	// checks pending chunks every `requestInterval` milliseconds
	e.unit.LaunchPeriodically(e.checkPendingChunks,
		time.Duration(e.requestInterval)*time.Millisecond,
		0)
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
	case *flow.ExecutionReceipt:
		return e.handleExecutionReceipt(originID, resource)
	case *flow.Collection:
		return e.handleCollection(originID, resource)
	case *messages.CollectionResponse:
		return e.handleCollection(originID, &resource.Collection)
	case *messages.ChunkDataPackResponse:
		return e.handleChunkDataPack(originID, &resource.Data)
	default:
		return ErrInvType
	}
}

// handleExecutionReceipt receives an execution receipt, verifies its origin Id and
// accordingly adds it to either the pending receipts or the authenticated receipts mempools
func (e *Engine) handleExecutionReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {
	receiptID := receipt.ID()

	e.log.Info().
		Hex("origin_id", logging.ID(originID)).
		Hex("receipt_id", logging.ID(receiptID)).
		Msg("execution receipt received at ingest engine")

	if e.ingestedResultIDs.Has(receipt.ExecutionResult.ID()) {
		e.log.Debug().
			Hex("origin_id", logging.ID(originID)).
			Hex("receipt_id", logging.ID(receiptID)).
			Msg("execution receipt with already ingested result discarded")
		// discards the receipt if its result has already been ingested
		return nil
	}

	// TODO: correctness check for execution receipts
	// extracts list of verifier nodes id
	origin, err := e.state.AtBlockID(receipt.ExecutionResult.BlockID).Identity(originID)
	if err != nil {
		// TODO: potential attack on authenticity
		// stores ER in pending receipts till a block arrives authenticating this
		ok := e.pendingReceipts.Add(verificationmodel.NewPendingReceipt(receipt, originID))
		e.log.Debug().
			Hex("origin_id", logging.ID(originID)).
			Hex("receipt_id", logging.ID(receiptID)).
			Bool("mempool_insertion", ok).
			Msg("execution receipt added to pending mempool")

	} else {
		// execution results are only valid from execution nodes
		if origin.Role != flow.RoleExecution {
			// TODO: potential attack on integrity
			return fmt.Errorf("invalid role for generating an execution receipt, id: %s, role: %s", origin.NodeID, origin.Role)
		}

		// store the execution receipt in the store of the engine
		// this will fail if the receipt already exists in the store
		ok := e.authReceipts.Add(receipt)
		e.log.Debug().
			Hex("origin_id", logging.ID(originID)).
			Hex("receipt_id", logging.ID(receiptID)).
			Bool("mempool_insertion", ok).
			Msg("execution receipt added to authenticated mempool")

	}
	return nil
}

// handleChunkDataPack receives a chunk data pack, verifies its origin ID, and stores that in the mempool
func (e *Engine) handleChunkDataPack(originID flow.Identifier, chunkDataPack *flow.ChunkDataPack) error {
	e.log.Info().
		Hex("origin_id", logging.ID(originID)).
		Hex("chunk_data_pack_id", logging.Entity(chunkDataPack)).
		Msg("chunk data pack received")

	if e.ingestedChunkIDs.Has(chunkDataPack.ChunkID) {
		// belongs to an already ingested chunk
		// discards the chunk data pack
		e.log.Debug().
			Hex("origin_id", logging.ID(originID)).
			Hex("chunk_id", logging.ID(chunkDataPack.ChunkID)).
			Msg("discards the chunk data pack of an already ingested chunk")
		return nil
	}

	if e.chunkDataPacks.Has(chunkDataPack.ChunkID) {
		// discards an already existing chunk data pack
		e.log.Debug().
			Hex("origin_id", logging.ID(originID)).
			Hex("chunk_id", logging.ID(chunkDataPack.ChunkID)).
			Msg("discards the already exisiting chunk data pack")
		return nil
	}

	if !e.chunkDataPackTackers.Has(chunkDataPack.ChunkID) {
		// does not have a valid tracker
		// discards the chunk data pack
		e.log.Debug().
			Hex("origin_id", logging.ID(originID)).
			Hex("chunk_id", logging.ID(chunkDataPack.ChunkID)).
			Msg("discards the chunk data pack with no tracker")
		return nil
	}

	tracker, ok := e.chunkDataPackTackers.ByChunkID(chunkDataPack.ChunkID)
	if !ok {
		e.log.Debug().
			Hex("origin_id", logging.ID(originID)).
			Hex("chunk_id", logging.ID(chunkDataPack.ChunkID)).
			Msg("cannot retrieve tracker for received chunk data pack")
		return fmt.Errorf("no tracker available for chunk ID: %x", chunkDataPack.ChunkID)
	}

	// checks the authenticity of origin ID
	origin, err := e.state.AtBlockID(tracker.BlockID).Identity(originID)
	if err != nil {
		// TODO: potential attack on authenticity
		e.log.Debug().
			Err(err).
			Hex("origin_id", logging.ID(originID)).
			Hex("chunk_id", logging.ID(chunkDataPack.ChunkID)).
			Msg("invalid origin ID for chunk data pack")
		return fmt.Errorf("invalid origin id (%s): %w", originID[:], err)
	}

	// chunk data pack should only be sent by an execution node
	if origin.Role != flow.RoleExecution {
		// TODO: potential attack on integrity
		e.log.Debug().
			Hex("origin_id", logging.ID(originID)).
			Str("origin_role", origin.Role.String()).
			Hex("chunk_id", logging.ID(chunkDataPack.ChunkID)).
			Msg("invalid origin role for chunk data pack")
		return fmt.Errorf("invalid role for generating an execution receipt, id: %s, role: %s", origin.NodeID, origin.Role)
	}

	// store the chunk data pack in the store of the engine
	// this will fail if the receipt already exists in the store
	added := e.chunkDataPacks.Add(chunkDataPack)
	if !added {
		e.log.Debug().
			Err(err).
			Hex("origin_id", logging.ID(originID)).
			Hex("chunk_id", logging.ID(chunkDataPack.ChunkID)).
			Msg("could not store chunk data pack")
		return fmt.Errorf("could not store chunk data pack: %w", err)
	}

	// removes chunk data pack tracker from  mempool
	e.chunkDataPackTackers.Rem(chunkDataPack.ChunkID)
	e.log.Debug().
		Hex("origin_id", logging.ID(originID)).
		Hex("chunk_id", logging.ID(chunkDataPack.ChunkID)).
		Msg("chunk data pack stored in mempool, tracker removed")

	return nil
}

// handleCollection handles receipt of a new collection, either via push or
// after a request. It adds the collection to the mempool and checks for
// pending receipts that are ready for verification.
func (e *Engine) handleCollection(originID flow.Identifier, coll *flow.Collection) error {
	collID := coll.ID()

	e.log.Info().
		Hex("origin_id", logging.ID(originID)).
		Hex("collection_id", logging.ID(collID)).
		Msg("collection received")

	// drops already ingested collection
	if e.ingestedCollectionIDs.Has(collID) {
		e.log.Info().
			Hex("origin_id", logging.ID(originID)).
			Hex("collection_id", logging.ID(collID)).
			Msg("drops ingested collection")
		return nil
	}

	// drops collection if it is residing on the mempools
	if e.authCollections.Has(collID) || e.pendingCollections.Has(collID) {
		e.log.Info().
			Hex("origin_id", logging.ID(originID)).
			Hex("collection_id", logging.ID(collID)).
			Msg("drops existing collection")
		return nil
	}

	if e.collectionTrackers.Has(collID) {
		// this event is a reply to a prior request
		tracker, ok := e.collectionTrackers.ByCollectionID(collID)
		if !ok {
			return fmt.Errorf("could not retrieve collection tracker from mempool")
		}

		// a tracker exists for the requesting collection
		// verifies identity of origin
		origin, err := e.state.AtBlockID(tracker.BlockID).Identity(originID)
		if err != nil {
			return fmt.Errorf("invalid origin id (%s): %w", origin, err)
		}

		if origin.Role != flow.RoleCollection {
			return fmt.Errorf("invalid role for receiving collection: %s", origin.Role)
		}

		// adds collection to authenticated mempool
		added := e.authCollections.Add(coll)
		if !added {
			return fmt.Errorf("could not add collection to mempool")
		}

		// removes tracker
		e.collectionTrackers.Rem(collID)

		e.log.Debug().
			Hex("origin_id", logging.ID(originID)).
			Hex("collection_id", logging.ID(collID)).
			Msg("collection added to authenticated mempool, and tracker removed")
	} else {
		// this collection came passively
		// collections with no tracker add to the pending collections mempool
		ok := e.pendingCollections.Add(verificationmodel.NewPendingCollection(coll, originID))
		e.log.Debug().
			Hex("origin_id", logging.ID(originID)).
			Hex("collection_id", logging.ID(collID)).
			Bool("mempool_insertion", ok).
			Msg("collection added to pending mempool")
	}

	return nil
}

// requestCollection submits a request for the given collection to collection nodes,
// or drops and logs the request if the tracker associated with the request goes beyond the
// failure threshold
func (e *Engine) requestCollection(collID, blockID flow.Identifier) error {
	// updates tracker for this request
	ct, err := e.updateCollectionTracker(collID, blockID)
	if err != nil {
		return fmt.Errorf("could not update the collection tracker: %w", err)
	}

	// checks against maximum retries
	if ct.Counter > e.failureThreshold {
		// tracker met maximum retry chances
		// no longer retried
		// TODO raise a missing collection challenge
		// TODO drop tracker from memory once the challenge gets accepted, or trackers has nonce
		return fmt.Errorf("collection tracker met maximum retries, no longer retried, chunk ID: %x", collID)
	}

	// extracts list of collection nodes id
	//
	collNodes, err := e.state.Final().Identities(filter.HasRole(flow.RoleCollection))
	if err != nil {
		return fmt.Errorf("could not load collection node identities: %w", err)
	}

	req := &messages.CollectionRequest{
		ID:    collID,
		Nonce: rand.Uint64(),
	}

	// TODO we should only submit to cluster which owns the collection
	err = e.collectionsConduit.Submit(req, collNodes.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not submit request for collection (id=%s): %w", collID, err)
	}

	e.log.Debug().
		Hex("collection_id", logging.ID(collID)).
		Hex("block_id", logging.ID(blockID)).
		Msg("collection request submitted")
	return nil
}

// requestChunkDataPack submits a request for the given chunk ID to the execution nodes,
// or drops and logs the request if the tracker associated with the request goes beyond the
// failure threshold
func (e *Engine) requestChunkDataPack(chunkID, blockID flow.Identifier) error {
	// updates tracker for this request
	cdpt, err := e.updateChunkDataPackTracker(chunkID, blockID)
	if err != nil {
		return fmt.Errorf("could not update the chunk data pack tracker: %w", err)
	}
	// checks against maximum retries
	if cdpt.Counter > e.failureThreshold {
		// tracker met maximum retry chances
		// no longer retried
		// TODO raise a missing chunk data pack challenge
		// TODO drop tracker from memory once the challenge gets accepted, or trackers has nonce
		return fmt.Errorf("chunk data pack tracker met maximum retries, no longer retried, chunk ID: %x", chunkID)
	}

	// extracts list of execution nodes
	//
	execNodes, err := e.state.Final().Identities(filter.HasRole(flow.RoleExecution))
	if err != nil {
		return fmt.Errorf("could not load execution nodes identities: %w", err)
	}

	req := &messages.ChunkDataPackRequest{
		ChunkID: chunkID,
		Nonce:   rand.Uint64(),
	}

	// TODO we should only submit to execution node that generated execution receipt
	err = e.chunksConduit.Submit(req, execNodes.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not submit request for collection (id=%s): %w", chunkID, err)
	}

	e.log.Debug().
		Hex("chunk_id", logging.ID(chunkID)).
		Msg("chunk data pack request submitted")

	return nil
}

// getBlockForReceipt checks the block referenced by the given receipt. If the
// block is available locally, returns true and the block. Otherwise, returns
// false and requests the block.
func (e *Engine) getBlockForReceipt(receipt *flow.ExecutionReceipt) (*flow.Block, bool) {
	// ensure we have the block corresponding to this pending execution receipt
	block, err := e.blockStorage.ByID(receipt.ExecutionResult.BlockID)
	if err != nil {
		// block is not ready for retrieval. Should wait for the consensus follower.
		return nil, false
	}

	return block, true
}

// getChunkDataPackForReceipt checks the chunk data pack associated with a chunk ID and
// execution receipt. If the chunk data pack is available locally, returns true
// as well as the chunk data pack itself.
func (e *Engine) getChunkDataPackForReceipt(receipt *flow.ExecutionReceipt, chunkID flow.Identifier) (*flow.ChunkDataPack, bool) {
	log := e.log.With().
		Hex("block_id", logging.ID(receipt.ExecutionResult.BlockID)).
		Hex("chunk_id", logging.ID(chunkID)).
		Hex("receipt_id", logging.Entity(receipt)).
		Logger()

	// checks mempool
	//
	if e.chunkDataPacks.Has(chunkID) {
		// chunk data pack exists and retrieved and returned
		chunkDataPack, exists := e.chunkDataPacks.ByChunkID(chunkID)
		if !exists {
			// couldn't get chunk state from mempool, the chunk cannot yet be verified
			log.Error().Msg("could not get chunk data pack from mempool")
			return nil, false
		}
		return chunkDataPack, true
	}

	// requests the chunk data pack from network
	err := e.requestChunkDataPack(chunkID, receipt.ExecutionResult.BlockID)
	if err != nil {
		log.Error().
			Err(err).
			Hex("chunk_id", logging.ID(chunkID)).
			Msg("could not make a request of chunk data pack to the network")
	}

	return nil, false
}

// getCollectionForChunk checks the collection depended on the
// given execution receipt and chunk. Returns true if the collections is available
// locally. If the collections is not available locally, registers a tracker for it.
func (e *Engine) getCollectionForChunk(block *flow.Block, receipt *flow.ExecutionReceipt, chunk *flow.Chunk) (*flow.Collection, bool) {

	log := e.log.With().
		Hex("block_id", logging.ID(block.ID())).
		Hex("receipt_id", logging.Entity(receipt)).
		Logger()

	collIndex := int(chunk.CollectionIndex)

	// ensure the collection index specified by the ER is valid
	if len(block.Payload.Guarantees) <= collIndex {
		log.Error().
			Int("collection_index", collIndex).
			Msg("could not get collections - invalid collection index")

		// TODO this means the block or receipt is invalid, for now fail fast
		return nil, false
	}

	collID := block.Payload.Guarantees[collIndex].ID()

	// updates pending collection
	e.checkPendingCollections(collID, block.ID())

	// checks authenticated collections mempool
	//
	//
	if e.authCollections.Has(collID) {
		coll, exists := e.authCollections.ByID(collID)
		if !exists {
			// couldn't get the collection from mempool
			log.Error().
				Hex("collection_id", logging.ID(collID)).
				Msg("could not get collection from authenticated pool")
			return nil, false
		}

		log.Debug().
			Hex("collection_id", logging.ID(collID)).
			Hex("chunk_id", logging.ID(chunk.ID())).
			Msg("collection is resolved from authenticated mempool")

		return coll, true
	}

	// requests the collection from network
	err := e.requestCollection(collID, block.ID())
	if err != nil {
		log.Error().
			Err(err).
			Hex("collection_id", logging.ID(collID)).
			Msg("could make a request of collection to the network")
	}

	log.Debug().
		Hex("collection_id", logging.ID(collID)).
		Hex("chunk_id", logging.ID(chunk.ID())).
		Msg("collection for chunk requested")

	return nil, false
}

// checkPendingChunks checks all pending chunks of receipts in the mempool and verifies
// any that are ready for verification.
//
// NOTE: this method is protected by mutex to prevent double-verifying ERs.
func (e *Engine) checkPendingChunks() {
	e.log.Debug().
		Msg("check pending chunks background service started")

	// checks the current authenticated receipts for their resources
	// ready for verification
	receipts := e.authReceipts.All()
	for _, receipt := range receipts {
		block, blockReady := e.getBlockForReceipt(receipt)
		// we can't get collections without the block
		if !blockReady {
			continue
		}

		if receipt.ExecutionResult.Chunks.Len() == 0 {
			// TODO potential attack on availability
			e.log.Error().
				Hex("receipt_id", logging.Entity(receipt)).
				Hex("result_id", logging.Entity(receipt.ExecutionResult)).
				Msg("could not ingest execution result with zero chunks")

			continue
		}

		mychunks, err := e.myUningestedChunks(&receipt.ExecutionResult)
		// extracts list of chunks assigned to this Verification node
		if err != nil {
			e.log.Error().
				Err(err).
				Hex("result_id", logging.Entity(receipt.ExecutionResult)).
				Msg("could not fetch assigned chunks")
			continue
		}

		e.log.Debug().
			Hex("receipt_id", logging.Entity(receipt)).
			Hex("result_id", logging.Entity(receipt.ExecutionResult)).
			Int("total_chunks", receipt.ExecutionResult.Chunks.Len()).
			Int("assigned_chunks", len(mychunks)).
			Msg("chunk assignment is done")

		for _, chunk := range mychunks {

			// TODO replace chunk state with chunk data pack
			chunkDatapack, chunkDataPackReady := e.getChunkDataPackForReceipt(receipt, chunk.ID())
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
				ChunkIndex:    chunk.Index,
				Receipt:       receipt,
				EndState:      endState,
				Block:         block,
				Collection:    collection,
				ChunkDataPack: chunkDatapack,
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

			// does resource cleanup
			e.onChunkIngested(vchunk)

			e.log.Debug().
				Hex("result_id", logging.Entity(receipt.ExecutionResult)).
				Hex("chunk_id", logging.ID(chunk.ID())).
				Msg("chunk successfully ingested")
		}
	}
}

// onChunkIngested is called whenever a verifiable chunk is formed for a
// chunk and is sent to the verify engine successfully.
// It cleans up all resources associated with this chunk
func (e *Engine) onChunkIngested(vc *verification.VerifiableChunk) {
	// marks this chunk as ingested
	ok := e.ingestedChunkIDs.Add(vc.ChunkDataPack.ChunkID)
	if !ok {
		e.log.Error().
			Hex("chunk_id", logging.ID(vc.ChunkDataPack.ChunkID)).
			Msg("could not add chunk to ingested chunks mempool")
	}

	// marks collection corresponding to this chunk as ingested
	ok = e.ingestedCollectionIDs.Add(vc.Collection.ID())
	if !ok {
		e.log.Error().
			Hex("chunk_id", logging.ID(vc.ChunkDataPack.ChunkID)).
			Hex("chunk_id", logging.ID(vc.Collection.ID())).
			Msg("could not add collection to ingested collections mempool")
	}

	// cleans up resources of the ingested chunk from mempools
	e.authCollections.Rem(vc.Collection.ID())
	e.chunkDataPacks.Rem(vc.ChunkDataPack.ID())

	mychunks, err := e.myUningestedChunks(&vc.Receipt.ExecutionResult)
	// extracts list of chunks assigned to this Verification node
	if err != nil {
		e.log.Error().
			Err(err).
			Hex("result_id", logging.Entity(vc.Receipt.ExecutionResult)).
			Msg("could not fetch assigned chunks")
		return
	}

	if len(mychunks) == 0 {
		// no un-ingested chunk remains with this receipt
		// marks execution result as ingested
		added := e.ingestedResultIDs.Add(vc.Receipt.ExecutionResult.ID())

		if !added {
			e.log.Error().
				Hex("result_id", logging.Entity(vc.Receipt.ExecutionResult)).
				Msg("could add ingested result to mempool")
		}
		// removes receipt from mempool to avoid further iteration
		e.authReceipts.Rem(vc.Receipt.ID())

		// removes all pending and authenticated receipts with the same result
		// pending receipts
		for _, p := range e.pendingReceipts.All() {
			// TODO check for nil dereferencing
			if e.ingestedResultIDs.Has(p.Receipt.ExecutionResult.ID()) {
				e.pendingReceipts.Rem(p.Receipt.ID())
			}
		}

		// authenticated receipts
		for _, areceipt := range e.authReceipts.All() {
			// TODO check for nil dereferencing
			if e.ingestedResultIDs.Has(areceipt.ExecutionResult.ID()) {
				e.authReceipts.Rem(areceipt.ID())
			}
		}
	}
}

// myUningestedChunks returns the list of chunks in the chunk list that this verifier node
// is assigned to, and are not ingested yet. A chunk is ingested once a verifiable chunk is
// formed out of it and is passed to verify engine
func (e *Engine) myUningestedChunks(res *flow.ExecutionResult) (flow.ChunkList, error) {

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
		chunk, ok := res.Chunks.ByIndex(index)
		if !ok {
			return nil, fmt.Errorf("chunk out of range requested: %v", index)
		}
		// discard the chunk if it has been already ingested
		if e.ingestedChunkIDs.Has(chunk.ID()) {
			continue
		}
		mine = append(mine, chunk)
	}

	return mine, nil
}

// checkPendingReceipts iterates over all pending receipts
// if any receipt has the `blockID`, it evaluates the receipt's origin ID
// if originID is evaluated successfully, the receipt is added to authenticated receipts mempool
// Otherwise it is dropped completely
func (e *Engine) checkPendingReceipts(blockID flow.Identifier) {
	e.log.Info().
		Hex("block_id", logging.ID(blockID)).
		Msg("pending receipts are checking against finalized block")

	for _, p := range e.pendingReceipts.All() {
		if blockID == p.Receipt.ExecutionResult.BlockID {
			// removes receipt from pending receipts pool
			e.pendingReceipts.Rem(p.Receipt.ID())

			// evaluates receipt origin ID at the block it refers to
			origin, err := e.state.AtBlockID(blockID).Identity(p.OriginID)
			if err != nil {
				// could not verify origin Id of pending receipt based on its referenced block
				// drops it
				// TODO: potential attack on authenticity
				e.log.Error().
					Err(err).
					Hex("receipt_id", logging.ID(p.Receipt.ID())).
					Hex("origin_id", logging.ID(p.OriginID)).
					Msg("could not verify origin ID of pending receipt")
				continue
			}

			// execution results are only valid from execution nodes
			if origin.Role != flow.RoleExecution {
				// TODO: potential attack on integrity
				e.log.Error().
					Err(err).
					Hex("receipt_id", logging.ID(p.Receipt.ID())).
					Hex("origin_id", logging.ID(origin.NodeID)).
					Uint8("origin_role", uint8(origin.Role)).
					Msg("invalid role for pending execution receipt")
				continue
			}

			// store the execution receipt in the authenticated mempool of the engine
			// this will fail if the receipt already exists in the store
			_ = e.authReceipts.Add(p.Receipt)

			e.log.Debug().
				Hex("receipt_id", logging.ID(p.Receipt.ID())).
				Hex("block_id", logging.ID(blockID)).
				Msg("pending receipt moved to authenticated mempool")
		}
	}
}

// checkPendingCollections checks if a certain collection is available for the requested block.
// if the collection is available, it evaluates the collections's origin ID based on the block ID.
// if originID is evaluated successfully, the collection is added to authenticated collections mempool.
// Otherwise it is dropped completely.
func (e *Engine) checkPendingCollections(collID, blockID flow.Identifier) {
	if !e.pendingCollections.Has(collID) {
		e.log.Debug().
			Hex("collection_id", logging.ID(collID)).
			Hex("block_id", logging.ID(blockID)).
			Msg("no pending collection is available with this parameters")
		return
	}

	// retrieves collection from mempool
	pcoll, exists := e.pendingCollections.ByID(collID)
	if !exists {
		e.log.Error().
			Hex("collection_id", logging.ID(collID)).
			Msg("could not retrieve collection from pending mempool")
	}
	// removes collection from pending pool
	e.pendingCollections.Rem(collID)

	// evaluates origin ID of pending collection at the block it is referenced
	origin, err := e.state.AtBlockID(blockID).Identity(pcoll.OriginID)
	if err != nil {
		// could not verify origin ID of pending collection based on its referenced block
		// drops it
		// TODO: potential attack on authenticity
		e.log.Error().
			Err(err).
			Hex("collection_id", logging.ID(pcoll.ID())).
			Hex("origin_id", logging.ID(pcoll.OriginID)).
			Msg("could not verify origin ID of pending collection")
		return
	}
	// collections should come from collection nodes
	if origin.Role != flow.RoleCollection {
		// TODO: potential attack on integrity
		e.log.Error().
			Err(err).
			Hex("collection_id", logging.ID(pcoll.ID())).
			Hex("origin_id", logging.ID(pcoll.OriginID)).
			Str("origin_role", origin.Role.String()).
			Msg("invalid role for pending collection")
		return
	}
	// store the collection in the authenticated collections mempool
	// this will fail if the collection already exists in the store
	_ = e.authCollections.Add(pcoll.Collection)

	e.log.Debug().
		Hex("collection_id", logging.ID(collID)).
		Hex("block_id", logging.ID(blockID)).
		Msg("pending collection successfully moved to authenticated mempool")
}

// updateChunkDataPackTracker performs the following
// If there is a tracker for this chunk ID, it increases its counter by one in place
// Else it creates a new empty tracker with counter value of one and stores it in the trackers mempool
func (e *Engine) updateChunkDataPackTracker(chunkID flow.Identifier, blockID flow.Identifier) (*trackers.ChunkDataPackTracker, error) {
	var cdpt *trackers.ChunkDataPackTracker

	if e.chunkDataPackTackers.Has(chunkID) {
		// there is a tracker for this chunk
		// increases its counter
		t, err := e.chunkDataPackTackers.Inc(chunkID)
		if err != nil {
			return nil, fmt.Errorf("could not update chunk data pack tracker: %w", err)
		}
		cdpt = t
	} else {
		// creates a new chunk data pack tracker and stores in in memory
		cdpt = trackers.NewChunkDataPackTracker(chunkID, blockID)
		ok := e.chunkDataPackTackers.Add(cdpt)
		if !ok {
			return nil, fmt.Errorf("could not store tracker of chunk data pack request in mempool")
		}
	}

	return cdpt, nil
}

// updateCollectionTracker performs the following
// If there is a tracker for this collection ID, it pulls it out of mempool, increases its counter by one, and returns it
// Else it creates a new empty tracker for this collection with counter value of one and returns it.
func (e *Engine) updateCollectionTracker(collectionID flow.Identifier, blockID flow.Identifier) (*trackers.CollectionTracker, error) {
	var ct *trackers.CollectionTracker

	if e.collectionTrackers.Has(collectionID) {
		// there is a tracker for this collection
		// increases its counter
		t, err := e.collectionTrackers.Inc(collectionID)
		if err != nil {
			return nil, fmt.Errorf("could not update collection tracker: %w", err)
		}
		ct = t
	} else {
		// creates a new collection tracker and stores in in memory
		ct = trackers.NewCollectionTracker(collectionID, blockID)
		ok := e.collectionTrackers.Add(ct)
		if !ok {
			return nil, fmt.Errorf("could not store tracker of collection request in mempool")
		}
	}

	return ct, nil
}

// To implement FinalizationConsumer
func (e *Engine) OnBlockIncorporated(*model.Block) {

}

// OnFinalizedBlock is part of implementing FinalizationConsumer interface
//
// OnFinalizedBlock notifications are produced by the Finalization Logic whenever
// a block has been finalized. They are emitted in the order the blocks are finalized.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
func (e *Engine) OnFinalizedBlock(block *model.Block) {

	// block should be in the storage
	_, err := e.headerStorage.ByBlockID(block.BlockID)
	if errors.Is(err, storage.ErrNotFound) {
		e.log.Error().
			Hex("block_id", logging.ID(block.BlockID)).
			Msg("block is not available in storage")
		return
	}
	if err != nil {
		e.log.Error().
			Hex("block_id", logging.ID(block.BlockID)).
			Msg("could not check block availability in storage")
		return
	}

	// checks pending receipts in parallel and non-blocking based on new block ID
	_ = e.unit.Do(func() error {
		e.checkPendingReceipts(block.BlockID)
		return nil
	})
}

// To implement FinalizationConsumer
func (e *Engine) OnDoubleProposeDetected(*model.Block, *model.Block) {}
