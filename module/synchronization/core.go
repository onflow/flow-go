package synchronization

import (
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
)

const (
	// DefaultPollNodes is the default number of nodes we send a message to on
	// each poll interval.
	DefaultPollNodes uint = 3

	// DefaultBlockRequestNodes is the default number of nodes we request a
	// block resource from.
	DefaultBlockRequestNodes uint = 3
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
	log      zerolog.Logger
	Config   Config
	mu       sync.Mutex
	heights  map[uint64]*Status
	blockIDs map[flow.Identifier]*Status
}

func New(log zerolog.Logger, config Config) (*Core, error) {
	core := &Core{
		log:      log.With().Str("module", "synchronization").Logger(),
		Config:   config,
		heights:  make(map[uint64]*Status),
		blockIDs: make(map[flow.Identifier]*Status),
	}
	return core, nil
}

// HandleBlock handles receiving a new block from another node. It returns
// true if the block should be processed by the compliance layer and false
// if it should be ignored.
func (c *Core) HandleBlock(header *flow.Header) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	status := c.getRequestStatus(header.Height, header.ID())
	// if we never asked for this block, discard it
	if !status.WasQueued() {
		return false
	}
	// if we have already received this block, exit
	if status.WasReceived() {
		return false
	}

	// this is a new block, remember that we've seen it
	status.Header = header
	status.Received = time.Now()

	// track it by ID and by height so we don't accidentally request it again
	c.blockIDs[header.ID()] = status
	c.heights[header.Height] = status

	return true
}

// HandleHeight handles receiving a new highest finalized height from another node.
// If the height difference between local and the reported height, we do nothing.
// Otherwise, we queue each missing height.
func (c *Core) HandleHeight(final *flow.Header, height uint64) {
	// don't bother queueing anything if we're within tolerance
	if c.WithinTolerance(final, height) {
		return
	}

	// if we are sufficiently behind, we want to sync the missing blocks
	if height > final.Height {
		c.mu.Lock()
		defer c.mu.Unlock()

		// limit to request up to 2*MaxSize blocks from the peer.
		// without this limit, then if we are falling far behind,
		// we would queue up too many heights.
		heightLimit := final.Height + 2*uint64(c.Config.MaxSize)
		if height > heightLimit {
			height = heightLimit
		}

		for h := final.Height + 1; h <= height; h++ {
			c.requeueHeight(h)
		}
	}
}

func (c *Core) RequestBlock(blockID flow.Identifier) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// if we already received this block, reset the status so we can re-queue
	status := c.blockIDs[blockID]
	if status.WasReceived() {
		delete(c.blockIDs, status.Header.ID())
		delete(c.heights, status.Header.Height)
	}

	c.queueByBlockID(blockID)
}

func (c *Core) RequestHeight(height uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requeueHeight(height)
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
func (c *Core) ScanPending(final *flow.Header) ([]flow.Range, []flow.Batch) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// prune before doing any work
	c.prune(final)

	// get all items that are eligible for initial or re-requesting
	heights, blockIDs := c.getRequestableItems()

	// convert to valid range and batch requests
	ranges := c.getRanges(heights)
	batches := c.getBatches(blockIDs)

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

	// only queue the request if have never queued it before
	if c.heights[height].WasQueued() {
		return
	}

	// queue the request
	c.heights[height] = NewQueuedStatus()
}

// queueByBlockID queues a request for a block by block ID, only if no
// equivalent request has been queued before.
func (c *Core) queueByBlockID(blockID flow.Identifier) {

	// only queue the request if have never queued it before
	if c.blockIDs[blockID].WasQueued() {
		return
	}

	// queue the request
	c.blockIDs[blockID] = NewQueuedStatus()
}

// getRequestStatus retrieves a request status for a block, regardless of
// whether it was queued by height or by block ID.
func (c *Core) getRequestStatus(height uint64, blockID flow.Identifier) *Status {
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

	// track how many statuses we are pruning
	initialHeights := len(c.heights)
	initialBlockIDs := len(c.blockIDs)

	for height := range c.heights {
		if height <= final.Height {
			delete(c.heights, height)
			continue
		}
	}

	for blockID, status := range c.blockIDs {
		if status.WasReceived() {
			header := status.Header

			if header.Height <= final.Height {
				delete(c.blockIDs, blockID)
				continue
			}
		}
	}

	prunedHeights := initialHeights - len(c.heights)
	prunedBlockIDs := initialBlockIDs - len(c.blockIDs)

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
func (c *Core) RangeRequested(ran flow.Range) {
	c.mu.Lock()
	defer c.mu.Unlock()

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
func (c *Core) BatchRequested(batch flow.Batch) {
	c.mu.Lock()
	defer c.mu.Unlock()

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
func (c *Core) getRanges(heights []uint64) []flow.Range {

	// sort the heights so we can build contiguous ranges more easily
	sort.Slice(heights, func(i int, j int) bool {
		return heights[i] < heights[j]
	})

	// build contiguous height ranges with maximum batch size
	start := uint64(0)
	end := uint64(0)
	var ranges []flow.Range
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
			r := flow.Range{From: start, To: end}
			ranges = append(ranges, r)
			break
		}

		// at this point, we will have a next height as iteration will continue
		nextHeight := heights[index+1]

		// if we have reached the maximum size for a range, we create the range
		// and forward the start pointer to the next height
		rangeSize := end - start + 1
		if rangeSize >= uint64(c.Config.MaxSize) {
			r := flow.Range{From: start, To: end}
			ranges = append(ranges, r)
			start = nextHeight
			continue
		}

		// if end is more than one smaller than the next height, we have a gap
		// next, so we create a range and forward the start pointer
		if nextHeight > end+1 {
			r := flow.Range{From: start, To: end}
			ranges = append(ranges, r)
			start = nextHeight
			continue
		}
	}

	return ranges
}

// getBatches returns a set of batches that can be used in batch requests.
func (c *Core) getBatches(blockIDs []flow.Identifier) []flow.Batch {

	var batches []flow.Batch
	// split the block IDs into maximum sized requests
	for from := 0; from < len(blockIDs); from += int(c.Config.MaxSize) {

		// make sure last range is not out of bounds
		to := from + int(c.Config.MaxSize)
		if to > len(blockIDs) {
			to = len(blockIDs)
		}

		// create the block IDs slice
		requestIDs := blockIDs[from:to]
		batch := flow.Batch{
			BlockIDs: requestIDs,
		}
		batches = append(batches, batch)
	}

	return batches
}

// selectRequests selects which requests should be submitted, given a set of
// candidate range and batch requests. Range requests are given precedence and
// the total number of requests does not exceed the configured request maximum.
func (c *Core) selectRequests(ranges []flow.Range, batches []flow.Batch) ([]flow.Range, []flow.Batch) {
	max := int(c.Config.MaxRequests)

	if len(ranges) >= max {
		return ranges[:max], nil
	}
	if len(ranges)+len(batches) >= max {
		return ranges, batches[:max-len(ranges)]
	}
	return ranges, batches
}
