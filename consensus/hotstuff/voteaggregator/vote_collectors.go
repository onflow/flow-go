package voteaggregator

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/module/mempool"
)

// NewCollectorFactoryMethod is a factory method to generate a VoteCollector for concrete view
type NewCollectorFactoryMethod = func(view uint64) (hotstuff.VoteCollector, error)

// VoteCollectors implements management of multiple vote collectors indexed by view.
// Implements hotstuff.VoteCollectors interface. Creating a VoteCollector for a
// particular view is lazy (instances are created on demand).
// This structure is concurrently safe.
type VoteCollectors struct {
	lock            sync.RWMutex
	lowestView     uint64                            // lowest view that we have pruned up to
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
	cachedCollector, err := v.getCollector(view)
	if err != nil {
		return nil, false, err
	}

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

func (v *VoteCollectors) getCollector(view uint64) (hotstuff.VoteCollector, error) {
	v.lock.RLock()
	defer v.lock.RUnlock()
	if view < v.lowestLevel {
		return nil, mempool.NewDecreasingPruningHeightErrorf("cannot retrieve collector for pruned view %d (lowest retained view %d)", view, v.lowestLevel)
	}

	return v.collectors[view], nil
}

// PruneUpToView prunes all collectors below view, sets the lowest level to that value
func (v *VoteCollectors) PruneUpToView(view uint64) error {
	v.lock.Lock()
	defer v.lock.Unlock()
	if v.lowestLevel >= view {
		return nil
	}
	if len(v.collectors) == 0 {
		v.lowestView = view
		return nil
	}

	// to optimize the pruning of large view-ranges, we compare:
	//  * the number of views for which we have collectors: len(v.collectors)
	//  * the number of views that need to be pruned: view-v.lowestView
	// We iterate over the dimension which is smaller.
	if uint64(len(v.collectors)) < view-v.lowestView {
		for w, _ := range v.collectors {
			if w < view {
				delete(v.collectors, w)
			}
		}
	} else {
		for w := v.lowestView; w < view; w++ {
			delete(v.collectors, w)
		}
	}
	v.lowestLevel = view

	return nil
}
