package verification

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
	"sync"
)

type StopControl struct {
	sync.RWMutex

	stopAtHeight        uint64
	commenced           bool // indicates if any stop-related chunks/block were skipped, disallow changing stop height if true
	log                 zerolog.Logger
	state               protocol.State
	lastFinalizedHeight uint64
}

func NewStopControl(log zerolog.Logger, state protocol.State, cp storage.ConsumerProgress) (*StopControl, error) {

	// TODO load and reset stop height sentinel

	processedHeight, err := cp.ProcessedIndex()
	if err != nil {
		head, err2 := state.Sealed().Head()
		if err2 != nil {
			return nil, fmt.Errorf("cannot get sealed height with error :%s, and block height processed index for stop control init: %w", err2.Error(), err)
		}
		processedHeight = head.Height
	}

	return &StopControl{
		lastFinalizedHeight: processedHeight,
		log:                 log,
		state:               state,
		commenced:           false,
	}, nil
}

func (c *StopControl) Finalized(height uint64) error {
	c.Lock()
	defer c.Unlock()

	if height > c.lastFinalizedHeight {
		c.lastFinalizedHeight = height
	}

	if c.stopAtHeight > 0 {
		highestSealed, err := c.highestSealed()
		if err != nil {
			//c.log.Fatal().Err(err).Msg("cannot query highest sealed height")
			return err
		}
		if highestSealed >= c.stopAtHeight-1 {
			c.commenced = true

			// start crash sequence
			// TODO put restart sentinel in a DB with restart height of e.stopHeight-1, after restart VN star processing at e.stopHeight
			c.log.Fatal().Msgf("block sealed at height %d - stopping node, since stop at %d requested", highestSealed, c.stopAtHeight)
		}
	}
	return nil
}

func (c *StopControl) ShouldSkipChunk(chunk *flow.Chunk) (bool, error) {
	c.Lock()
	defer c.Unlock()

	if c.stopAtHeight > 0 {
		heightForBlock, err := c.heightForBlock(chunk.BlockID)
		if err != nil {
			c.log.Fatal().Err(err).Msg("cannot query height for a block")
			//return false, nil
		}
		return c.shouldSkipChunkAtHeight(heightForBlock), nil
	}
	return false, nil
}

func (c *StopControl) ShouldSkipChunkAtHeight(height uint64) bool {
	c.Lock()
	defer c.Unlock()

	return c.shouldSkipChunkAtHeight(height)
}

func (c *StopControl) shouldSkipChunkAtHeight(height uint64) bool {
	if c.stopAtHeight > 0 {
		if height >= c.stopAtHeight {
			c.commenced = true
			return true
		}
	}
	return false
}

func (c *StopControl) highestSealed() (uint64, error) {
	sealed, err := c.state.Sealed().Head()
	if err != nil {
		return 0, fmt.Errorf("cannot query head of sealed state: %w", err)
	}
	return sealed.Height, nil
}

func (c *StopControl) heightForBlock(id flow.Identifier) (uint64, error) {
	header, err := c.state.AtBlockID(id).Head()
	if err != nil {
		return 0, fmt.Errorf("cannot query state at block %s: %w", id, err)
	}
	return header.Height, nil
}

func (c *StopControl) GetStopHeight() uint64 {
	c.RLock()
	defer c.RUnlock()

	return c.stopAtHeight
}

func (c *StopControl) SetStopHeight(height uint64) (uint64, error) {
	c.Lock()
	defer c.Unlock()

	if height < c.lastFinalizedHeight {
		return 0, fmt.Errorf("cannot set stop height (%d) below last finalied height (%d)", height, c.lastFinalizedHeight)
	}
	if c.commenced {
		return 0, fmt.Errorf("one or more stop-related data was already skipped, disallowing changing stop height (%d)", c.stopAtHeight)
	}

	old := c.stopAtHeight

	c.stopAtHeight = height

	return old, nil
}

func (c *StopControl) HasCommenced() bool {
	c.RLock()
	defer c.RUnlock()

	return c.commenced
}
