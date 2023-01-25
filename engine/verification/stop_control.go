package verification

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/rs/zerolog"
)

type StopControl struct {
	stopAtHeight uint64
	log          zerolog.Logger
	state        protocol.State
}

func NewStopControl(stopHeight uint64, state protocol.State, log zerolog.Logger) *StopControl {
	return &StopControl{
		stopAtHeight: stopHeight,
		log:          log,
		state:        state,
	}
}

func (c *StopControl) Finalized() error {
	if c.stopAtHeight > 0 {
		highestSealed, err := c.highestSealed()
		if err != nil {
			//c.log.Fatal().Err(err).Msg("cannot query highest sealed height")
			return err
		}
		if highestSealed >= c.stopAtHeight-1 {
			// start crash sequence
			// TODO put restart sentinel in a DB with restart height of e.stopHeight-1, after restart VN star processing at e.stopHeight
			c.log.Fatal().Msgf("block sealed at height %d - stopping node, since stop at %d requested", highestSealed, c.stopAtHeight)
		}
	}
	return nil
}

func (c *StopControl) ShouldSkipChunk(chunk *flow.Chunk) (bool, error) {
	if c.stopAtHeight > 0 {
		heightForBlock, err := c.heightForBlock(chunk.BlockID)
		if err != nil {
			c.log.Fatal().Err(err).Msg("cannot query height for a block")
			//return false, nil
		}
		return c.ShouldSkipChunkAtHeight(heightForBlock), nil
	}
	return false, nil
}

func (c *StopControl) ShouldSkipChunkAtHeight(height uint64) bool {
	if c.stopAtHeight > 0 {
		if height >= c.stopAtHeight {
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
	return c.stopAtHeight
}
