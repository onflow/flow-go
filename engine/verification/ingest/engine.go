package ingest

import (
	"fmt"
	"math/rand"
	"sync"
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
	unit                 *engine.Unit
	log                  zerolog.Logger
	collectionsConduit   network.Conduit
	stateConduit         network.Conduit
	chunksConduit        network.Conduit
	me                   module.Local
	state                protocol.State
	verifierEng          network.Engine                // for submitting ERs that are ready to be verified
	authReceipts         mempool.Receipts              // keeps receipts with authenticated origin IDs
	pendingReceipts      mempool.PendingReceipts       // keeps receipts pending for their originID to be authenticated
	authCollections      mempool.Collections           // keeps collections with authenticated origin IDs
	pendingCollections   mempool.PendingCollections    // keeps collections pending for their origin IDs to be authenticated
	collectionTrackers   mempool.CollectionTrackers    // keeps track of collection requests that this engine made
	chunkDataPacks       mempool.ChunkDataPacks        // keeps chunk data packs with authenticated origin IDs
	chunkDataPackTackers mempool.ChunkDataPackTrackers // keeps track of chunk data pack requests that this engine made
	ingestedResultIDs    mempool.Identifiers           // keeps ids of ingested execution results
	ingestedChunkIDs     mempool.Identifiers           // keeps ids of ingested chunks
	blockStorage         storage.Blocks
	checkChunksLock      sync.Mutex           // protects the checkPendingChunks method to prevent double-verifying
	checkTrackerLock     sync.Mutex           // provides mutual exclusion for checkTrackers method to prevent race condition
	assigner             module.ChunkAssigner // used to determine chunks this node needs to verify
	requestInterval      uint                 // determines time in milliseconds for retrying tracked requests
	failureThreshold     uint                 // determines number of retries for tracked requests before raising a challenge
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
	blockStorage storage.Blocks,
	assigner module.ChunkAssigner,
	requestIntervalMs uint,
	failureThreshold uint,
) (*Engine, error) {

	e := &Engine{
		unit:                 engine.NewUnit(),
		log:                  log,
		state:                state,
		me:                   me,
		authReceipts:         authReceipts,
		pendingReceipts:      pendingReceipts,
		verifierEng:          verifierEng,
		authCollections:      authCollections,
		pendingCollections:   pendingCollections,
		collectionTrackers:   collectionTrackers,
		chunkDataPacks:       chunkDataPacks,
		chunkDataPackTackers: chunkDataPackTrackers,
		ingestedChunkIDs:     ingestedChunkIDs,
		ingestedResultIDs:    ingestedResultIDs,
		blockStorage:         blockStorage,
		assigner:             assigner,
		failureThreshold:     failureThreshold,
		requestInterval:      requestIntervalMs,
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
	// runs cleanup periodically every `requestInterval` milliseconds
	e.unit.LaunchPeriodically(e.checkTrackers,
		time.Duration(e.requestInterval)*time.Millisecond)
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

// handleExecutionReceipt receives an execution receipt (exrcpt), verifies that and emits
// a result approval upon successful verification
func (e *Engine) handleExecutionReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {
	e.log.Info().
		Hex("origin_id", logging.ID(originID)).
		Hex("receipt_id", logging.Entity(receipt)).
		Msg("execution receipt received")

	if e.ingestedResultIDs.Has(receipt.ExecutionResult.ID()) {
		// discards the receipt if its result has already been ingested
		return nil
	}

	// TODO: correctness check for execution receipts
	// extracts list of verifier nodes id
	origin, err := e.state.AtBlockID(receipt.ExecutionResult.BlockID).Identity(originID)
	if err != nil {
		// TODO: potential attack on authenticity
		// stores ER in pending receipts till a block arrives authenticating this
		preceipt := &verificationmodel.PendingReceipt{
			Receipt:  receipt,
			OriginID: originID,
		}
		err = e.pendingReceipts.Add(preceipt)
		if err != nil && err != mempool.ErrAlreadyExists {
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
		if err != nil && err != mempool.ErrAlreadyExists {
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

	if e.ingestedChunkIDs.Has(chunkDataPack.ChunkID) {
		// discards the chunk data pack if it belongs to an already ingested chunk
		return nil
	}

	// checks if this event is a reply of a prior request
	// extracts the tracker
	tracker, err := e.chunkDataPackTackers.ByChunkID(chunkDataPack.ChunkID)
	if err != nil {
		return fmt.Errorf("no tracker available for chunk ID: %x", chunkDataPack.ChunkID)
	}

	// checks the authenticity of origin ID
	origin, err := e.state.AtBlockID(tracker.BlockID).Identity(originID)
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
	err = e.chunkDataPacks.Add(chunkDataPack)
	if err != nil {
		return fmt.Errorf("could not store execution receipt: %w", err)
	}

	// removes chunk data pack tracker from  mempool
	e.chunkDataPackTackers.Rem(chunkDataPack.ChunkID)

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

	// checks if this event is a reply of a prior request extracts the tracker
	collID := coll.ID()
	tracker, err := e.collectionTrackers.ByCollectionID(collID)
	if err != nil {
		// collections with no tracker add to the pending collections mempool
		pcoll := &verificationmodel.PendingCollection{
			Collection: coll,
			OriginID:   originID,
		}
		err = e.pendingCollections.Add(pcoll)
		if err != nil && err != mempool.ErrAlreadyExists {
			return fmt.Errorf("could not store collection in pending pool: %w", err)
		}
	} else {
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
		err = e.authCollections.Add(coll)
		if err != nil {
			return fmt.Errorf("could not add collection to mempool: %w", err)
		}

		// removes tracker
		e.collectionTrackers.Rem(collID)

		e.checkPendingChunks()
	}
	return nil
}

// requestCollection submits a request for the given collection to collection nodes.
func (e *Engine) requestCollection(collID flow.Identifier) error {
	// updates tracker for this request
	tracker, err := e.updateCollectionTracker(collID)
	if err != nil {
		return fmt.Errorf("could not update the collection tracker: %w", err)
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
		Msg("collection request submitted")

	// stores collection tracker in the memory
	err = e.collectionTrackers.Add(tracker)
	// Todo handle the case of duplicate trackers
	if err != nil && err != mempool.ErrAlreadyExists {
		return fmt.Errorf("could not store tracker of collection request in mempool: %w", err)
	}
	return nil
}

// requestChunkDataPack submits a request for the given chunk ID to the execution nodes.
func (e *Engine) requestChunkDataPack(chunkID flow.Identifier) error {
	// updates tracker for this request
	tracker, err := e.updateChunkDataPackTracker(chunkID)
	if err != nil {
		return fmt.Errorf("could not update the chunk data pack tracker: %w", err)
	}

	// extracts list of execution nodes
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

	e.log.Debug().
		Hex("chunk_id", logging.ID(chunkID)).
		Msg("chunk data pack request submitted")

	// stores chunk data pack tracker in the memory
	err = e.chunkDataPackTackers.Add(tracker)
	// TODO handle the case of duplicate trackers
	if err != nil && err != mempool.ErrAlreadyExists {
		return fmt.Errorf("could not store tracker of chunk data pack request in mempool: %w", err)
	}
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
		chunkDataPack, err := e.chunkDataPacks.ByChunkID(chunkID)
		if err != nil {
			// couldn't get chunk state from mempool, the chunk cannot yet be verified
			log.Error().
				Err(err).
				Msg("could not get chunk data pack from mempool")
			return nil, false
		}
		return chunkDataPack, true
	}

	// checks trackers
	//
	if e.chunkDataPackTackers.Has(chunkID) {
		// chunk data pack has been already requested from network
		return nil, false
	}

	// registers a tracker for the chunk data pack
	//
	tracker := &trackers.ChunkDataPackTracker{
		ChunkID: chunkID,
		BlockID: receipt.ExecutionResult.BlockID,
		Counter: 0,
	}
	err := e.chunkDataPackTackers.Add(tracker)
	// TODO handle the case of duplicate trackers
	if err != nil && err != mempool.ErrAlreadyExists {
		e.log.Error().
			Err(err).
			Hex("chunk_id", logging.ID(chunkID)).
			Msg("could not store tracker of chunk data pack request in mempool")
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
	if len(block.Guarantees) <= collIndex {
		log.Error().
			Int("collection_index", collIndex).
			Msg("could not get collections - invalid collection index")

		// TODO this means the block or receipt is invalid, for now fail fast
		return nil, false
	}

	collID := block.Guarantees[collIndex].ID()

	// checks authenticated collections mempool
	//
	//
	if e.authCollections.Has(collID) {
		coll, err := e.authCollections.ByID(collID)
		if err != nil {
			// couldn't get the collection from mempool
			log.Error().
				Err(err).
				Hex("collection_id", logging.ID(collID)).
				Msg("could not get collection from authenticated pool")
			return nil, false
		}
		return coll, true
	}

	// checks collection trackers mempool
	//
	if e.collectionTrackers.Has(collID) {
		// collection has been already requested from network
		return nil, false
	}

	// registers a tracker for collection
	//
	tracker := &trackers.CollectionTracker{
		CollectionID: collID,
		BlockID:      block.ID(),
		Counter:      0,
	}

	err := e.collectionTrackers.Add(tracker)
	// Todo handle the case of duplicate trackers
	if err != nil && err != mempool.ErrAlreadyExists {
		e.log.Error().
			Err(err).
			Hex("collection_id", logging.ID(collID)).
			Msg("could not store tracker collection request in mempool")
	}
	return nil, false
}

// checkPendingChunks checks all pending chunks of receipts in the mempool and verifies
// any that are ready for verification.
//
// NOTE: this method is protected by mutex to prevent double-verifying ERs.
func (e *Engine) checkPendingChunks() {
	e.checkChunksLock.Lock()
	defer e.checkChunksLock.Unlock()

	// checks the current authenticated receipts for their resources
	// ready for verification
	receipts := e.authReceipts.All()
	for _, receipt := range receipts {
		block, blockReady := e.getBlockForReceipt(receipt)
		// we can't get collections without the block
		if !blockReady {
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
		}
	}
}

// onChunkIngested is called whenever a verifiable chunk is formed for a
// chunk and is sent to the verify engine successfully.
// It cleans up all resources associated with this chunk
func (e *Engine) onChunkIngested(vc *verification.VerifiableChunk) {
	// marks this chunk as ingested
	err := e.ingestedChunkIDs.Add(vc.ChunkDataPack.ChunkID)
	if err != nil {
		e.log.Error().
			Err(err).
			Hex("chunk_id", logging.ID(vc.ChunkDataPack.ChunkID)).
			Msg("could not add chunk to ingested chunks mempool")
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
		err := e.ingestedResultIDs.Add(vc.Receipt.ExecutionResult.ID())

		if err != nil {
			e.log.Error().
				Err(err).
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
	for _, p := range e.pendingReceipts.All() {
		if blockID == p.Receipt.ExecutionResult.BlockID {
			// removes receipt from pending receipts pool
			e.pendingReceipts.Rem(p.Receipt.ID())
			// adds receipt to authenticated receipts pool
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
			} else {
				// execution results are only valid from execution nodes
				if origin.Role != flow.RoleExecution {
					// TODO: potential attack on integrity
					e.log.Error().
						Err(err).
						Hex("receipt_id", logging.ID(p.Receipt.ID())).
						Hex("origin_id", logging.ID(origin.NodeID)).
						Uint8("origin_role", uint8(origin.Role)).
						Msg("invalid role for pending execution receipt")
				}
				// store the execution receipt in the store of the engine
				// this will fail if the receipt already exists in the store
				err = e.authReceipts.Add(p.Receipt)
				if err != nil && err != mempool.ErrAlreadyExists {
					// TODO potential memory leakage
					e.log.Error().
						Err(err).
						Hex("receipt_id", logging.ID(p.Receipt.ID())).
						Hex("origin_id", logging.ID(origin.NodeID)).
						Msg("could not store authenticated receipt in mempool")
				}
			}
		}
	}
}

// checkTrackers should be called periodically at intervals
// It retries the requests of all registered trackers a the node
func (e *Engine) checkTrackers() {
	e.checkTrackerLock.Lock()
	defer e.checkTrackerLock.Unlock()

	// chunk data packs
	//
	// iterates over all chunk data pack trackers
	for _, cdpt := range e.chunkDataPackTackers.All() {
		if cdpt.Counter > e.failureThreshold {
			// tracker met maximum retry chances
			// drops out of memory
			e.chunkDataPackTackers.Rem(cdpt.ChunkID)

			// TODO raise a missing chunk data pack challenge
			e.log.Info().
				Hex("chunk_id", logging.ID(cdpt.ChunkID)).
				Msg("chunk data pack tracker met maximum retries")

			continue
		}
		// retries requesting chunk data pack
		err := e.requestChunkDataPack(cdpt.ChunkID)
		if err != nil {
			e.log.Error().
				Err(err).
				Hex("chunk_id", logging.ID(cdpt.ChunkID)).
				Msg("could not request chunk data pack")
		}
	}

	// collection trackers
	//
	for _, ct := range e.collectionTrackers.All() {
		if ct.Counter > e.failureThreshold {
			// tracker met maximum retry chances
			// drops out of memory
			e.collectionTrackers.Rem(ct.CollectionID)
			// TODO raise a missing collection
			e.log.Info().
				Hex("collection_id", logging.ID(ct.CollectionID)).
				Msg("collection tracker met maximum retries")

			continue
		}

		// retries requesting collection
		err := e.requestCollection(ct.CollectionID)
		if err != nil {
			e.log.Error().
				Err(err).
				Hex("collection_id", logging.ID(ct.CollectionID)).
				Msg("could not request collection")
		}
	}
}

// updateChunkDataPackTracker performs the following
// If there is a tracker for this chunk ID, it pulls it out of mempool, increases its counter by one, and returns it
// Else it creates a new empty tracker with counter value of one and returns it
func (e *Engine) updateChunkDataPackTracker(chunkID flow.Identifier) (*trackers.ChunkDataPackTracker, error) {
	// pulls tracker out of mempool
	tracker, err := e.chunkDataPackTackers.ByChunkID(chunkID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve chunk data pack tracker from mempool: %w", err)
	}

	removed := e.chunkDataPackTackers.Rem(tracker.ChunkID)
	if !removed {
		return nil, fmt.Errorf("could not remove data pack tracker from mempool")
	}
	// increases tracker retry counter
	tracker.Counter += 1

	return tracker, nil
}

// updateCollectionTracker performs the following
// If there is a tracker for this collection ID, it pulls it out of mempool, increases its counter by one, and returns it
// Else it creates a new empty tracker for this collection with counter value of one and returns it
func (e *Engine) updateCollectionTracker(collectionID flow.Identifier) (*trackers.CollectionTracker, error) {

	// pulls tracker out of mempool
	tracker, err := e.collectionTrackers.ByCollectionID(collectionID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve chunk data pack tracker from mempool: %w", err)
	}

	removed := e.collectionTrackers.Rem(collectionID)
	if !removed {
		return nil, fmt.Errorf("could not remove collection tracker from mempool")
	}
	// increases tracker retry counter
	tracker.Counter += 1

	return tracker, nil

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
	if !e.blockStorage.Has(block.BlockID) {
		e.log.Error().
			Hex("block_id", logging.ID(block.BlockID)).
			Msg("expected block is not available in the storage")
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
