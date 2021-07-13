package approvals

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/forest"
	"github.com/onflow/flow-go/storage"
)

// assignmentCollectorVertex is a helper structure that implements a LevelledForrest Vertex interface and encapsulates
// AssignmentCollector and information if collector is processable or not
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
// descending from the latest finalized block are relevant.
// Safe for concurrent access. Internally, the mempool utilizes the LevelledForrest.
type AssignmentCollectorTree struct {
	forest              *forest.LevelledForest
	lock                sync.RWMutex
	createCollector     NewCollectorFactoryMethod
	size                uint64
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
		size:                0,
		lastSealedID:        lastSealed.ID(),
		lastFinalizedHeight: lastSealed.Height,
		lastSealedHeight:    lastSealed.Height,
		headers:             headers,
	}
}

func (t *AssignmentCollectorTree) GetSize() uint64 {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.size
}

// GetCollector returns collector by ID and whether it is processable or not
func (t *AssignmentCollectorTree) GetCollector(resultID flow.Identifier) AssignmentCollectorState {
	t.lock.RLock()
	defer t.lock.RUnlock()
	vertex, found := t.forest.GetVertex(resultID)
	if !found {
		return nil
	}

	v := vertex.(*assignmentCollectorVertex)
	return v.collector
}

// FinalizeForkAtLevel performs finalization of fork which is stored in leveled forest. When block is finalized we
// can mark other forks as orphan and stop processing approvals for it. Eventually all forks will be cleaned up by height
func (t *AssignmentCollectorTree) FinalizeForkAtLevel(finalized *flow.Header, sealed *flow.Header) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.lastFinalizedHeight >= finalized.Height {
		return nil
	}

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
	// CONTEXT: as blocks are incorporated into chain they are picked up by sealing.Core and added to AssignmentCollectorTree
	// by definition all blocks should be reported to sealing.Core and that's why all results should be saved in AssignmentCollectorTree.
	// When finalization kicks in we must have a finalized processable fork of assignment collectors.
	// Next section checks if we indeed have a finalized fork, starting from last finalized seal. By definition it has to be
	// processable. If it's not then we have a critical bug which results in blocks being missed by sealing.Core.
	// TODO: remove this at some point when this logic matures.
	if t.lastSealedHeight < sealed.Height {
		finalizedFork, err := t.selectFinalizedFork(sealed.Height+1, finalized.Height)
		if err != nil {
			return fmt.Errorf("could not select finalized fork: %w", err)
		}

		if len(finalizedFork) > 0 {
			if finalizedFork[0].collector.ProcessingStatus() != VerifyingApprovals {
				log.Error().Msgf("AssignmentCollectorTree has found not processable finalized fork %v,"+
					" this is unexpected and shouldn't happen, recovering", finalizedFork[0].collector.BlockID())
				for _, vertex := range finalizedFork {
					expectedStatus := vertex.collector.ProcessingStatus()
					err = vertex.collector.ChangeProcessingStatus(expectedStatus, VerifyingApprovals)
					if err != nil {
						return err
					}
				}
				err = t.updateForkState(finalizedFork[len(finalizedFork)-1], VerifyingApprovals)
				if err != nil {
					return err
				}
			}
		}

		t.lastSealedHeight = sealed.Height
	}

	return nil
}

// selectFinalizedFork traverses chain of collectors starting from some height and picks every collector which executed
// block was finalized
func (t *AssignmentCollectorTree) selectFinalizedFork(startHeight, finalizedHeight uint64) ([]*assignmentCollectorVertex, error) {
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

// updateForkState takes starting vertex of some fork and marks it as processable in recursive manner
func (t *AssignmentCollectorTree) updateForkState(vertex *assignmentCollectorVertex, newState ProcessingStatus) error {
	expectedState := vertex.collector.ProcessingStatus()
	err := vertex.collector.ChangeProcessingStatus(expectedState, newState)
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

// GetCollectorsByInterval returns processable collectors that satisfy interval [from; to)
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
	Collector AssignmentCollectorState
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
		return nil, fmt.Errorf("could not create assignment collector for %v: %w", resultID, err)
	}
	vertex := &assignmentCollectorVertex{
		collector: collector,
	}

	executedBlock, err := t.headers.ByBlockID(result.BlockID)
	if err != nil {
		return nil, fmt.Errorf("could not fetch executed block %v: %w", result.BlockID, err)
	}

	// Initial check showed that there was no collector. However, it's possible that after the
	// initial check but before acquiring the lock to add the newly-created collector, another
	// goroutine already added the needed collector. Hence we need to check again:
	t.lock.Lock()
	defer t.lock.Unlock()
	v, found := t.forest.GetVertex(resultID)
	if found {
		return &LazyInitCollector{
			Collector: v.(*assignmentCollectorVertex).collector,
			Created:   false,
		}, nil
	}

	// An assignment collector is processable if and only if:
	// either (i) the parent result is the latest sealed result (seal is finalized)
	//    or (ii) the result's parent is processable
	parent, parentFound := t.forest.GetVertex(result.PreviousResultID)
	newStatus := CachingApprovals
	if parentFound {
		newStatus = parent.(*assignmentCollectorVertex).collector.ProcessingStatus()
	} else if executedBlock.ParentID == t.lastSealedID {
		newStatus = VerifyingApprovals
	}

	err = t.forest.VerifyVertex(vertex)
	if err != nil {
		return nil, fmt.Errorf("failed to store assignment collector into the tree: %w", err)
	}

	t.forest.AddVertex(vertex)
	t.size += 1
	err = t.updateForkState(vertex, newStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to update fork state: %w", err)
	}
	return &LazyInitCollector{
		Collector: vertex.collector,
		Created:   true,
	}, nil
}

// PruneUpToHeight prunes all results for all assignment collectors with height up to but
// NOT INCLUDING `limit`. Noop, if limit is lower than the previous value (caution:
// this is different than the levelled forest's convention).
// Returns list of resultIDs that were pruned
func (t *AssignmentCollectorTree) PruneUpToHeight(limit uint64) ([]flow.Identifier, error) {
	var pruned []flow.Identifier
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.forest.LowestLevel >= limit {
		return pruned, nil
	}

	if t.size > 0 {
		// collect IDs of vertices that were pruned
		for l := t.forest.LowestLevel; l < limit; l++ {
			iterator := t.forest.GetVerticesAtLevel(l)
			for iterator.HasNext() {
				vertex := iterator.NextVertex()
				pruned = append(pruned, vertex.VertexID())
			}
		}
	}

	// remove vertices and adjust size
	err := t.forest.PruneUpToLevel(limit)
	if err != nil {
		return nil, fmt.Errorf("pruning Levelled Forest up to height (aka level) %d failed: %w", limit, err)
	}
	t.size -= uint64(len(pruned))

	return pruned, nil
}
