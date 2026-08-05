package chainsync

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/chainsync"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

const (
	// DefaultPollNodes is the default number of nodes we send a message to on
	// each poll interval.
	DefaultPollNodes uint = 3

	// DefaultBlockRequestNodes is the default number of nodes we request a
	// block resource from.
	DefaultBlockRequestNodes uint = 3

	// DefaultQueuedHeightMultiplicity limits the number of heights we queue
	// above the current finalized height.
	DefaultQueuedHeightMultiplicity uint = 4
)

type Config struct {
	RetryInterval time.Duration // the initial interval before we retry a request, uses exponential backoff
	Tolerance     uint          // determines how big of a difference in block heights we tolerated before actively syncing with range requests
	MaxAttempts   uint          // the maximum number of attempts we make for each requested block/height before discarding
	MaxSize       uint          // the maximum number of blocks we request in the same block request message
	MaxRequests   uint          // the maximum number of requests we send during each scanning period
}

func DefaultConfig() Config {
	return Config{
		RetryInterval: 4 * time.Second,
		Tolerance:     10,
		MaxAttempts:   5,
		MaxSize:       64,
		MaxRequests:   3,
	}
}

// Core contains core logic, configuration, and state for chain state
// synchronization. It is generic to chain type, so it works for both consensus
// and collection nodes.
//
// Core should be wrapped by a type-aware engine that manages the specifics of
// each chain. Example: https://github.com/onflow/flow-go/blob/master/engine/common/synchronization/engine.go
//
// Core is safe for concurrent use by multiple goroutines.
type Core struct {
	log                  zerolog.Logger
	Config               Config
	mu                   sync.Mutex
	heights              map[uint64]*chainsync.Status
	blockIDs             map[flow.Identifier]*chainsync.Status
	metrics              module.ChainSyncMetrics
	localFinalizedHeight uint64
}

func New(log zerolog.Logger, config Config, metrics module.ChainSyncMetrics, chainID flow.ChainID) (*Core, error) {
	core := &Core{
		log:                  log.With().Str("sync_core", chainID.String()).Logger(),
		Config:               config,
		heights:              make(map[uint64]*chainsync.Status),
		blockIDs:             make(map[flow.Identifier]*chainsync.Status),
		metrics:              metrics,
		localFinalizedHeight: 0,
	}
	return core, nil
}

// HandleBlock handles receiving a new block from another node. It returns
// true if the block should be processed by the compliance layer and false
// if it should be ignored.
func (c *Core) HandleBlock(header *flow.Header) bool {
	log := c.log
	if c.log.Debug().Enabled() {
		log = c.log.With().Str("block_id", header.ID().String()).Uint64("block_height", header.Height).Logger()
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	status := c.getRequestStatus(header.Height, header.ID())

	// if we never asked for this block, discard it
	if !status.WasQueued() {
		log.Debug().Msg("discarding not queued block")
		return false
	}
	// if we have already received this block, exit
	if status.WasReceived() {
		log.Debug().Msg("discarding not received block")
		return false
	}

	// this is a new block, remember that we've seen it
	status.Header = header
	status.Received = time.Now()

	// track it by ID and by height so we don't accidentally request it again
	c.blockIDs[header.ID()] = status
	c.heights[header.Height] = status

	log.Debug().Msg("handled block")
	return true
}

// HandleHeight handles receiving a new highest finalized height from another node.
// If the height difference between local and the reported height is outside tolerance, we do nothing.
// Otherwise, we queue each missing height.
func (c *Core) HandleHeight(final *flow.Header, height uint64) {
	log := c.log.With().Uint64("final_height", final.Height).Uint64("recv_height", height).Logger()
	log.Debug().Msg("received height")
	// don't bother queueing anything if we're within tolerance
	if c.WithinTolerance(final, height) {
		log.Debug().Msg("height within tolerance - discarding")
		return
	}

	// if we are sufficiently behind, we want to sync the missing blocks
	if height > final.Height {
		c.mu.Lock()
		defer c.mu.Unlock()

		// limit to request up to DefaultQueuedHeightMultiplicity*MaxRequests*MaxSize blocks from the peer.
		// without this limit, then if we are falling far behind,
		// we would queue up too many heights.
		heightLimit := final.Height + uint64(DefaultQueuedHeightMultiplicity*c.Config.MaxRequests*c.Config.MaxSize)
		if height > heightLimit {
			height = heightLimit
		}

		for h := final.Height + 1; h <= height; h++ {
			c.requeueHeight(h)
		}
		log.Debug().Msgf("requeued heights [%d-%d]", final.Height+1, height)
	}
}

func (c *Core) RequestBlock(blockID flow.Identifier, height uint64) {
	log := c.log.With().Str("block_id", blockID.String()).Uint64("height", height).Logger()
	// requesting a block by its ID storing the height to prune more efficiently
	c.mu.Lock()
	defer c.mu.Unlock()

	// if we already received this block, reset the status so we can re-queue
	status := c.blockIDs[blockID]
	if status.WasReceived() {
		log.Debug().Msgf("requested block was already received")
		delete(c.blockIDs, status.Header.ID())
		delete(c.heights, status.Header.Height)
	}

	c.queueByBlockID(blockID, height)
	log.Debug().Msgf("enqueued requested block")
}

func (c *Core) RequestHeight(height uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requeueHeight(height)
	c.log.Debug().Uint64("height", height).Msg("enqueued requested height")
}

// requeueHeight queues the given height, ignoring any previously received
// blocks at that height
func (c *Core) requeueHeight(height uint64) {
	// if we already received this block, reset the status so we can re-queue
	status := c.heights[height]
	if status.WasReceived() {
		delete(c.blockIDs, status.Header.ID())
		delete(c.heights, status.Header.Height)
	}

	c.queueByHeight(height)
}

// ScanPending scans all pending block statuses for blocks that should be
// requested. It apportions requestable items into range and batch requests
// according to configured maximums, giving precedence to range requests.
func (c *Core) ScanPending(final *flow.Header) ([]chainsync.Range, []chainsync.Batch) {
	c.mu.Lock()
	defer c.mu.Unlock()

	log := c.log.With().Uint64("final_height", final.Height).Logger()

	// prune if the current height is less than the new height
	c.prune(final)

	// get all items that are eligible for initial or re-requesting
	heights, blockIDs := c.getRequestableItems()
	c.log.Debug().Msgf("scan found %d requestable heights, %d requestable block IDs", len(heights), len(blockIDs))

	// convert to valid range and batch requests
	ranges := c.getRanges(heights)
	batches := c.getBatches(blockIDs)
	log.Debug().Str("ranges", fmt.Sprintf("%v", ranges)).Str("batches", fmt.Sprintf("%v", batches)).Msg("compiled range and batch requests")

	return c.selectRequests(ranges, batches)
}

// WithinTolerance returns whether or not the given height is within configured
// height tolerance, wrt the given local finalized header.
func (c *Core) WithinTolerance(final *flow.Header, height uint64) bool {

	lower := final.Height - uint64(c.Config.Tolerance)
	if lower > final.Height { // underflow check
		lower = 0
	}
	upper := final.Height + uint64(c.Config.Tolerance)

	return height >= lower && height <= upper
}

// queueByHeight queues a request for the finalized block at the given height,
// only if no equivalent request has been queued before.
func (c *Core) queueByHeight(height uint64) {
	// do not queue the block if the height is lower or the same as the local finalized height
	// the check != 0 is necessary or we will never queue blocks at height 0
	if height <= c.localFinalizedHeight && c.localFinalizedHeight != 0 {
		return
	}

	// only queue the request if have never queued it before
	if c.heights[height].WasQueued() {
		return
	}

	// queue the request
	c.heights[height] = chainsync.NewQueuedStatus(height)
}

// queueByBlockID queues a request for a block by block ID, only if no
// equivalent request has been queued before.
func (c *Core) queueByBlockID(blockID flow.Identifier, height uint64) {
	// do not queue the block if the height is lower or the same as the local finalized height
	// the check != 0 is necessary or we will never queue blocks at height 0
	if height <= c.localFinalizedHeight && c.localFinalizedHeight != 0 {
		return
	}

	// only queue the request if have never queued it before
	if c.blockIDs[blockID].WasQueued() {
		return
	}

	// queue the request
	c.blockIDs[blockID] = chainsync.NewQueuedStatus(height)
}

// getRequestStatus retrieves a request status for a block, regardless of
// whether it was queued by height or by block ID.
func (c *Core) getRequestStatus(height uint64, blockID flow.Identifier) *chainsync.Status {
	heightStatus := c.heights[height]
	idStatus := c.blockIDs[blockID]

	if idStatus.WasQueued() {
		return idStatus
	}
	// Only return the height status if there is no matching status for the ID
	if heightStatus.WasQueued() {
		return heightStatus
	}

	return nil
}

// prune removes any pending requests which we have received and which is below
// the finalized height, or which we received sufficiently long ago.
func (c *Core) prune(final *flow.Header) {
	if c.localFinalizedHeight >= final.Height {
		return
	}

	c.localFinalizedHeight = final.Height

	// track how many statuses we are pruning
	initialHeights := len(c.heights)
	initialBlockIDs := len(c.blockIDs)

	for height, status := range c.heights {
		if height <= final.Height {
			delete(c.heights, height)
			c.metrics.PrunedBlockByHeight(status)
		}
	}

	for blockID, status := range c.blockIDs {
		if status.BlockHeight <= final.Height {
			delete(c.blockIDs, blockID)
			c.metrics.PrunedBlockById(status)
		}
	}

	currentHeights := len(c.heights)
	currentBlockIDs := len(c.blockIDs)

	prunedHeights := initialHeights - currentHeights
	prunedBlockIDs := initialBlockIDs - currentBlockIDs

	c.metrics.PrunedBlocks(prunedHeights, prunedBlockIDs, currentHeights, currentBlockIDs)

	c.log.Debug().
		Uint64("final_height", final.Height).
		Msgf("pruned %d heights, %d block IDs", prunedHeights, prunedBlockIDs)
}

func (c *Core) Prune(final *flow.Header) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prune(final)
}

// getRequestableItems will find all block IDs and heights that are eligible
// to be requested.
func (c *Core) getRequestableItems() ([]uint64, []flow.Identifier) {

	// TODO: we will probably want to limit the maximum amount of in-flight
	// requests and maximum amount of blocks requested at the same time here;
	// for now, we just ignore that problem, but once we do, we should always
	// prioritize range requests over batch requests

	now := time.Now()

	// create a list of all height requests that should be sent
	var heights []uint64
	for height, status := range c.heights {

		// if the last request is young enough, skip
		retryAfter := status.Requested.Add(c.Config.RetryInterval << status.Attempts)
		if now.Before(retryAfter) {
			continue
		}

		// if we've already received this block, skip
		if status.WasReceived() {
			continue
		}

		// if we reached maximum number of attempts, delete
		if status.Attempts >= c.Config.MaxAttempts {
			delete(c.heights, height)
			continue
		}

		// otherwise, append to heights to be requested
		heights = append(heights, height)
	}

	// create list of all the block IDs blocks that are missing
	var blockIDs []flow.Identifier
	for blockID, status := range c.blockIDs {

		// if the last request is young enough, skip
		retryAfter := status.Requested.Add(c.Config.RetryInterval << status.Attempts)
		if now.Before(retryAfter) {
			continue
		}

		// if we've already received this block, skip
		if status.WasReceived() {
			continue
		}

		// if we reached the maximum number of attempts for a queue item, drop
		if status.Attempts >= c.Config.MaxAttempts {
			delete(c.blockIDs, blockID)
			continue
		}

		// otherwise, append to blockIDs to be requested
		blockIDs = append(blockIDs, blockID)
	}

	return heights, blockIDs
}

// RangeRequested updates status state for a range of block heights that has
// been successfully requested. Must be called when a range request is submitted.
func (c *Core) RangeRequested(ran chainsync.Range) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.RangeRequested(ran)

	for height := ran.From; height <= ran.To; height++ {
		status, exists := c.heights[height]
		if !exists {
			return
		}
		status.Requested = time.Now()
		status.Attempts++
	}
}

// BatchRequested updates status state for a batch of block IDs that has been
// successfully requested. Must be called when a batch request is submitted.
func (c *Core) BatchRequested(batch chainsync.Batch) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.BatchRequested(batch)

	for _, blockID := range batch.BlockIDs {
		status, exists := c.blockIDs[blockID]
		if !exists {
			return
		}
		status.Requested = time.Now()
		status.Attempts++
	}
}

// getRanges returns a set of ranges of heights that can be used as range
// requests.
func (c *Core) getRanges(heights []uint64) []chainsync.Range {

	// sort the heights so we can build contiguous ranges more easily
	sort.Slice(heights, func(i int, j int) bool {
		return heights[i] < heights[j]
	})

	// build contiguous height ranges with maximum batch size
	start := uint64(0)
	end := uint64(0)
	var ranges []chainsync.Range
	for index, height := range heights {

		// on the first iteration, we set the start pointer, so we don't need to
		// guard the for loop when heights is empty
		if index == 0 {
			start = height
		}

		// we always forward the end pointer to the new height
		end = height

		// if we have the end of the loop, we always create one final range
		if index >= len(heights)-1 {
			r := chainsync.Range{From: start, To: end}
			ranges = append(ranges, r)
			break
		}

		// at this point, we will have a next height as iteration will continue
		nextHeight := heights[index+1]

		// if we have reached the maximum size for a range, we create the range
		// and forward the start pointer to the next height
		rangeSize := end - start + 1
		if rangeSize >= uint64(c.Config.MaxSize) {
			r := chainsync.Range{From: start, To: end}
			ranges = append(ranges, r)
			start = nextHeight
			continue
		}

		// if end is more than one smaller than the next height, we have a gap
		// next, so we create a range and forward the start pointer
		if nextHeight > end+1 {
			r := chainsync.Range{From: start, To: end}
			ranges = append(ranges, r)
			start = nextHeight
			continue
		}
	}

	return ranges
}

// getBatches returns a set of batches that can be used in batch requests.
func (c *Core) getBatches(blockIDs []flow.Identifier) []chainsync.Batch {

	var batches []chainsync.Batch
	// split the block IDs into maximum sized requests
	for from := 0; from < len(blockIDs); from += int(c.Config.MaxSize) {

		// make sure last range is not out of bounds
		to := from + int(c.Config.MaxSize)
		if to > len(blockIDs) {
			to = len(blockIDs)
		}

		// create the block IDs slice
		requestIDs := blockIDs[from:to]
		batch := chainsync.Batch{
			BlockIDs: requestIDs,
		}
		batches = append(batches, batch)
	}

	return batches
}

// selectRequests selects which requests should be submitted, given a set of
// candidate range and batch requests. Range requests are given precedence and
// the total number of requests does not exceed the configured request maximum.
func (c *Core) selectRequests(ranges []chainsync.Range, batches []chainsync.Batch) ([]chainsync.Range, []chainsync.Batch) {
	max := int(c.Config.MaxRequests)

	if len(ranges) >= max {
		return ranges[:max], nil
	}
	if len(ranges)+len(batches) >= max {
		return ranges, batches[:max-len(ranges)]
	}
	return ranges, batches
}
