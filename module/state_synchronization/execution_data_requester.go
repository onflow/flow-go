package state_synchronization

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	ipfsds "github.com/ipfs/go-datastore"
	badger "github.com/ipfs/go-ds-badger2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/compressor"
	"github.com/onflow/flow-go/storage"
)

const (
	defaultMissingBlockCheckInterval = 15 * time.Minute
)

// ExecutionDataRequester is a Components that receives block finalization events, and requests
// ExecutionData for all blocks sealed in newly finalized blocks. It also checks its local datastore
// to ensure that there is ExecutionData for all sealed blocks since the configured root block.
type ExecutionDataRequester struct {
	component.Component
	cm  *component.ComponentManager
	ds  *badger.Datastore
	bs  network.BlobService
	eds ExecutionDataService
	log zerolog.Logger

	// Local db objects
	blocks  storage.Blocks
	results storage.ExecutionResults

	// The first block for which to request ExecutionData
	rootBlock *flow.Block

	// The highest height block for which ExecutionData exists in the blobstore
	last uint64

	// List of block IDs that are missing from the blobstore
	missingBlocks map[flow.Identifier]struct{}

	// List of callbacks to call when ExecutionData is successfully fetched for a block
	consumers []ExecutionDataReceivedCallback

	// Interval at which to retry downloads for missing blocks
	missingCheckInterval time.Duration

	// Mutex to protect access to state
	missingMu sync.Mutex
	stateMu   sync.Mutex

	// Channel used to pass finalized block IDs to the finalization processor worker routine
	finalizedBlocks chan flow.Identifier
}

type ExecutionDataReceivedCallback func(*ExecutionData)

// NewExecutionDataRequester creates a new execution data requester engine
func NewExecutionDataRequester(
	finalizationDistributor *pubsub.FinalizationDistributor,
	datastore *badger.Datastore,
	blobservice network.BlobService,
	rootBlock *flow.Block,
	blocks storage.Blocks,
	results storage.ExecutionResults,
	edsMetrics module.ExecutionDataServiceMetrics,
	log zerolog.Logger,
) (*ExecutionDataRequester, error) {

	e := &ExecutionDataRequester{
		log:                  log.With().Str("engine", "executiondata_requester").Logger(),
		ds:                   datastore,
		bs:                   blobservice,
		rootBlock:            rootBlock,
		blocks:               blocks,
		results:              results,
		missingBlocks:        make(map[flow.Identifier]struct{}),
		missingCheckInterval: defaultMissingBlockCheckInterval,
		finalizedBlocks:      make(chan flow.Identifier),
	}

	finalizationDistributor.AddOnBlockFinalizedConsumer(e.onBlockFinalized)

	// setup execution data service
	codec := new(cbor.Codec)
	compressor := compressor.NewLz4Compressor()

	e.eds = NewExecutionDataService(codec, compressor, blobservice, edsMetrics, e.log)

	e.cm = component.NewComponentManagerBuilder().
		AddWorker(e.finalizedBlockProcessor).
		AddWorker(e.checkAndBackfill).
		Build()
	e.Component = e.cm

	return e, nil
}

// onBlockFinalized is called when a new block is finalized, and passes the block ID to the
// main worker for processing
func (e *ExecutionDataRequester) onBlockFinalized(blockID flow.Identifier) {
	if util.CheckClosed(e.cm.ShutdownSignal()) {
		return
	}
	e.finalizedBlocks <- blockID
}

// finalizedBlockProcessor runs the main process that processes finalized block notifications and
// requests ExecutionData for each block seal
func (e *ExecutionDataRequester) finalizedBlockProcessor(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	_, _ = e.loadLastProcessedHeight(ctx)
	<-e.eds.Ready()
	ready()

	e.finalizationProcessingLoop(ctx)
}

// finalizationProcessingLoop watches for new finalized blocks then kicks off a goroutine to
// process it
// This uses a channel to receive the block instead of processing the finalized block directly in the
// callback, because the irrecoverable context passed into start is only available via the worker
func (e *ExecutionDataRequester) finalizationProcessingLoop(ctx irrecoverable.SignalerContext) {
	for {
		select {
		case <-ctx.Done():
			return
		case blockID := <-e.finalizedBlocks:
			go e.processFinalizedBlock(ctx, blockID)
		}
	}
}

// processFinalizedBlock downloads the ExecutionData for each of the blocks sealed in the finalized block
func (e *ExecutionDataRequester) processFinalizedBlock(ctx irrecoverable.SignalerContext, blockID flow.Identifier) {
	logger := e.log.With().Str("finalized_block_id", blockID.String()).Logger()
	logger.Debug().Msg("received finalized block")

	// get the block
	block, err := e.blocks.ByID(blockID)
	if errors.Is(err, storage.ErrNotFound) {
		logger.Error().Err(err).Msg("block is missing from protocol state db")
		return
	}
	if err != nil {
		// This means there was an issue with the db, filesystem or data stored in the db
		logger.Error().Err(err).Msg("failed to lookup block in protocol state db")
		return
	}

	// go through the seals and request the ExecutionData for each sealed block
	logger.Debug().Msgf("checking %d seals in block", len(block.Payload.Seals))

	sealedHeights := []uint64{}
	for _, seal := range block.Payload.Seals {
		sealLogger := logger.With().Str("sealed_block_id", seal.BlockID.String()).Logger()
		sealLogger.Debug().Msg("checking sealed block")

		sealedBlock, err := e.blocks.ByID(seal.BlockID)

		// missing blocks are enqueued for retry
		if errors.Is(err, storage.ErrNotFound) {
			sealLogger.Error().Err(err).Msg("block is missing from protocol state db")
			go e.addMissing(seal.BlockID)
			continue
		}

		// other errors means there was an issue connecting to the db or with the stored data
		if err != nil {
			sealLogger.Error().Err(err).Msg("failed to lookup sealed block in protocol state db")
			go e.addMissing(seal.BlockID)
			continue
		}

		sealedHeights = append(sealedHeights, sealedBlock.Header.Height)

		executionData, err := e.byBlockID(ctx, seal.BlockID)
		if err != nil {
			sealLogger.Error().Err(err).Msg("failed to get execution data for block")
			go e.addMissing(seal.BlockID)
			continue
		}

		sealLogger.Debug().Msgf("fetched execution data for height %d", sealedBlock.Header.Height)
		go e.onExecutionDataFetched(executionData)
	}

	// Finally, check if there are any heights we've missed. The set of seals in a block must be
	// for consecutive heights, so sort the heights and then check for any that are missing before
	// the first seal
	if len(sealedHeights) == 0 {
		return
	}

	sort.Slice(sealedHeights, func(i, j int) bool {
		return sealedHeights[i] < sealedHeights[j]
	})

	err = e.addMissingBefore(sealedHeights[0])
	if err != nil {
		logger.Error().Err(err).Msg("failed to check for missing seals")
	}

	e.updateLastProcessedHeight(ctx, sealedHeights[len(sealedHeights)-1])
}

// addMissingBefore searches for blocks missed by the finalization event processor, and enqueues them
// to be fetched
func (e *ExecutionDataRequester) addMissingBefore(current uint64) error {
	next := e.last + 1
	if next >= current {
		return nil
	}

	// one or more blocks were skipped, iterate over the gap and add each block to the missing list
	for next < current {
		e.log.Debug().Msgf("next: %d, sealed: %d", next, current)
		nextBlock, err := e.blocks.ByHeight(next)
		if err != nil {
			// TODO: handle this condition better. If this error is encountered, the blocks will not
			//       be flagged as missing and reattempted until the next boot when the full check is
			//       performed.
			return fmt.Errorf("failed to get block for height %d: %w", next, err)
		}
		go e.addMissing(nextBlock.ID())
		next++
	}

	return nil
}

// addMissing adds a block height to the list of missing blocks
// This should be run in a separate goroutine since it will block on the mutex until checkMissing
// has finished
func (e *ExecutionDataRequester) addMissing(blockID flow.Identifier) {
	e.missingMu.Lock()
	defer e.missingMu.Unlock()
	e.missingBlocks[blockID] = struct{}{}
}

// byBlockID fetches the ExecutionData for a block by its ID
func (e *ExecutionDataRequester) byBlockID(ctx irrecoverable.SignalerContext, blockID flow.Identifier) (*ExecutionData, error) {
	// Get the ExecutionResult, which contains the root CID for the execution data
	result, err := e.results.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup execution result for block %v: %w", blockID, err)
	}

	// Fetch the ExecutionData for blockID from the blobstore. If it doesn't exist locally, it will
	// be fetched from the network.
	executionData, err := e.eds.Get(ctx, result.ExecutionDataID)

	// Data is either corrupt or malformed. delete and redownload
	var malformedDataError *MalformedDataError
	var blobSizeLimitExceededError *BlobSizeLimitExceededError
	if errors.Is(err, malformedDataError) || errors.Is(err, blobSizeLimitExceededError) {
		cid := flow.FlowIDToCid(result.ExecutionDataID)
		e.bs.DeleteBlob(ctx, cid)
	}

	// Otherwise, blob was not available on the network. Try again later

	if err != nil {
		return nil, fmt.Errorf("failed to get execution data for block %v: %w", blockID, err)
	}

	return executionData, nil
}

// AddOnExecutionDataFetchedConsumer adds a callback to be called when a new ExecutionData is received
func (e *ExecutionDataRequester) AddOnExecutionDataFetchedConsumer(fn ExecutionDataReceivedCallback) {
	e.consumers = append(e.consumers, fn)
}

// onExecutionDataFetched is called when a new ExecutionData is received, and calls all registered
// consumers
func (e *ExecutionDataRequester) onExecutionDataFetched(executionData *ExecutionData) {
	for _, fn := range e.consumers {
		fn(executionData)
	}
}

// loadLastProcessedHeight loads the last processed height from the db.
// If no height is found or an error is encountered, it uses the root block's height and returns an
// error
func (e *ExecutionDataRequester) loadLastProcessedHeight(ctx context.Context) (uint64, error) {
	e.stateMu.Lock()
	defer e.stateMu.Unlock()
	if e.last != 0 {
		return e.last, nil
	}

	genesis := e.rootBlock.Header.Height

	data, err := e.ds.Get(ctx, ipfsds.NewKey("lastProcessedHeight"))
	if err != nil {
		return genesis, err
	}

	last, err := strconv.ParseUint(string(data), 10, 64)
	if err != nil {
		return genesis, err
	}

	e.log.Debug().Msgf("loaded last processed height as %d", last)

	e.last = last
	return e.last, nil
}

// updateLastProcessedHeight caches the last processed height in the db
func (e *ExecutionDataRequester) updateLastProcessedHeight(ctx context.Context, height uint64) error {
	e.stateMu.Lock()
	defer e.stateMu.Unlock()

	if height <= e.last {
		return nil
	}
	e.last = height

	e.log.Debug().Msgf("updating last processed height to %d", height)

	data := fmt.Sprintf("%d", height)
	return e.ds.Put(ctx, ipfsds.NewKey("lastProcessedHeight"), []byte(data))
}

// The following methods make up the background process that ensures the blobstore contains data
// for all sealed blocks. These run as a goroutine and do not block the requester from becoming
// Ready.
// The following outlines the process:
// 1. Load the last block height processed from the db. If it doesn't exist, assume that there's
//    no data in the blobstore and start at the genesis block
// 2. Otherwise, scan the blobstore from the genesis height to last. If any are missing, the
//    blobservice will automatically fetch them from the network. If the check fails for any heights,
//    they are added to the missing blocks list.
// 3. Finally, start polling the network for missing blocks. Every missingCheckInterval, request each
//    missing block from the network. If any are found, remove them from the missing list and send
//    onExecutionDataFetched notifications.

// checkAndBackfill runs a single background goroutine that checks for and requests ExecutionData for
// blocks that are missing from the blobstore.
func (e *ExecutionDataRequester) checkAndBackfill(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	last, err := e.loadLastProcessedHeight(ctx)
	<-e.eds.Ready()
	ready()

	// if there's an error, assume there's no data in the store and we're starting from the root
	// block height. only check the datastore if we have already processed blocks
	if err == nil {
		e.checkDatastore(ctx, last)
	}

	e.missingBlockProcessingLoop(ctx)
}

// missingBlockProcessingLoop checks for missing blocks every missingCheckInterval
func (e *ExecutionDataRequester) missingBlockProcessingLoop(ctx irrecoverable.SignalerContext) {
	check := time.NewTicker(e.missingCheckInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-check.C:
			e.checkMissing(ctx)
		}
	}
}

// checkDatastore fetches blobs for each block since the rootBlock, and identifies any that are
// missing ExecutionData.
// Since this check requests the data from disk, it guarantees that a local copy exists, otherwise
// it will be marked missing and refetched from the network.
func (e *ExecutionDataRequester) checkDatastore(ctx irrecoverable.SignalerContext, lastHeight uint64) {

	genesis := e.rootBlock.Header.Height

	if lastHeight < genesis {
		// this is probably a configuration issue
		e.log.Fatal().Msg("latest block height is lower than first block")
	}

	for height := genesis; height <= lastHeight; height++ {
		if ctx.Err() != nil {
			return
		}

		block, err := e.blocks.ByHeight(height)
		if err != nil {
			e.log.Error().Err(err).Msgf("failed to get block for height %d", height)
			continue
		}

		executionData, err := e.byBlockID(ctx, block.ID())
		if err != nil {
			e.log.Error().Err(err).Msg("error during datastore check")
			e.addMissing(block.ID())
			continue
		}

		// TODO: should we track downloads of missing blocks between starts?
		if false {
			e.log.Debug().Msgf("fetched execution data for height %d", block.Header.Height)
			go e.onExecutionDataFetched(executionData)
		}
	}
}

// checkMissing attempts to download ExecutionData for any blocks in missingBlocks
// Any blocks that are successfully downloaded will be removed from missingBlocks and notifications
// will be sent to all consumers
func (e *ExecutionDataRequester) checkMissing(ctx irrecoverable.SignalerContext) {
	// TODO: should we timeout after some number of failures?
	for blockID := range e.missingBlocks {
		if ctx.Err() != nil {
			return
		}

		executionData, err := e.byBlockID(ctx, blockID)
		if err != nil {
			e.log.Error().Err(err).Msg("failed during check for missing block")
			continue
		}

		e.missingMu.Lock()
		delete(e.missingBlocks, blockID)
		e.missingMu.Unlock()

		e.log.Debug().Msgf("fetched missing execution data for block %v", blockID)
		go e.onExecutionDataFetched(executionData)
	}
}
