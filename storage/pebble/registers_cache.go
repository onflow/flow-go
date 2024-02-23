package pebble

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

const (
	registerResourceName = "registers"
)

type RegistersCache struct {
	registers *Registers
	cache     *ReadCache
}

var _ storage.RegisterIndex = (*RegistersCache)(nil)

// NewRegistersCache wraps a read cache around Get requests to a underlying Registers object.
func NewRegistersCache(registers *Registers, cacheType CacheType, size uint, metrics module.CacheMetrics) (*RegistersCache, error) {
	if size == 0 {
		return nil, errors.New("cache size cannot be 0")
	}

	cache, err := newReadCache(
		metrics,
		registerResourceName,
		cacheType,
		size,
		func(key string) (flow.RegisterValue, error) {
			return registers.lookupRegister([]byte(key))
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not create cache: %w", err)
	}

	return &RegistersCache{
		registers: registers,
		cache:     cache,
	}, nil
}

// Get returns the most recent updated payload for the given RegisterID.
// "most recent" means the updates happens most recent up the given height.
//
// For example, if there are 2 values stored for register A at height 6 and 11, then
// GetPayload(13, A) would return the value at height 11.
//
// - storage.ErrNotFound if no register values are found
// - storage.ErrHeightNotIndexed if the requested height is out of the range of stored heights
func (c *RegistersCache) Get(
	reg flow.RegisterID,
	height uint64,
) (flow.RegisterValue, error) {
	return c.cache.Get(newLookupKey(height, reg).String())
}

// Store sets the given entries in a batch.
// This function is expected to be called at one batch per height, sequentially. Under normal conditions,
// it should be called wth the value of height set to LatestHeight + 1
// CAUTION: This function is not safe for concurrent use.
func (c *RegistersCache) Store(
	entries flow.RegisterEntries,
	height uint64,
) error {
	return c.registers.Store(entries, height)
}

// LatestHeight Gets the latest height of complete registers available
func (c *RegistersCache) LatestHeight() uint64 {
	return c.registers.LatestHeight()
}

// FirstHeight first indexed height found in the store, typically root block for the spork
func (c *RegistersCache) FirstHeight() uint64 {
	return c.registers.FirstHeight()
}
