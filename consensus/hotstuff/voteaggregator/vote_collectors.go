package voteaggregator

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool"
)

// NewCollectorFactoryMethod is a factory method to generate a VoteCollector for concrete view
type NewCollectorFactoryMethod = func(view uint64, workers hotstuff.Workers) (hotstuff.VoteCollector, error)

// VoteCollectors implements management of multiple vote collectors indexed by view.
// Implements hotstuff.VoteCollectors interface. Creating a VoteCollector for a
// particular view is lazy (instances are created on demand).
// This structure is concurrently safe.
type VoteCollectors struct {
	*component.ComponentManager
	log                zerolog.Logger
	lock               sync.RWMutex
	lowestRetainedView uint64                            // lowest view, for which we still retain a VoteCollector and process votes
	collectors         map[uint64]hotstuff.VoteCollector // view -> VoteCollector
	workerPool         hotstuff.Workerpool               // for processing votes that are already cached in VoteCollectors and waiting for respective block
	createCollector    NewCollectorFactoryMethod         // factory method for creating collectors
}

var _ hotstuff.VoteCollectors = (*VoteCollectors)(nil)

func NewVoteCollectors(logger zerolog.Logger, lowestRetainedView uint64, workerPool hotstuff.Workerpool, factoryMethod NewCollectorFactoryMethod) *VoteCollectors {
	// Component manager for wrapped worker pool
	componentBuilder := component.NewComponentManagerBuilder()
	componentBuilder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		ready()
		<-ctx.Done()          // wait for parent context to signal shutdown
		workerPool.StopWait() // wait till all workers exit
	})

	return &VoteCollectors{
		log:                logger,
		ComponentManager:   componentBuilder.Build(),
		lowestRetainedView: lowestRetainedView,
		collectors:         make(map[uint64]hotstuff.VoteCollector),
		workerPool:         workerPool,
		createCollector:    factoryMethod,
	}
}

// GetOrCreateCollector retrieves the hotstuff.VoteCollector for the specified
// view or creates one if none exists.
//  -  (collector, true, nil) if no collector can be found by the block ID, and a new collector was created.
//  -  (collector, false, nil) if the collector can be found by the block ID
//  -  (nil, false, error) if running into any exception creating the vote collector state machine
// Expected error returns during normal operations:
//  * mempool.DecreasingPruningHeightError - in case view is lower than lowestRetainedView
func (v *VoteCollectors) GetOrCreateCollector(view uint64) (hotstuff.VoteCollector, bool, error) {
	cachedCollector, hasCachedCollector, err := v.getCollector(view)
	if err != nil {
		return nil, false, err
	}

	if hasCachedCollector {
		return cachedCollector, false, nil
	}

	collector, err := v.createCollector(view, v.workerPool)
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

// getCollector retrieves hotstuff.VoteCollector from local cache in concurrent safe way.
// Performs check for lowestRetainedView.
// Expected error returns during normal operations:
//  * mempool.DecreasingPruningHeightError - in case view is lower than lowestRetainedView
func (v *VoteCollectors) getCollector(view uint64) (hotstuff.VoteCollector, bool, error) {
	v.lock.RLock()
	defer v.lock.RUnlock()
	if view < v.lowestRetainedView {
		return nil, false, mempool.NewDecreasingPruningHeightErrorf("cannot retrieve collector for pruned view %d (lowest retained view %d)", view, v.lowestRetainedView)
	}

	clr, found := v.collectors[view]

	return clr, found, nil
}

// PruneUpToView prunes the vote collectors with views _below_ the given value, i.e.
// we only retain and process whose view is equal or larger than `lowestRetainedView`.
// If `lowestRetainedView` is smaller than the previous value, the previous value is
// kept and the method call is a NoOp.
func (v *VoteCollectors) PruneUpToView(lowestRetainedView uint64) {
	v.lock.Lock()
	defer v.lock.Unlock()
	if v.lowestRetainedView >= lowestRetainedView {
		return
	}
	if len(v.collectors) == 0 {
		v.lowestRetainedView = lowestRetainedView
		return
	}

	sizeBefore := len(v.collectors)

	// to optimize the pruning of large view-ranges, we compare:
	//  * the number of views for which we have collectors: len(v.collectors)
	//  * the number of views that need to be pruned: view-v.lowestRetainedView
	// We iterate over the dimension which is smaller.
	if uint64(len(v.collectors)) < lowestRetainedView-v.lowestRetainedView {
		for w := range v.collectors {
			if w < lowestRetainedView {
				delete(v.collectors, w)
			}
		}
	} else {
		for w := v.lowestRetainedView; w < lowestRetainedView; w++ {
			delete(v.collectors, w)
		}
	}
	from := v.lowestRetainedView
	v.lowestRetainedView = lowestRetainedView

	v.log.Debug().
		Uint64("prior_lowest_retained_view", from).
		Uint64("lowest_retained_view", lowestRetainedView).
		Int("prior_size", sizeBefore).
		Int("size", len(v.collectors)).
		Msgf("pruned vote collectors")
}
