package events

import (
	"fmt"
	"sync/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/state/protocol"
)

// FinalizedHeaderCache caches a copy of the most recently finalized block header by
// consuming BlockFinalized events from HotStuff, using a FinalizationActor.
// The constructor returns both the cache and a worker function.
//
// NOTE: The protocol state already guarantees that state.Final().Head() will be cached, however,
// since the protocol state is shared among many components, there may be high contention on its cache.
// The FinalizedHeaderCache can be used in place of state.Final().Head() to avoid read contention with other components.
type FinalizedHeaderCache struct {
	state              protocol.State
	val                *atomic.Pointer[flow.Header]
	*FinalizationActor // expose OnBlockFinalized method
}

var _ module.FinalizedHeaderCache = (*FinalizedHeaderCache)(nil)

// Get returns the most recently finalized block.
// Guaranteed to be non-nil after construction.
func (cache *FinalizedHeaderCache) Get() *flow.Header {
	return cache.val.Load()
}

// update reads the latest finalized header and updates the cache.
// No errors are expected during normal operation.
func (cache *FinalizedHeaderCache) update() error {
	final, err := cache.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not retrieve latest finalized header: %w", err)
	}
	cache.val.Store(final)
	return nil
}

// NewFinalizedHeaderCache returns a new FinalizedHeaderCache subscribed to the given FinalizationDistributor,
// and the ComponentWorker function to maintain the cache.
// The caller MUST start the returned ComponentWorker in a goroutine to maintain the cache.
// No errors are expected during normal operation.
func NewFinalizedHeaderCache(state protocol.State) (*FinalizedHeaderCache, component.ComponentWorker, error) {
	cache := &FinalizedHeaderCache{
		state: state,
		val:   new(atomic.Pointer[flow.Header]),
	}
	// initialize the cache with the current finalized header
	if err := cache.update(); err != nil {
		return nil, nil, fmt.Errorf("could not initialize cache: %w", err)
	}

	// create a worker to continuously track the latest finalized header
	actor, worker := NewFinalizationActor(func(_ *model.Block) error {
		return cache.update()
	})
	cache.FinalizationActor = actor

	return cache, worker, nil
}
