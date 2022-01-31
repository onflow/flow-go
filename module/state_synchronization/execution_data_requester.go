package state_synchronization

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"
	ipfsds "github.com/ipfs/go-datastore"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/storage"
)

const (
	// Frequency at which to check for missing blocks
	missingBlockCheckInterval = 15 * time.Minute

	// Timeout for fetching ExecutionData from the db/network
	edFetchTimeout = time.Minute
)

// ExecutionDataRequester is a Components that receives block finalization events, and requests
// ExecutionData for all blocks sealed in newly finalized blocks. It also checks its local datastore
// to ensure that there is ExecutionData for all sealed blocks since the configured root block.
type ExecutionDataRequester struct {
	component.Component
	cm  *component.ComponentManager
	ds  datastore.Batching
	bs  network.BlobService
	eds ExecutionDataService
	log zerolog.Logger

	// Local db objects
	blocks  storage.Blocks
	results storage.ExecutionResults

	// The first block for which to request ExecutionData
	rootBlock *flow.Block

	// The highest height block for which ExecutionData exists in the blobstore
	last lastProcessedHeight

	// List of block IDs that are missing from the blobstore
	missingBlocks missingBlocksList

	// List of callbacks to call when ExecutionData is successfully fetched for a block
	consumers []ExecutionDataReceivedCallback

	// Interval at which to retry downloads for missing blocks
	missingCheckInterval time.Duration

	// Channel used to pass finalized block IDs to the finalization processor worker routine
	finalizedBlocks chan flow.Identifier

	// Channel used to signal that the config has been loaded from the db
	configLoaded chan struct{}

	consumerMu sync.RWMutex
}

type ExecutionDataReceivedCallback func(*ExecutionData)

// NewExecutionDataRequester creates a new execution data requester engine
func NewExecutionDataRequester(
	log zerolog.Logger,
	edsMetrics module.ExecutionDataServiceMetrics,
	finalizationDistributor *pubsub.FinalizationDistributor,
	datastore datastore.Batching,
	blobservice network.BlobService,
	eds ExecutionDataService,
	rootBlock *flow.Block,
	blocks storage.Blocks,
	results storage.ExecutionResults,
) (*ExecutionDataRequester, error) {

	e := &ExecutionDataRequester{
		log:                  log.With().Str("engine", "executiondata_requester").Logger(),
		ds:                   datastore,
		bs:                   blobservice,
		eds:                  eds,
		rootBlock:            rootBlock,
		blocks:               blocks,
		results:              results,
		missingCheckInterval: missingBlockCheckInterval,
		finalizedBlocks:      make(chan flow.Identifier),
		configLoaded:         make(chan struct{}),
	}

	finalizationDistributor.AddOnBlockFinalizedConsumer(e.onBlockFinalized)

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
	e.last = lastProcessedHeight{ds: e.ds}
	if err := e.last.load(ctx); err != nil {
		e.log.Debug().Err(err).Msg("last processed height not found in db")
		e.last.set(ctx, e.rootBlock.Header.Height)
	}
	e.log.Debug().Msgf("starting with last processed height %d", e.last.height())

	e.missingBlocks = missingBlocksList{ds: e.ds}
	if err := e.missingBlocks.load(ctx); err != nil {
		e.log.Debug().Err(err).Msg("missing blocks list not found in db. using empty list")
	}

	close(e.configLoaded)

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
		logger.Error().Err(err).Msg("finalized block is missing from protocol state db")
		return
	}
	if err != nil {
		// there was an issue with the db, filesystem or data stored in the db
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
			sealLogger.Error().Err(err).Msg("sealed block is missing from protocol state db")
			e.missingBlocks.add(seal.BlockID)
			continue
		}

		// other errors means there was an issue connecting to the db or with the stored data
		if err != nil {
			sealLogger.Error().Err(err).Msg("failed to lookup sealed block in protocol state db")
			e.missingBlocks.add(seal.BlockID)
			continue
		}

		sealedHeights = append(sealedHeights, sealedBlock.Header.Height)

		executionData, err := e.byBlockID(ctx, seal.BlockID)
		if err != nil {
			sealLogger.Error().Err(err).Msg("failed to get execution data for block")
			e.missingBlocks.add(seal.BlockID)
			continue
		}

		sealLogger.Debug().Msgf("fetched execution data for height %d", sealedBlock.Header.Height)
		go e.onExecutionDataFetched(executionData)
	}

	e.missingBlocks.save(ctx)

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

	e.last.set(ctx, sealedHeights[len(sealedHeights)-1])
}

// addMissingBefore searches for blocks missed by the finalized block processor, and enqueues them
// to be fetched
func (e *ExecutionDataRequester) addMissingBefore(current uint64) error {
	next := e.last.height() + 1
	if next >= current {
		// current is either the next block in the sequence, or one we've already seen
		return nil
	}

	for next < current {
		e.log.Debug().Msgf("next: %d, sealed: %d", next, current)
		nextBlock, err := e.blocks.ByHeight(next)
		if err != nil {
			// TODO: handle this condition better. If this error is encountered, the blocks will not
			//       be flagged as missing and reattempted until the next boot when the full check is
			//       performed.
			return fmt.Errorf("failed to get block for height %d: %w", next, err)
		}
		e.missingBlocks.add(nextBlock.ID())
		next++
	}

	return nil
}

// byBlockID fetches the ExecutionData for a block by its ID
func (e *ExecutionDataRequester) byBlockID(signalerCtx irrecoverable.SignalerContext, blockID flow.Identifier) (*ExecutionData, error) {
	// Get the ExecutionResult, which contains the root CID for the execution data
	result, err := e.results.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup execution result for block: %w", err)
	}

	ctx, cancel := context.WithTimeout(signalerCtx, edFetchTimeout)
	defer cancel()

	// Fetch the ExecutionData for blockID from the blobstore. If it doesn't exist locally, it will
	// be fetched from the network.
	executionData, err := e.eds.Get(ctx, result.ExecutionDataID)

	// Data is either corrupt or malformed. delete and redownload
	var malformedDataError *MalformedDataError
	var blobSizeLimitExceededError *BlobSizeLimitExceededError
	if errors.Is(err, malformedDataError) || errors.Is(err, blobSizeLimitExceededError) {
		cid := flow.FlowIDToCid(result.ExecutionDataID)
		e.bs.DeleteBlob(signalerCtx, cid)
	}

	// Otherwise, blob was not available on the network or the fetch timed out.
	if err != nil {
		return nil, fmt.Errorf("failed to get execution data for block: %w", err)
	}

	return executionData, nil
}

// AddOnExecutionDataFetchedConsumer adds a callback to be called when a new ExecutionData is received
// Callback Implementations must:
//   * be concurrency safe
//   * be non-blocking
//   * handle repetition of the same events (with some processing overhead).
func (e *ExecutionDataRequester) AddOnExecutionDataFetchedConsumer(fn ExecutionDataReceivedCallback) {
	e.consumerMu.Lock()
	defer e.consumerMu.Unlock()
	e.consumers = append(e.consumers, fn)
}

// onExecutionDataFetched is called when a new ExecutionData is received, and calls all registered
// consumers
func (e *ExecutionDataRequester) onExecutionDataFetched(executionData *ExecutionData) {
	e.consumerMu.RLock()
	defer e.consumerMu.RUnlock()
	for _, fn := range e.consumers {
		fn(executionData)
	}
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
	<-e.configLoaded
	<-e.eds.Ready()
	ready()

	e.checkDatastore(ctx, e.last.height())
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
// missing.
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
			e.missingBlocks.add(block.ID())
			continue
		}

		if e.missingBlocks.delete(block.ID()) {
			e.log.Debug().Msgf("fetched execution data for height %d", block.Header.Height)
			go e.onExecutionDataFetched(executionData)
		}
	}
	e.missingBlocks.save(ctx)
}

// checkMissing attempts to download ExecutionData for any blocks in missingBlocks
// Any blocks that are successfully downloaded will be removed from missingBlocks and notifications
// will be sent to all consumers
func (e *ExecutionDataRequester) checkMissing(ctx irrecoverable.SignalerContext) {
	// TODO: timeout after some number of failures
	e.missingBlocks.filter(ctx, func(blockID flow.Identifier) bool {
		executionData, err := e.byBlockID(ctx, blockID)
		if err != nil {
			e.log.Error().Err(err).Msg("failed during check for missing block")
			return true
		}

		e.log.Debug().Msgf("fetched missing execution data for block %v", blockID)
		go e.onExecutionDataFetched(executionData)
		return false
	})
	e.missingBlocks.save(ctx)
}

type lastProcessedHeight struct {
	last uint64

	ds datastore.Batching
	mu sync.Mutex
}

// load loads the last processed height from the db.
func (h *lastProcessedHeight) load(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.last != 0 {
		return nil
	}

	data, err := h.ds.Get(ctx, ipfsds.NewKey("lastProcessedHeight"))
	if err != nil {
		return err
	}

	height, err := strconv.ParseUint(string(data), 10, 64)
	if err != nil {
		return err
	}

	h.last = height
	return nil
}

// set caches the last processed height in the db
func (h *lastProcessedHeight) set(ctx context.Context, height uint64) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if height <= h.last {
		return nil
	}
	h.last = height

	value := fmt.Sprintf("%d", height)
	return h.ds.Put(ctx, ipfsds.NewKey("lastProcessedHeight"), []byte(value))
}

// get returns the value of height
func (h *lastProcessedHeight) height() uint64 {
	return h.last
}

type missingBlocksList struct {
	missingBlocks map[flow.Identifier]struct{}

	ds datastore.Batching
	mu sync.Mutex
}

// load loads the list of missing block IDs from the db.
// If no list is found or an error is encountered, it returns an error and makes no changes
func (l *missingBlocksList) load(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := l.ds.Get(ctx, ipfsds.NewKey("missingBlockIDs"))
	if err != nil {
		return err
	}

	missingHex := strings.Split(string(data), ",")
	missing := make(map[flow.Identifier]struct{})
	for _, idHex := range missingHex {
		id, err := flow.HexStringToIdentifier(idHex)
		if err != nil {
			return err
		}

		missing[id] = struct{}{}
	}

	l.missingBlocks = missing
	return nil
}

// set caches the list of missing block IDs in the db
func (l *missingBlocksList) save(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.ds.Put(ctx, ipfsds.NewKey("missingBlockIDs"), []byte(l.String()))
}

// add adds an ID to the list of missing block IDs
func (l *missingBlocksList) add(id flow.Identifier) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.missingBlocks[id] = struct{}{}
}

// delete attempts to remove the provided ID from the list of missing blocks and returns true if
// the ID was removed
func (l *missingBlocksList) delete(id flow.Identifier) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, has := l.missingBlocks[id]; !has {
		return false
	}

	delete(l.missingBlocks, id)
	return true

}

func (l *missingBlocksList) filter(ctx context.Context, fn func(flow.Identifier) bool) {
	l.mu.Lock()
	for blockID := range l.missingBlocks {
		if ctx.Err() != nil {
			break
		}

		l.mu.Unlock()
		keep := fn(blockID)
		l.mu.Lock()

		if !keep {
			delete(l.missingBlocks, blockID)
		}
	}
	l.mu.Unlock()
}

func (l *missingBlocksList) String() string {
	data := ""
	for blockID := range l.missingBlocks {
		if len(data) == 0 {
			data = blockID.String()
			continue
		}
		data = fmt.Sprintf("%s,%s", data, blockID)
	}

	return data
}
