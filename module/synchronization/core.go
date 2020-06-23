package synchronization

import (
	"fmt"
	"sort"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
)

type Config struct {
	PollInterval  time.Duration // how often we poll other nodes for their finalized height
	ScanInterval  time.Duration // how often we scan our pending statuses and request blocks
	RetryInterval time.Duration // the initial interval before we retry a request, uses exponential backoff
	Tolerance     uint          // determines how big of a difference in block heights we tolerated before actively syncing with range requests
	MaxAttempts   uint          // the maximum number of attempts we make for each requested block/height before discarding
	MaxSize       uint          // the maximum number of blocks we request in the same block request message
	MaxRequests   uint          // the maximum number of requests we send during each scanning period
}

func DefaultConfig() Config {
	return Config{
		PollInterval:  8 * time.Second,
		ScanInterval:  2 * time.Second,
		RetryInterval: 4 * time.Second,
		Tolerance:     10,
		MaxAttempts:   5,
		MaxSize:       64,
		MaxRequests:   3,
	}
}

type Core struct {
	log      zerolog.Logger
	Config   Config
	heights  map[uint64]*Status
	blockIDs map[flow.Identifier]*Status
}

func New(log zerolog.Logger, config Config) (*Core, error) {
	core := &Core{
		log:      log,
		Config:   config,
		heights:  make(map[uint64]*Status),
		blockIDs: make(map[flow.Identifier]*Status),
	}
	return core, nil
}

// HandleHeight handles receiving a new highest finalized height from another.
// We queue any new heights and return
func (c *Core) HandleHeight(final *flow.Header, height uint64) {

	// don't bother queueing anything if we're within tolerance
	if c.WithinTolerance(final, height) {
		return
	}

	// if we are sufficiently behind, we want to sync the missing blocks
	if height > final.Height {
		for h := final.Height + 1; h <= height; h++ {
			c.QueueByHeight(h)
		}
	}
}

// HandleBlock handles receiving a new block from another node. It returns
// true if the block should be processed by the compliance layer and false
// if it should be ignored.
func (c *Core) HandleBlock(header *flow.Header) bool {

	status := c.GetRequestStatus(header.Height, header.ID())
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

// QueueByHeight queues a request for the finalized block at the given height,
// only if no equivalent request has been queued before.
func (c *Core) QueueByHeight(height uint64) {

	// only queue the request if have never queued it before
	if c.heights[height].WasQueued() {
		return
	}

	// queue the request
	c.heights[height] = NewQueuedStatus()
}

// QueueByBlockID queues a request for a block by block ID, only if no
// equivalent request has been queued before.
func (c *Core) QueueByBlockID(blockID flow.Identifier) {

	// only queue the request if have never queued it before
	if c.blockIDs[blockID].WasQueued() {
		return
	}

	// queue the request
	c.blockIDs[blockID] = NewQueuedStatus()
}

func (c *Core) RequestBlock(blockID flow.Identifier) {

	// if we already received this block, reset the status so we can re-queue
	status := c.blockIDs[blockID]
	if status.WasReceived() {
		delete(c.blockIDs, status.Header.ID())
		delete(c.heights, status.Header.Height)
	}

	c.QueueByBlockID(blockID)
}

// GetRequestStatus retrieves a request status for a block, regardless of
// whether it was queued by height or by block ID.
func (c *Core) GetRequestStatus(height uint64, blockID flow.Identifier) *Status {
	heightStatus := c.heights[height]
	idStatus := c.blockIDs[blockID]

	if heightStatus.WasQueued() {
		return heightStatus
	}
	if idStatus.WasQueued() {
		return idStatus
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

	prunedHeights := len(c.heights) - initialHeights
	prunedBlockIDs := len(c.blockIDs) - initialBlockIDs
	c.log.Debug().
		Uint64("final_height", final.Height).
		Msgf("pruned %d heights, %d block IDs", prunedHeights, prunedBlockIDs)
}

// ScanPending scans all pending block statuses for blocks that should be
// requested. It apportions requestable items into range and batch requests
// according to configured maximums, giving precedence to range requests.
func (c *Core) ScanPending(final *flow.Header) ([]Range, []Batch, error) {

	heights, blockIDs, err := c.getRequestableItems(final)
	if err != nil {
		return nil, nil, fmt.Errorf("could not scan for requestable items: %w", err)
	}

	ranges, batches := c.getRequests(heights, blockIDs)
	return ranges, batches, nil
}

// getRequestableItems will check which items shall be requested.
func (c *Core) getRequestableItems(final *flow.Header) ([]uint64, []flow.Identifier, error) {

	// TODO: we will probably want to limit the maximum amount of in-flight
	// requests and maximum amount of blocks requested at the same time here;
	// for now, we just ignore that problem, but once we do, we should always
	// prioritize range requests over batch requests

	now := time.Now()

	// prune before doing any work
	c.prune(final)

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

	return heights, blockIDs, nil
}

// getRequests will divide the given heights and block IDs appropriately and
// request the desired blocks.
func (c *Core) getRequests(heights []uint64, blockIDs []flow.Identifier) ([]Range, []Batch) {

	// get all valid ranges and batches
	ranges := c.getRanges(heights)
	batches := c.getBatches(blockIDs)

	// pick some ranges/batches to request, up to the maximum of `totalRequests`
	// and giving precedence to range requests
	var rangesToRequest []Range
	var batchesToRequest []Batch

	for _, ran := range ranges {
		// check if the number of ranges exceeds the maximum requests
		if uint(len(rangesToRequest)) >= c.Config.MaxRequests {
			break
		}

		// mark all of the heights as requested
		for height := ran.From; height <= ran.To; height++ {
			// NOTE: during the short window between scan and send, we could
			// have evicted a status
			status, exists := c.heights[height]
			if !exists {
				continue
			}
			status.Requested = time.Now()
			status.Attempts++
		}

		rangesToRequest = append(rangesToRequest, ran)
	}

	for _, batch := range batches {
		// check if the number of batches exceeds the maximum requests
		if uint(len(rangesToRequest)+len(batchesToRequest)) >= c.Config.MaxRequests {
			break
		}

		// mark all the IDs as requested
		for _, id := range batch.BlockIDs {
			// NOTE: during the short window between scan and send, we could
			// have evicted a status
			status, exists := c.blockIDs[id]
			if !exists {
				continue
			}
			status.Requested = time.Now()
			status.Attempts++
		}

		batchesToRequest = append(batchesToRequest, batch)
	}

	return rangesToRequest, batchesToRequest
}

// getRanges returns a set of ranges of heights that can be used as range
// requests.
func (c *Core) getRanges(heights []uint64) []Range {

	// sort the heights so we can build contiguous ranges more easily
	sort.Slice(heights, func(i int, j int) bool {
		return heights[i] < heights[j]
	})

	// build contiguous height ranges with maximum batch size
	start := uint64(0)
	end := uint64(0)
	var ranges []Range
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
			r := Range{From: start, To: end}
			ranges = append(ranges, r)
			break
		}

		// at this point, we will have a next height as iteration will continue
		nextHeight := heights[index+1]

		// if we have reached the maximum size for a range, we create the range
		// and forward the start pointer to the next height
		rangeSize := end - start + 1
		if rangeSize >= uint64(c.Config.MaxSize) {
			r := Range{From: start, To: end}
			ranges = append(ranges, r)
			start = nextHeight
			continue
		}

		// if end is more than one smaller than the next height, we have a gap
		// next, so we create a range and forward the start pointer
		if nextHeight > end+1 {
			r := Range{From: start, To: end}
			ranges = append(ranges, r)
			start = nextHeight
			continue
		}
	}

	return ranges
}

// getBatches returns a set of batches that can be used in batch requests.
func (c *Core) getBatches(blockIDs []flow.Identifier) []Batch {

	now := time.Now()

	var batches []Batch
	// split the block IDs into maximum sized requests
	for from := 0; from < len(blockIDs); from += int(c.Config.MaxSize) {

		// make sure last range is not out of bounds
		to := from + int(c.Config.MaxSize)
		if to > len(blockIDs) {
			to = len(blockIDs)
		}

		// create the block IDs slice
		requestIDs := blockIDs[from:to]
		batch := Batch{
			BlockIDs: requestIDs,
		}
		batches = append(batches, batch)

		// mark all of the blocks as requested
		for _, blockID := range requestIDs {
			// NOTE: during the short window between scan and send, we could
			// have received a block and removed a key
			status := c.blockIDs[blockID]
			if status.WasReceived() {
				continue
			}
			status.Requested = now
			status.Attempts++
		}
	}

	return batches
}
