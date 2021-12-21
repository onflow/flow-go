package synchronization

import (
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// Core contains core logic, configuration, and state for chain state
// synchronization. It is generic to chain type, so it works for both consensus
// and collection nodes.
//
// Core should be wrapped by a type-aware engine that manages the specifics of
// each chain. Example: https://github.com/onflow/flow-go/blob/master/engine/common/synchronization/engine.go
//
// Core is safe for concurrent use by multiple goroutines.
type Core struct {
	log zerolog.Logger
	mu  sync.RWMutex

	activeRange           module.ActiveRange
	targetFinalizedHeight module.TargetFinalizedHeight

	blockIDs map[flow.Identifier]uint64

	config *Config

	finalizedHeader     *module.FinalizedHeaderCache
	lastFinalizedHeight uint64
}

var _ module.SyncCore = (*Core)(nil)
var _ module.BlockRequester = (*Core)(nil)

type Config struct {
	BlockHeightDifferenceThreshold uint64
	DefaultRangeSize               uint
}

func DefaultConfig() *Config {
	return &Config{
		BlockHeightDifferenceThreshold: 192,
		DefaultRangeSize:               192,
	}
}

func New(log zerolog.Logger, activeRange module.ActiveRange, targetFinalizedHeight module.TargetFinalizedHeight, config *Config) *Core {
	return &Core{
		log:                   log.With().Str("module", "synchronization").Logger(),
		blockIDs:              make(map[flow.Identifier]uint64),
		activeRange:           activeRange,
		targetFinalizedHeight: targetFinalizedHeight,
		config:                config,
	}
}

// RequestBlock indicates that the given block should be queued for retrieval.
func (c *Core) RequestBlock(blockID flow.Identifier, height uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	localHeight := c.finalizedHeader.Get().Height

	c.activeRange.LocalFinalizedHeight(localHeight)

	if height > localHeight+c.config.BlockHeightDifferenceThreshold || height <= localHeight {
		return
	}

	c.blockIDs[blockID] = height
}

// GetRequestableItems returns the set of requestable items.
func (c *Core) GetRequestableItems() (flow.Range, flow.Batch) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	localHeight := c.finalizedHeader.Get().Height

	c.activeRange.LocalFinalizedHeight(localHeight)
	c.activeRange.TargetFinalizedHeight(c.targetFinalizedHeight.Get())

	blockIDs := make([]flow.Identifier, 0, len(c.blockIDs))

	for blockID, height := range c.blockIDs {
		if height <= localHeight {
			delete(c.blockIDs, blockID)
		} else {
			blockIDs = append(blockIDs, blockID)
		}
	}

	return c.activeRange.Get(), flow.Batch{BlockIDs: blockIDs}
}

// RangeReceived updates sync state after a Range Request response is received.
func (c *Core) RangeReceived(headers []flow.Header, originID flow.Identifier) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.activeRange.Update(headers, originID)
}

// BatchReceived updates sync state after a Batch Request response is received.
func (c *Core) BatchReceived(headers []flow.Header, originID flow.Identifier) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, header := range headers {
		delete(c.blockIDs, header.ID())
	}
}

// HeightReceived updates sync state after a Sync Height response is received.
func (c *Core) HeightReceived(height uint64, originID flow.Identifier) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.targetFinalizedHeight.Update(height, originID)
}
