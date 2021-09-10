package voteaggregator

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
)

// NewCollectorFactoryMethod is a factory method to generate a VoteCollector for concrete view
type NewCollectorFactoryMethod = func(view uint64) (hotstuff.VoteCollector, error)

type VoteCollectors struct {
	lock            sync.RWMutex
	lowestLevel     uint64                            // lowest view that we have pruned up to
	collectors      map[uint64]hotstuff.VoteCollector // view -> VoteCollector
	createCollector NewCollectorFactoryMethod         // factory method for creating collectors
}

var _ hotstuff.VoteCollectors = &VoteCollectors{}

func NewVoteCollectors(lowestLevel uint64, factoryMethod NewCollectorFactoryMethod) *VoteCollectors {
	return &VoteCollectors{
		lowestLevel:     lowestLevel,
		collectors:      make(map[uint64]hotstuff.VoteCollector),
		createCollector: factoryMethod,
	}
}

// GetOrCreateCollector performs lazy initialization of collectors based on their view
func (v *VoteCollectors) GetOrCreateCollector(view uint64) (hotstuff.VoteCollector, bool, error) {
	cachedCollector := v.getCollector(view)
	if cachedCollector != nil {
		return cachedCollector, false, nil
	}

	collector, err := v.createCollector(view)
	if err != nil {
		return nil, false, fmt.Errorf("could not create vote collector for view %d: %w", view, err)
	}

	// Initial check showed that there was no collector. However, it's possible that after the
	// initial check but before acquiring the lock to add the newly-created collector, another
	// goroutine already added the needed collector. Hence, check again after acquiring the lock:
	v.lock.Lock()
	defer v.lock.Unlock()
	clr, found := v.collectors[view]
	if found {
		return clr, false, nil
	}

	v.collectors[view] = collector
	return collector, true, nil
}

func (v *VoteCollectors) getCollector(view uint64) hotstuff.VoteCollector {
	v.lock.RLock()
	defer v.lock.RUnlock()
	return v.collectors[view]
}

// PruneUpToView prunes all collectors below view, sets the lowest level to that value
func (v *VoteCollectors) PruneUpToView(view uint64) error {
	v.lock.Lock()
	defer v.lock.Unlock()
	if v.lowestLevel >= view {
		return nil
	}

	if len(v.collectors) == 0 {
		v.lowestLevel = 0
		return nil
	}

	for l := v.lowestLevel; l < view; l++ {
		delete(v.collectors, l)
	}

	return nil
}
