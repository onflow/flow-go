package ingestion

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// DefaultCatchUpThreshold is the number of blocks that if the execution is far behind
// the finalization then we will only lazy load the next unexecuted finalized
// blocks until the execution has caught up
const DefaultCatchUpThreshold = 500

// BlockIDHeight is a helper struct that holds the block ID and height
type BlockIDHeight struct {
	ID     flow.Identifier
	Height uint64
}

func HeaderToBlockIDHeight(header *flow.Header) BlockIDHeight {
	return BlockIDHeight{
		ID:     header.ID(),
		Height: header.Height,
	}
}

// Throttle is used to throttle the blocks to be added to the processables channel
type Throttle interface {
	// Init initializes the throttle with the processables channel to forward the blocks
	Init(processables chan<- BlockIDHeight, threshold int) error
	// OnBlock is called when a block is received, the throttle will check if the execution
	// is falling far behind the finalization, and add the block to the processables channel
	// if it's not falling far behind.
	OnBlock(blockID flow.Identifier, height uint64) error
	// OnBlockExecuted is called when a block is executed, the throttle will check whether
	// the execution is caught up with the finalization, and allow all the remaining blocks
	// to be added to the processables channel.
	OnBlockExecuted(blockID flow.Identifier, height uint64) error
	// OnBlockFinalized is called when a block is finalized, the throttle will update the
	// finalized height.
	OnBlockFinalized(height uint64)
	// Done stops the throttle, and stop sending new blocks to the processables channel
	Done() error
}

var _ Throttle = (*BlockThrottle)(nil)

// BlockThrottle is a helper struct that helps throttle the unexecuted blocks to be sent
// to the block queue for execution.
// It is useful for case when execution is falling far behind the finalization, in which case
// we want to throttle the blocks to be sent to the block queue for fetching data to execute
// them. Without throttle, the block queue will be flooded with blocks, and the network
// will be flooded with requests fetching collections, and the EN might quickly run out of memory.
type BlockThrottle struct {
	// when initialized, if the execution is falling far behind the finalization, then
	// the throttle will only load the next "throttle" number of unexecuted blocks to processables,
	// and ignore newly received blocks until the execution has caught up the finalization.
	// During the catching up phase, after a block is executed, the throttle will load the next block
	// to processables, and keep doing so until the execution has caught up the finalization.
	// Once caught up, the throttle will process all the remaining unexecuted blocks, including
	// unfinalized blocks.
	mu        sync.Mutex
	stopped   bool   // whether the throttle is stopped, if true, no more block will be loaded
	loadedAll bool   // whether all blocks have been loaded. if true, no block will be throttled.
	loaded    uint64 // the last block height pushed to processables. Used to track if has caught up
	finalized uint64 // the last finalized height. Used to track if has caught up

	// notifier
	processables chan<- BlockIDHeight

	// dependencies
	log     zerolog.Logger
	state   protocol.State
	headers storage.Headers
}

func NewBlockThrottle(
	log zerolog.Logger,
	state protocol.State,
	execState state.ExecutionState,
	headers storage.Headers,
) (*BlockThrottle, error) {
	finalizedHead, err := state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized head: %w", err)
	}

	finalized := finalizedHead.Height
	executed, err := execState.GetHighestFinalizedExecuted()
	if err != nil {
		return nil, fmt.Errorf("could not get highest finalized executed: %w", err)
	}

	if executed > finalized {
		return nil, fmt.Errorf("executed finalized %v is greater than finalized %v", executed, finalized)
	}

	return &BlockThrottle{
		loaded:    executed,
		finalized: finalized,
		stopped:   false,
		loadedAll: false,

		log:     log.With().Str("component", "block_throttle").Logger(),
		state:   state,
		headers: headers,
	}, nil
}

// inited returns true if the throttle has been inited
func (c *BlockThrottle) inited() bool {
	return c.processables != nil
}

func (c *BlockThrottle) Init(processables chan<- BlockIDHeight, threshold int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inited() {
		return fmt.Errorf("throttle already inited")
	}

	c.processables = processables

	lastFinalizedToLoad := c.loaded + uint64(threshold)
	if lastFinalizedToLoad > c.finalized {
		lastFinalizedToLoad = c.finalized
	}

	loadedAll := lastFinalizedToLoad == c.finalized

	lg := c.log.With().
		Uint64("executed", c.loaded).
		Uint64("finalized", c.finalized).
		Uint64("lastFinalizedToLoad", lastFinalizedToLoad).
		Int("threshold", threshold).
		Bool("loadedAll", loadedAll).
		Logger()

	lg.Info().Msgf("finding finalized blocks")

	unexecuted, err := findFinalized(c.state, c.headers, c.loaded, lastFinalizedToLoad)
	if err != nil {
		return err
	}

	if loadedAll {
		pendings, err := findAllPendingBlocks(c.state, c.headers, c.finalized)
		if err != nil {
			return err
		}
		unexecuted = append(unexecuted, pendings...)
	}

	lg = lg.With().Int("unexecuted", len(unexecuted)).
		Logger()

	lg.Debug().Msgf("initializing throttle")

	// the ingestion core engine must have initialized the 'processables' with 10000 (default) buffer size,
	// and the 'unexecuted' will only contain up to DefaultCatchUpThreshold (500) blocks,
	// so pushing all the unexecuted to processables won't be blocked.
	for _, b := range unexecuted {
		c.processables <- b
		c.loaded = b.Height
	}

	c.loadedAll = loadedAll

	lg.Info().Msgf("throttle initialized unexecuted blocks")

	return nil
}

func (c *BlockThrottle) OnBlockExecuted(_ flow.Identifier, executed uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.inited() {
		return fmt.Errorf("throttle not inited")
	}

	if c.stopped {
		return nil
	}

	// we have already caught up, ignore
	if c.caughtUp() {
		return nil
	}

	// in this case, c.loaded must be < c.finalized
	// so we must be able to load the next block
	err := c.loadNextBlock(c.loaded)
	if err != nil {
		return fmt.Errorf("could not load next block: %w", err)
	}

	if !c.caughtUp() {
		// after loading the next block, if the execution height is no longer
		// behind the finalization height
		return nil
	}

	c.log.Info().Uint64("executed", executed).Uint64("finalized", c.finalized).
		Uint64("loaded", c.loaded).
		Msgf("execution has caught up, processing remaining unexecuted blocks")

	// if the execution have just caught up close enough to the latest finalized blocks,
	// then process all unexecuted blocks, including finalized unexecuted and pending unexecuted
	unexecuted, err := findAllPendingBlocks(c.state, c.headers, c.finalized)
	if err != nil {
		return fmt.Errorf("could not find unexecuted blocks for processing: %w", err)
	}

	c.log.Info().Int("unexecuted", len(unexecuted)).Msgf("forwarding unexecuted blocks")

	for _, block := range unexecuted {
		c.processables <- block
		c.loaded = block.Height
	}

	c.log.Info().Msgf("all unexecuted blocks have been processed")

	return nil
}

// Done marks the throttle as done, and no more blocks will be processed
func (c *BlockThrottle) Done() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.log.Info().Msgf("throttle done")

	if !c.inited() {
		return fmt.Errorf("throttle not inited")
	}

	c.stopped = true

	return nil
}

func (c *BlockThrottle) OnBlock(blockID flow.Identifier, height uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.log.Debug().Msgf("recieved block (%v) height: %v", blockID, height)

	if !c.inited() {
		return fmt.Errorf("throttle not inited")
	}

	if c.stopped {
		return nil
	}

	// ignore the block if has not caught up.
	if !c.caughtUp() {
		return nil
	}

	// if has caught up, then process the block
	c.processables <- BlockIDHeight{
		ID:     blockID,
		Height: height,
	}
	c.loaded = height
	c.log.Debug().Msgf("processed block (%v), height: %v", blockID, height)

	return nil
}

func (c *BlockThrottle) OnBlockFinalized(finalizedHeight uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.inited() {
		return
	}

	if c.caughtUp() {
		// once caught up, all unfinalized blocks will be loaded, and loadedAll will be set to true
		// which will always be caught up, so we don't need to update finalized height any more.
		return
	}

	if finalizedHeight <= c.finalized {
		return
	}

	c.finalized = finalizedHeight
}

func (c *BlockThrottle) loadNextBlock(height uint64) error {
	c.log.Debug().Uint64("height", height).Msg("loading next block")
	// load next block
	next := height + 1
	blockID, err := c.headers.BlockIDByHeight(next)
	if err != nil {
		return fmt.Errorf("could not get block ID by height %v: %w", next, err)
	}

	c.processables <- BlockIDHeight{
		ID:     blockID,
		Height: next,
	}
	c.loaded = next
	c.log.Debug().Uint64("height", next).Msg("loaded next block")

	return nil
}

func (c *BlockThrottle) caughtUp() bool {
	// load all pending blocks should only happen at most once.
	// if the execution is already caught up finalization during initialization,
	// then loadedAll is true, and we don't need to catch up again.
	// if the execution was falling behind finalization, and has caught up,
	// then loadedAll is also true, and we don't need to catch up again, because
	// otherwise we might load the same block twice.
	if c.loadedAll {
		return true
	}

	// in this case, the execution was falling behind the finalization during initialization,
	// whether the execution has caught up is determined by whether the loaded block is equal
	// to or above the finalized block.
	return c.loaded >= c.finalized
}

func findFinalized(state protocol.State, headers storage.Headers, lastExecuted, finalizedHeight uint64) ([]BlockIDHeight, error) {
	// get finalized height
	finalized := state.AtHeight(finalizedHeight)
	final, err := finalized.Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized block: %w", err)
	}

	// dynamically bootstrapped execution node will have highest finalized executed as sealed root,
	// which is lower than finalized root. so we will reload blocks from
	// [sealedRoot.Height + 1, finalizedRoot.Height] and execute them on startup.
	unexecutedFinalized := make([]BlockIDHeight, 0)

	// starting from the first unexecuted block, go through each unexecuted and finalized block
	for height := lastExecuted + 1; height <= final.Height; height++ {
		finalizedID, err := headers.BlockIDByHeight(height)
		if err != nil {
			return nil, fmt.Errorf("could not get block ID by height %v: %w", height, err)
		}

		unexecutedFinalized = append(unexecutedFinalized, BlockIDHeight{
			ID:     finalizedID,
			Height: height,
		})
	}

	return unexecutedFinalized, nil
}

func findAllPendingBlocks(state protocol.State, headers storage.Headers, finalizedHeight uint64) ([]BlockIDHeight, error) {
	// loaded all pending blocks
	pendings, err := state.AtHeight(finalizedHeight).Descendants()
	if err != nil {
		return nil, fmt.Errorf("could not get descendants of finalized block: %w", err)
	}

	unexecuted := make([]BlockIDHeight, 0, len(pendings))
	for _, id := range pendings {
		header, err := headers.ByBlockID(id)
		if err != nil {
			return nil, fmt.Errorf("could not get header by block ID %v: %w", id, err)
		}
		unexecuted = append(unexecuted, BlockIDHeight{
			ID:     id,
			Height: header.Height,
		})
	}

	return unexecuted, nil
}
