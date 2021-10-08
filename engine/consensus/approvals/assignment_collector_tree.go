package approvals

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/forest"
	"github.com/onflow/flow-go/storage"
)

// assignmentCollectorVertex is a helper structure that wraps an AssignmentCollector
// so it implements the LevelledForest's `Vertex` interface:
//  * VertexID is defined as the ID of the execution result
//  * Level is defined as the height of the executed block
type assignmentCollectorVertex struct {
	collector AssignmentCollector
}

/* Methods implementing LevelledForest's Vertex interface */

func (v *assignmentCollectorVertex) VertexID() flow.Identifier { return v.collector.ResultID() }
func (v *assignmentCollectorVertex) Level() uint64             { return v.collector.Block().Height }
func (v *assignmentCollectorVertex) Parent() (flow.Identifier, uint64) {
	return v.collector.Result().PreviousResultID, v.collector.Block().Height - 1
}

// NewCollectorFactoryMethod is a factory method to generate an AssignmentCollector for an execution result
type NewCollectorFactoryMethod = func(result *flow.ExecutionResult) (AssignmentCollector, error)

// AssignmentCollectorTree is a mempool holding assignment collectors, which is aware of the tree structure
// formed by the execution results. The mempool supports pruning by height: only collectors
// descending from the latest sealed and finalized result are relevant.
// Safe for concurrent access. Internally, the mempool utilizes the LevelledForest.
type AssignmentCollectorTree struct {
	forest              *forest.LevelledForest
	lock                sync.RWMutex
	createCollector     NewCollectorFactoryMethod
	lastSealedID        flow.Identifier
	lastSealedHeight    uint64
	lastFinalizedHeight uint64
	headers             storage.Headers
}

func NewAssignmentCollectorTree(lastSealed *flow.Header, headers storage.Headers, createCollector NewCollectorFactoryMethod) *AssignmentCollectorTree {
	return &AssignmentCollectorTree{
		forest:              forest.NewLevelledForest(lastSealed.Height),
		lock:                sync.RWMutex{},
		createCollector:     createCollector,
		lastSealedID:        lastSealed.ID(),
		lastFinalizedHeight: lastSealed.Height,
		lastSealedHeight:    lastSealed.Height,
		headers:             headers,
	}
}

func (t *AssignmentCollectorTree) GetSize() uint64 {
	t.lock.RLock()
	defer t.lock.RUnlock()
	//locking is still needed, since forest.GetSize is not concurrent safe.
	return t.forest.GetSize()
}

// GetCollector returns assignment collector for the given result.
func (t *AssignmentCollectorTree) GetCollector(resultID flow.Identifier) AssignmentCollector {
	t.lock.RLock()
	defer t.lock.RUnlock()
	vertex, found := t.forest.GetVertex(resultID)
	if !found {
		return nil
	}

	v := vertex.(*assignmentCollectorVertex)
	return v.collector
}

// FinalizeForkAtLevel orphans forks in the AssignmentCollectorTree and prunes levels below the
// sealed finalized height. When a block is finalized we can mark results for conflicting forks as
// orphaned and stop processing approvals for them. Eventually all forks will be cleaned up by height.
func (t *AssignmentCollectorTree) FinalizeForkAtLevel(finalized *flow.Header, sealed *flow.Header) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.lastFinalizedHeight >= finalized.Height {
		return nil
	}

	// STEP 1: orphan forks in the AssignmentCollectorTree whose results are
	// for blocks that are conflicting with the finalized blocks
	t.lastSealedID = sealed.ID()
	for height := finalized.Height; height > t.lastFinalizedHeight; height-- {
		finalizedBlock, err := t.headers.ByHeight(height)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized block at height %d: %w", height, err)
		}
		finalizedBlockID := finalizedBlock.ID()
		iter := t.forest.GetVerticesAtLevel(height)
		for iter.HasNext() {
			vertex := iter.NextVertex().(*assignmentCollectorVertex)
			if finalizedBlockID != vertex.collector.BlockID() {
				err = t.updateForkState(vertex, Orphaned)
				if err != nil {
					return err
				}
			}
		}
	}

	t.lastFinalizedHeight = finalized.Height

	// WARNING: next block of code implements a special fallback mechanism to recover from sealing halt.
	// CONTEXT: As blocks are incorporated into chain they are picked up by sealing.Core and added to AssignmentCollectorTree.
	// By definition, all blocks should be reported to sealing.Core and that's why all results should be saved in AssignmentCollectorTree.
	// When finalization kicks in, we must have a finalized processable fork of assignment collectors.
	// Next section checks if we indeed have a finalized fork, starting from last finalized seal. By definition it has to be
	// processable. If it's not then we have a critical bug which results in blocks being missed by sealing.Core.
	// TODO: remove this at some point when this logic matures.
	if t.lastSealedHeight < sealed.Height {
		collectors, err := t.selectCollectorsForFinalizedFork(sealed.Height+1, finalized.Height)
		if err != nil {
			return fmt.Errorf("could not select finalized fork: %w", err)
		}

		for _, collectorVertex := range collectors {
			clr := collectorVertex.collector
			if clr.ProcessingStatus() != VerifyingApprovals {
				log.Error().Msgf("AssignmentCollectorTree has found not processable finalized fork %v,"+
					" this is unexpected and shouldn't happen, recovering", clr.BlockID())
			}
			currentStatus := clr.ProcessingStatus()
			if clr.Block().Height < finalized.Height {
				err = clr.ChangeProcessingStatus(currentStatus, VerifyingApprovals)
			} else {
				err = t.updateForkState(collectorVertex, VerifyingApprovals)
			}
			if err != nil {
				return err
			}
		}

		t.lastSealedHeight = sealed.Height
	}

	// STEP 2: prune levels below the latest sealed finalized height.
	err := t.pruneUpToHeight(sealed.Height)
	if err != nil {
		return fmt.Errorf("could not prune collectors tree up to height %d: %w", sealed.Height, err)
	}

	return nil
}

// selectCollectorsForFinalizedFork collects all collectors for blocks with height
// in [startHeight, finalizedHeight], whose block is finalized.
// NOT concurrency safe.
func (t *AssignmentCollectorTree) selectCollectorsForFinalizedFork(startHeight, finalizedHeight uint64) ([]*assignmentCollectorVertex, error) {
	var fork []*assignmentCollectorVertex
	for height := startHeight; height <= finalizedHeight; height++ {
		iter := t.forest.GetVerticesAtLevel(height)
		finalizedBlock, err := t.headers.ByHeight(height)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve finalized block at height %d: %w", height, err)
		}
		finalizedBlockID := finalizedBlock.ID()
		for iter.HasNext() {
			vertex := iter.NextVertex().(*assignmentCollectorVertex)
			if finalizedBlockID == vertex.collector.BlockID() {
				fork = append(fork, vertex)
				break
			}
		}
	}
	return fork, nil
}

// updateForkState changes the state of `vertex` and all its descendants to `newState`.
// NOT concurrency safe.
func (t *AssignmentCollectorTree) updateForkState(vertex *assignmentCollectorVertex, newState ProcessingStatus) error {
	currentStatus := vertex.collector.ProcessingStatus()
	if currentStatus == newState {
		return nil
	}
	err := vertex.collector.ChangeProcessingStatus(currentStatus, newState)
	if err != nil {
		return err
	}

	iter := t.forest.GetChildren(vertex.VertexID())
	for iter.HasNext() {
		err := t.updateForkState(iter.NextVertex().(*assignmentCollectorVertex), newState)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetCollectorsByInterval returns all collectors in state `VerifyingApprovals`
// whose executed block has height in [from; to)
func (t *AssignmentCollectorTree) GetCollectorsByInterval(from, to uint64) []AssignmentCollector {
	var vertices []AssignmentCollector
	t.lock.RLock()
	defer t.lock.RUnlock()

	if from < t.forest.LowestLevel {
		from = t.forest.LowestLevel
	}

	for l := from; l < to; l++ {
		iter := t.forest.GetVerticesAtLevel(l)
		for iter.HasNext() {
			vertex := iter.NextVertex().(*assignmentCollectorVertex)
			if vertex.collector.ProcessingStatus() == VerifyingApprovals {
				vertices = append(vertices, vertex.collector)
			}
		}
	}

	return vertices
}

// LazyInitCollector is a helper structure that is used to return collector which is lazy initialized
type LazyInitCollector struct {
	Collector AssignmentCollector
	Created   bool // whether collector was created or retrieved from cache
}

// GetOrCreateCollector performs lazy initialization of AssignmentCollector using double-checked locking.
func (t *AssignmentCollectorTree) GetOrCreateCollector(result *flow.ExecutionResult) (*LazyInitCollector, error) {
	resultID := result.ID()
	// first let's check if we have a collector already
	cachedCollector := t.GetCollector(resultID)
	if cachedCollector != nil {
		return &LazyInitCollector{
			Collector: cachedCollector,
			Created:   false,
		}, nil
	}

	collector, err := t.createCollector(result)
	if err != nil {
		return nil, fmt.Errorf("could not create assignment collector for result %v: %w", resultID, err)
	}
	vertex := &assignmentCollectorVertex{
		collector: collector,
	}

	// Initial check showed that there was no collector. However, it's possible that after the
	// initial check but before acquiring the lock to add the newly-created collector, another
	// goroutine already added the needed collector. Hence, check again after acquiring the lock:
	t.lock.Lock()
	defer t.lock.Unlock()

	// leveled forest doesn't treat this case as error, we shouldn't create collectors
	// for vertices lower that forest.LowestLevel
	if vertex.Level() < t.forest.LowestLevel {
		return nil, engine.NewOutdatedInputErrorf("cannot add collector because its height %d is smaller than the lowest height %d", vertex.Level(), t.forest.LowestLevel)
	}

	v, found := t.forest.GetVertex(resultID)
	if found {
		return &LazyInitCollector{
			Collector: v.(*assignmentCollectorVertex).collector,
			Created:   false,
		}, nil
	}

	// add AssignmentCollector as vertex to tree
	err = t.forest.VerifyVertex(vertex)
	if err != nil {
		return nil, fmt.Errorf("failed to store assignment collector into the tree: %w", err)
	}

	t.forest.AddVertex(vertex)

	// An assignment collector is processable if and only if:
	// either (i) the parent result is the latest sealed result (seal is finalized)
	//    or (ii) the result's parent is processable
	parent, parentFound := t.forest.GetVertex(result.PreviousResultID)
	newStatus := CachingApprovals
	if parentFound {
		newStatus = parent.(*assignmentCollectorVertex).collector.ProcessingStatus()
	}
	if collector.Block().ParentID == t.lastSealedID {
		newStatus = VerifyingApprovals
	}
	err = t.updateForkState(vertex, newStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to update fork state: %w", err)
	}

	return &LazyInitCollector{
		Collector: vertex.collector,
		Created:   true,
	}, nil
}

// pruneUpToHeight prunes all assignment collectors for results with height up to but
// NOT INCLUDING `limit`. Noop, if limit is lower than the previous value (caution:
// this is different from the levelled forest's convention).
// This function is NOT concurrency safe.
func (t *AssignmentCollectorTree) pruneUpToHeight(limit uint64) error {
	if t.forest.LowestLevel >= limit {
		return nil
	}

	// remove vertices and adjust size
	err := t.forest.PruneUpToLevel(limit)
	if err != nil {
		return fmt.Errorf("pruning Levelled Forest up to height (aka level) %d failed: %w", limit, err)
	}

	return nil
}
