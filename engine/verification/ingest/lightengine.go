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
	trackers "github.com/dapperlabs/flow-go/model/verification/tracker"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// LightEngine implements a lighter version ingest engine of the verification node.
// It is responsible for receiving and handling new execution receipts. It requests
// all dependent resources for each execution receipt and relays a complete
// execution result to the verifier engine when all dependencies are ready.
type LightEngine struct {
	unit                  *engine.Unit
	log                   zerolog.Logger
	collectionsConduit    network.Conduit
	stateConduit          network.Conduit
	chunksConduit         network.Conduit
	me                    module.Local
	state                 protocol.State
	verifierEng           network.Engine                // for submitting ERs that are ready to be verified
	receipts              mempool.Receipts              // keeps execution receipts
	collections           mempool.Collections           // keeps collections
	chunkDataPacks        mempool.ChunkDataPacks        // keeps chunk data packs with authenticated origin IDs
	chunkDataPackTackers  mempool.ChunkDataPackTrackers // keeps track of chunk data pack requests that this engine made
	collectionTrackers    mempool.CollectionTrackers    // keeps track of collection requests that this engine made
	ingestedResultIDs     mempool.Identifiers           // keeps ids of ingested execution results
	ingestedChunkIDs      mempool.Identifiers           // keeps ids of ingested chunks
	assignedChunkIDs      mempool.Identifiers           // keeps ids of assigned chunk IDs pending for ingestion
	ingestedCollectionIDs mempool.Identifiers           // keeps ids collections of ingested chunks
	headerStorage         storage.Headers               // used to check block existence to improve performance
	blockStorage          storage.Blocks                // used to retrieve blocks
	assigner              module.ChunkAssigner          // used to determine chunks this node needs to verify
	requestInterval       uint                          // determines time in milliseconds for retrying tracked requests
	failureThreshold      uint                          // determines number of retries for tracked requests before raising a challenge
}

// New creates and returns a new instance of the ingest engine.
func NewLightEngine(
	log zerolog.Logger,
	net module.Network,
	state protocol.State,
	me module.Local,
	verifierEng network.Engine,
	receipts mempool.Receipts,
	collections mempool.Collections,
	chunkDataPacks mempool.ChunkDataPacks,
	collectionTrackers mempool.CollectionTrackers,
	chunkDataPackTrackers mempool.ChunkDataPackTrackers,
	ingestedChunkIDs mempool.Identifiers,
	ingestedResultIDs mempool.Identifiers,
	ingestedCollectionIDs mempool.Identifiers,
	assignedChunkIDs mempool.Identifiers,
	headerStorage storage.Headers,
	blockStorage storage.Blocks,
	assigner module.ChunkAssigner,
	requestIntervalMs uint,
	failureThreshold uint,
) (*LightEngine, error) {

	e := &LightEngine{
		unit:                  engine.NewUnit(),
		log:                   log,
		state:                 state,
		me:                    me,
		verifierEng:           verifierEng,
		collections:           collections,
		receipts:              receipts,
		chunkDataPacks:        chunkDataPacks,
		chunkDataPackTackers:  chunkDataPackTrackers,
		collectionTrackers:    collectionTrackers,
		ingestedChunkIDs:      ingestedChunkIDs,
		ingestedResultIDs:     ingestedResultIDs,
		ingestedCollectionIDs: ingestedCollectionIDs,
		assignedChunkIDs:      assignedChunkIDs,
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
func (l *LightEngine) Ready() <-chan struct{} {
	// checks pending chunks every `requestInterval` milliseconds
	l.unit.LaunchPeriodically(l.checkPendingChunks,
		time.Duration(l.requestInterval)*time.Millisecond,
		0)
	return l.unit.Ready()
}

// Done returns a channel that is closed when the verifier engine is done.
func (l *LightEngine) Done() <-chan struct{} {
	return l.unit.Done()
}

// SubmitLocal submits an event originating on the local node.
func (l *LightEngine) SubmitLocal(event interface{}) {
	l.Submit(l.me.NodeID(), event)
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (l *LightEngine) Submit(originID flow.Identifier, event interface{}) {
	l.unit.Launch(func() {
		err := l.Process(originID, event)
		if err != nil {
			l.log.Error().Err(err).Msg("could not process submitted event")
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (l *LightEngine) ProcessLocal(event interface{}) error {
	return l.Process(l.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (l *LightEngine) Process(originID flow.Identifier, event interface{}) error {
	return l.unit.Do(func() error {
		return l.process(originID, event)
	})
}

// process receives and submits an event to the verifier engine for processing.
// It returns an error so the verifier engine will not propagate an event unless
// it is successfully processed by the engine.
// The origin ID indicates the node which originally submitted the event to
// the peer-to-peer network.
func (l *LightEngine) process(originID flow.Identifier, event interface{}) error {
	switch resource := event.(type) {
	case *flow.ExecutionReceipt:
		return l.handleExecutionReceipt(originID, resource)
	case *flow.Collection:
		return l.handleCollection(originID, resource)
	case *messages.CollectionResponse:
		return l.handleCollection(originID, &resource.Collection)
	case *messages.ChunkDataPackResponse:
		return l.handleChunkDataPack(originID, &resource.Data)
	default:
		return ErrInvType
	}
}

// handleExecutionReceipt receives an execution receipt, and adds it to receipts mempool
func (l *LightEngine) handleExecutionReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {
	receiptID := receipt.ID()
	resultID := receipt.ExecutionResult.ID()

	l.log.Info().
		Hex("origin_id", logging.ID(originID)).
		Hex("receipt_id", logging.ID(receiptID)).
		Msg("execution receipt received at ingest engine")

	if l.ingestedResultIDs.Has(resultID) {
		l.log.Debug().
			Hex("origin_id", logging.ID(originID)).
			Hex("receipt_id", logging.ID(receiptID)).
			Msg("execution receipt with already ingested result discarded")
		// discards the receipt if its result has already been ingested
		return nil
	}

	// stores the execution receipt in the mempool
	ok := l.receipts.Add(receipt)
	l.log.Debug().
		Hex("origin_id", logging.ID(originID)).
		Hex("receipt_id", logging.ID(receiptID)).
		Bool("mempool_insertion", ok).
		Msg("execution receipt added to mempool")

	// checks if the execution result has empty chunk
	if receipt.ExecutionResult.Chunks.Len() == 0 {
		// TODO potential attack on availability
		l.log.Debug().
			Hex("receipt_id", logging.ID(receiptID)).
			Hex("result_id", logging.ID(resultID)).
			Msg("could not ingest execution result with zero chunks")
		return nil
	}

	mychunks, err := l.myAssignedChunks(&receipt.ExecutionResult)
	// extracts list of chunks assigned to this Verification node
	if err != nil {
		l.log.Error().
			Err(err).
			Hex("result_id", logging.Entity(receipt.ExecutionResult)).
			Msg("could not fetch assigned chunks")
		return fmt.Errorf("could not perfrom chunk assignment on receipt: %w", err)
	}

	l.log.Debug().
		Hex("receipt_id", logging.ID(receiptID)).
		Hex("result_id", logging.ID(resultID)).
		Int("total_chunks", receipt.ExecutionResult.Chunks.Len()).
		Int("assigned_chunks", len(mychunks)).
		Msg("chunk assignment is done")

	for _, chunk := range mychunks {
		err := l.handleChunk(chunk, receipt)
		if err != nil {
			l.log.Err(err).
				Hex("receipt_id", logging.ID(receiptID)).
				Hex("result_id", logging.ID(resultID)).
				Hex("chunk_id", logging.ID(chunk.ID())).
				Msg("could not handle chunk")
		}
	}

	return nil
}

// handleChunk receives an assigned chunk as part of an incoming receipt. It requests chunk data pack for it, and
// stores the chunk ID in the assigned chunk IDs mempool.
func (l *LightEngine) handleChunk(chunk *flow.Chunk, receipt *flow.ExecutionReceipt) error {
	chunkID := chunk.ID()

	// checks that the chunk has not been handled yet
	if l.assignedChunkIDs.Has(chunkID) {
		l.log.Debug().
			Hex("chunk_id", logging.ID(chunkID)).
			Msg("discards handling an already handled chunk")
		return nil
	}

	// checks that the chunk has not been ingested yet
	if l.ingestedChunkIDs.Has(chunkID) {
		l.log.Debug().
			Hex("chunk_id", logging.ID(chunkID)).
			Msg("discards handling an already ingested chunk")
		return nil
	}

	// requests the chunk data pack from network
	err := l.requestChunkDataPack(chunkID, receipt.ExecutionResult.BlockID)
	if err != nil {
		return fmt.Errorf("could not make a request of chunk data pack to the network")
	}

	// adds chunk to assigned mempool
	ok := l.assignedChunkIDs.Add(chunkID)
	if !ok {
		l.log.Debug().
			Hex("chunk_id", logging.ID(chunkID)).
			Msg("could not add chunk to memory pool")
		return nil
	}

	l.log.Debug().
		Hex("chunk_id", logging.ID(chunkID)).
		Msg("chunk handling is done")

	return nil
}

// handleChunkDataPack receives a chunk data pack, verifies its origin ID, and stores that in the mempool
func (l *LightEngine) handleChunkDataPack(originID flow.Identifier, chunkDataPack *flow.ChunkDataPack) error {
	l.log.Info().
		Hex("origin_id", logging.ID(originID)).
		Hex("chunk_data_pack_id", logging.Entity(chunkDataPack)).
		Msg("chunk data pack received")

	if l.ingestedChunkIDs.Has(chunkDataPack.ChunkID) {
		// belongs to an already ingested chunk
		// discards the chunk data pack
		l.log.Debug().
			Hex("origin_id", logging.ID(originID)).
			Hex("chunk_id", logging.ID(chunkDataPack.ChunkID)).
			Msg("discards the chunk data pack of an already ingested chunk")
		return nil
	}

	if l.chunkDataPacks.Has(chunkDataPack.ChunkID) {
		// discards an already existing chunk data pack
		l.log.Debug().
			Hex("origin_id", logging.ID(originID)).
			Hex("chunk_id", logging.ID(chunkDataPack.ChunkID)).
			Msg("discards the already exisiting chunk data pack")
		return nil
	}

	if !l.chunkDataPackTackers.Has(chunkDataPack.ChunkID) {
		// does not have a valid tracker
		// discards the chunk data pack
		l.log.Debug().
			Hex("origin_id", logging.ID(originID)).
			Hex("chunk_id", logging.ID(chunkDataPack.ChunkID)).
			Msg("discards the chunk data pack with no tracker")
		return nil
	}

	// store the chunk data pack in the store of the engine
	// this will fail if the receipt already exists in the store
	added := l.chunkDataPacks.Add(chunkDataPack)
	if !added {
		l.log.Debug().
			Hex("origin_id", logging.ID(originID)).
			Hex("chunk_id", logging.ID(chunkDataPack.ChunkID)).
			Msg("could not store chunk data pack")
		return nil
	}

	// removes chunk data pack tracker from  mempool
	l.chunkDataPackTackers.Rem(chunkDataPack.ChunkID)
	l.log.Debug().
		Hex("origin_id", logging.ID(originID)).
		Hex("chunk_id", logging.ID(chunkDataPack.ChunkID)).
		Msg("chunk data pack stored in mempool, tracker removed")

	return nil
}

// handleCollection handles receipt of a new collection, either via push or
// after a request. It adds the collection to the mempool.
func (l *LightEngine) handleCollection(originID flow.Identifier, coll *flow.Collection) error {
	collID := coll.ID()

	l.log.Info().
		Hex("origin_id", logging.ID(originID)).
		Hex("collection_id", logging.ID(collID)).
		Msg("collection received")

	// drops already ingested collection
	if l.ingestedCollectionIDs.Has(collID) {
		l.log.Info().
			Hex("origin_id", logging.ID(originID)).
			Hex("collection_id", logging.ID(collID)).
			Msg("drops ingested collection")
		return nil
	}

	// drops collection if it is residing on the mempool
	if l.collections.Has(collID) {
		l.log.Info().
			Hex("origin_id", logging.ID(originID)).
			Hex("collection_id", logging.ID(collID)).
			Msg("drops existing collection")
		return nil
	}

	// cleans tracker for the collection if any exists
	l.collectionTrackers.Rem(collID)

	// adds collection to mempool
	added := l.collections.Add(coll)
	if !added {
		return fmt.Errorf("could not add collection to mempool")
	}

	l.log.Debug().
		Hex("origin_id", logging.ID(originID)).
		Hex("collection_id", logging.ID(collID)).
		Msg("collection added to mempool, and tracker removed")
	return nil
}

// requestCollection submits a request for the given collection to collection nodes,
// or drops and logs the request if the tracker associated with the request goes beyond the
// failure threshold
func (l *LightEngine) requestCollection(collID, blockID flow.Identifier) error {
	// extracts list of collection nodes id
	//
	collNodes, err := l.state.Final().Identities(filter.HasRole(flow.RoleCollection))
	if err != nil {
		return fmt.Errorf("could not load collection node identities: %w", err)
	}

	req := &messages.CollectionRequest{
		ID:    collID,
		Nonce: rand.Uint64(),
	}

	// TODO we should only submit to cluster which owns the collection
	err = l.collectionsConduit.Submit(req, collNodes.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not submit request for collection (id=%s): %w", collID, err)
	}

	l.log.Debug().
		Hex("collection_id", logging.ID(collID)).
		Hex("block_id", logging.ID(blockID)).
		Msg("collection request submitted")
	return nil
}

// requestChunkDataPack submits a request for the given chunk ID to the execution nodes,
// or drops and logs the request if the tracker associated with the request goes beyond the
// failure threshold
func (l *LightEngine) requestChunkDataPack(chunkID, blockID flow.Identifier) error {
	// updates tracker for this request
	cdpt, err := l.updateChunkDataPackTracker(chunkID, blockID)
	if err != nil {
		return fmt.Errorf("could not update the chunk data pack tracker: %w", err)
	}
	// checks against maximum retries
	if cdpt.Counter > l.failureThreshold {
		// tracker met maximum retry chances
		// no longer retried
		// TODO raise a missing chunk data pack challenge
		// TODO drop tracker from memory once the challenge gets accepted, or trackers has nonce
		return fmt.Errorf("chunk data pack tracker met maximum retries, no longer retried, chunk ID: %x", chunkID)
	}

	// extracts list of execution nodes
	//
	execNodes, err := l.state.Final().Identities(filter.HasRole(flow.RoleExecution))
	if err != nil {
		return fmt.Errorf("could not load execution nodes identities: %w", err)
	}

	req := &messages.ChunkDataPackRequest{
		ChunkID: chunkID,
		Nonce:   rand.Uint64(),
	}

	// TODO we should only submit to execution node that generated execution receipt
	err = l.chunksConduit.Submit(req, execNodes.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not submit request for collection (id=%s): %w", chunkID, err)
	}

	l.log.Debug().
		Hex("chunk_id", logging.ID(chunkID)).
		Msg("chunk data pack request submitted")

	return nil
}

// getChunkDataPackForReceipt checks the chunk data pack associated with a chunk ID and
// execution receipt. If the chunk data pack is available locally, returns true
// as well as the chunk data pack itself.
func (l *LightEngine) getChunkDataPackForReceipt(receipt *flow.ExecutionReceipt, chunkID flow.Identifier) (*flow.ChunkDataPack, bool) {
	log := l.log.With().
		Hex("block_id", logging.ID(receipt.ExecutionResult.BlockID)).
		Hex("chunk_id", logging.ID(chunkID)).
		Hex("receipt_id", logging.Entity(receipt)).
		Logger()

	// checks mempool
	//
	if l.chunkDataPacks.Has(chunkID) {
		// chunk data pack exists and retrieved and returned
		chunkDataPack, exists := l.chunkDataPacks.ByChunkID(chunkID)
		if !exists {
			// couldn't get chunk state from mempool, the chunk cannot yet be verified
			log.Error().Msg("could not get chunk data pack from mempool")
			return nil, false
		}
		return chunkDataPack, true
	}

	// requests the chunk data pack from network
	err := l.requestChunkDataPack(chunkID, receipt.ExecutionResult.BlockID)
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
func (l *LightEngine) getCollectionForChunk(block *flow.Block, receipt *flow.ExecutionReceipt, chunk *flow.Chunk) (*flow.Collection, bool) {

	log := l.log.With().
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

	// checks authenticated collections mempool
	//
	//
	if l.collections.Has(collID) {
		coll, exists := l.collections.ByID(collID)
		if !exists {
			// couldn't get the collection from mempool
			l.log.Error().
				Hex("collection_id", logging.ID(collID)).
				Msg("could not get collection from authenticated pool")
			return nil, false
		}

		l.log.Debug().
			Hex("collection_id", logging.ID(collID)).
			Hex("chunk_id", logging.ID(chunk.ID())).
			Msg("collection is resolved from authenticated mempool")

		return coll, true
	}

	if l.collectionTrackers.Has(collID) {
		// collection has already been requested
		// drops its request
		l.log.Debug().
			Hex("collection_id", logging.ID(collID)).
			Msg("drops the request of already requested collection")
	}

	// adds a tracker for this collection
	l.collectionTrackers.Add(trackers.NewCollectionTracker(collID, block.ID()))

	// requests the collection from network
	err := l.requestCollection(collID, block.ID())
	if err != nil {
		l.log.Error().
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
func (l *LightEngine) checkPendingChunks() {
	l.log.Debug().
		Msg("check pending chunks background service started")

	for _, receipt := range l.receipts.All() {
		readyToClean := true
		block, err := l.blockStorage.ByID(receipt.ExecutionResult.BlockID)
		if err != nil {
			// we can't get collections without the block
			continue
		}

		for _, chunk := range receipt.ExecutionResult.Chunks {
			chunkID := chunk.ID()

			if !l.assignedChunkIDs.Has(chunkID) {
				// discards ingesting un-assigned chunk
				continue
			}

			if l.ingestedChunkIDs.Has(chunkID) {
				// discards ingesting an already ingested chunk
				continue
			}

			// receipt has at least one chunk for ingestion
			// not ready to clean
			readyToClean = false

			// retrieves collection corresponding to the chunk
			collection, collectionReady := l.getCollectionForChunk(block, receipt, chunk)
			if !collectionReady {
				// can not verify a chunk without its collection, moves to the next chunk
				continue
			}

			// retrieves chunk data pack for chunk
			chunkDatapack, chunkDataPackReady := l.getChunkDataPackForReceipt(receipt, chunk.ID())
			if !chunkDataPackReady {
				// can not verify a chunk without its chunk data, moves to the next chunk
				continue
			}

			err := l.ingestChunk(chunk, receipt, block, collection, chunkDatapack)
			if err != nil {
				l.log.Error().
					Err(err).
					Hex("result_id", logging.Entity(receipt.ExecutionResult)).
					Hex("chunk_id", logging.ID(chunk.ID())).
					Msg("could not ingest chunk")
			}
		}

		if readyToClean {
			// marks the receipt as ingested
			l.onReceiptIngested(receipt.ID(), receipt.ExecutionResult.ID())
		}
	}
}

// ingestChunk is called whenever a chunk is ready for ingestion, i.e., its receipt, block, collection, and chunkDataPack
// are all ready. It computes the end state of the chunk and ingests the chunk by submitting a verifiable chunk to the verify
// engine.
func (l *LightEngine) ingestChunk(chunk *flow.Chunk,
	receipt *flow.ExecutionReceipt,
	block *flow.Block,
	collection *flow.Collection,
	chunkDataPack *flow.ChunkDataPack) error {

	// creates chunk end state
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
		ChunkDataPack: chunkDataPack,
	}

	// verify the receipt
	err := l.verifierEng.ProcessLocal(vchunk)
	if err != nil {
		return fmt.Errorf("could not submit verifiable chunk to verify engine: %w", err)
	}

	// does resource cleanup
	l.onChunkIngested(vchunk)

	l.log.Debug().
		Hex("result_id", logging.Entity(receipt.ExecutionResult)).
		Hex("chunk_id", logging.ID(chunk.ID())).
		Msg("chunk successfully ingested")

	return nil
}

// onChunkIngested is called whenever a verifiable chunk is formed for a
// chunk and is sent to the verify engine successfully.
// It cleans up all resources associated with this chunk.
func (l *LightEngine) onChunkIngested(vc *verification.VerifiableChunk) {
	// marks this chunk as ingested
	ok := l.ingestedChunkIDs.Add(vc.ChunkDataPack.ChunkID)
	if !ok {
		l.log.Error().
			Hex("chunk_id", logging.ID(vc.ChunkDataPack.ChunkID)).
			Msg("could not add chunk to ingested chunks mempool")
	}

	// marks collection corresponding to this chunk as ingested
	ok = l.ingestedCollectionIDs.Add(vc.Collection.ID())
	if !ok {
		l.log.Error().
			Hex("chunk_id", logging.ID(vc.ChunkDataPack.ChunkID)).
			Hex("chunk_id", logging.ID(vc.Collection.ID())).
			Msg("could not add collection to ingested collections mempool")
	}

	// cleans up resources of the ingested chunk from mempools
	l.collections.Rem(vc.Collection.ID())
	l.chunkDataPacks.Rem(vc.ChunkDataPack.ID())
}

// onReceiptIngested is called whenever all chunks in the execution receipt are ingested.
// It removes the receipt from the memory pool, marks its result as ingested, and removes all
// receipts with the same result part from the mempool.
func (l *LightEngine) onReceiptIngested(receiptID flow.Identifier, resultID flow.Identifier) {

	// marks execution result as ingested
	added := l.ingestedResultIDs.Add(resultID)
	if !added {
		l.log.Debug().
			Hex("result_id", logging.ID(resultID)).
			Msg("could add ingested id result to mempool")
	}
	// removes receipt from mempool to avoid further iteration
	l.receipts.Rem(receiptID)

	// removes all authenticated receipts with the same result
	for _, receipt := range l.receipts.All() {
		// TODO check for nil dereferencing
		id := receipt.ExecutionResult.ID()
		if id == resultID {
			l.receipts.Rem(id)
		}
	}
}

// myAssignedChunks returns the list of chunks in the chunk list that this verifier node
// is assigned to.
func (l *LightEngine) myAssignedChunks(res *flow.ExecutionResult) (flow.ChunkList, error) {

	// extracts list of verifier nodes id
	//
	// TODO state extraction should be done based on block references
	// https://github.com/dapperlabs/flow-go/issues/2787
	verifierNodes, err := l.state.Final().
		Identities(filter.HasRole(flow.RoleVerification))
	if err != nil {
		return nil, fmt.Errorf("could not load verifier node IDs: %w", err)
	}

	rng, err := utils.NewChunkAssignmentRNG(res)
	if err != nil {
		return nil, fmt.Errorf("could not generate random generator: %w", err)
	}
	// TODO pull up caching of chunk assignments to here
	a, err := l.assigner.Assign(verifierNodes, res.Chunks, rng)
	if err != nil {
		return nil, fmt.Errorf("could not create chunk assignment %w", err)
	}

	// indices of chunks assigned to this node
	chunkIndices := a.ByNodeID(l.me.NodeID())

	// mine keeps the list of chunks assigned to this node
	mine := make(flow.ChunkList, 0, len(chunkIndices))
	for _, index := range chunkIndices {
		chunk, ok := res.Chunks.ByIndex(index)
		if !ok {
			return nil, fmt.Errorf("chunk out of range requested: %v", index)
		}
		mine = append(mine, chunk)
	}

	return mine, nil
}

// updateChunkDataPackTracker performs the following
// If there is a tracker for this chunk ID, it increases its counter by one in place
// Else it creates a new empty tracker with counter value of one and stores it in the trackers mempool
func (l *LightEngine) updateChunkDataPackTracker(chunkID flow.Identifier, blockID flow.Identifier) (*trackers.ChunkDataPackTracker, error) {
	var cdpt *trackers.ChunkDataPackTracker

	if l.chunkDataPackTackers.Has(chunkID) {
		// there is a tracker for this chunk
		// increases its counter
		t, err := l.chunkDataPackTackers.Inc(chunkID)
		if err != nil {
			return nil, fmt.Errorf("could not update chunk data pack tracker: %w", err)
		}
		cdpt = t
	} else {
		// creates a new chunk data pack tracker and stores in in memory
		cdpt = trackers.NewChunkDataPackTracker(chunkID, blockID)
		ok := l.chunkDataPackTackers.Add(cdpt)
		if !ok {
			return nil, fmt.Errorf("could not store tracker of chunk data pack request in mempool")
		}
	}

	return cdpt, nil
}

// To implement FinalizationConsumer
func (l *LightEngine) OnBlockIncorporated(*model.Block) {

}

// OnFinalizedBlock is part of implementing FinalizationConsumer interface
//
// OnFinalizedBlock notifications are produced by the Finalization Logic whenever
// a block has been finalized. They are emitted in the order the blocks are finalized.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
func (l *LightEngine) OnFinalizedBlock(block *model.Block) {

	// block should be in the storage
	_, err := l.headerStorage.ByBlockID(block.BlockID)
	if errors.Is(err, storage.ErrNotFound) {
		l.log.Error().
			Hex("block_id", logging.ID(block.BlockID)).
			Msg("block is not available in storage")
		return
	}
	if err != nil {
		l.log.Error().
			Hex("block_id", logging.ID(block.BlockID)).
			Msg("could not check block availability in storage")
		return
	}
}

// To implement FinalizationConsumer
func (l *LightEngine) OnDoubleProposeDetected(*model.Block, *model.Block) {}
