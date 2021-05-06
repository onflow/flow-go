package approvals

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/forest"
)

// AssignmentCollectorVertex is a helper structure that implements a LevelledForrest Vertex interface and encapsulates
// AssignmentCollector and if it's orphan or not
type AssignmentCollectorVertex struct {
	Collector *AssignmentCollector
	Orphan    bool
}

/* Methods implementing LevelledForest's Vertex interface */

func (rsr *AssignmentCollectorVertex) VertexID() flow.Identifier { return rsr.Collector.ResultID }
func (rsr *AssignmentCollectorVertex) Level() uint64             { return rsr.Collector.BlockHeight }
func (rsr *AssignmentCollectorVertex) Parent() (flow.Identifier, uint64) {
	return rsr.Collector.result.PreviousResultID, rsr.Collector.BlockHeight - 1
}

type NewCollectorFactoryMethod = func(result *flow.ExecutionResult) (*AssignmentCollector, error)

// AssignmentCollectorTree is a mempool holding assignment collectors, which is aware of the tree structure
// formed by the execution results. The mempool supports pruning by height: only collectors
// descending from the latest finalized block are relevant.
// Safe for concurrent access. Internally, the mempool utilizes the LevelledForrest.
type AssignmentCollectorTree struct {
	forest            *forest.LevelledForest
	lock              sync.RWMutex
	onCreateCollector NewCollectorFactoryMethod
}

func NewAssignmentCollectorTree(onCreateCollector NewCollectorFactoryMethod) *AssignmentCollectorTree {
	return &AssignmentCollectorTree{
		forest:            forest.NewLevelledForest(),
		lock:              sync.RWMutex{},
		onCreateCollector: onCreateCollector,
	}
}

func (t *AssignmentCollectorTree) GetSize() uint {
	return t.forest.GetSize()
}

func (t *AssignmentCollectorTree) GetCollector(resultID flow.Identifier) *AssignmentCollectorVertex {
	t.lock.RLock()
	defer t.lock.RUnlock()
	vertex, found := t.forest.GetVertex(resultID)
	if !found {
		return nil
	}
	return vertex.(*AssignmentCollectorVertex)
}

// FinalizeForkAtLevel performs finalization of fork which is stored in leveled forest. When block is finalized we
// can mark other forks as orphan and stop processing approvals for it. Eventually all forks will be cleaned up by height
func (t *AssignmentCollectorTree) FinalizeForkAtLevel(level uint64, finalizedBlockID flow.Identifier) {
	t.lock.Lock()
	defer t.lock.Unlock()
	iter := t.forest.GetVerticesAtLevel(level)
	for iter.HasNext() {
		vertex := iter.NextVertex().(*AssignmentCollectorVertex)
		if finalizedBlockID != vertex.Collector.BlockID() {
			t.markOrphanFork(vertex)
		}
	}
}

// markOrphanFork takes starting vertex of some fork and marks it as orphan in recursive manner
func (t *AssignmentCollectorTree) markOrphanFork(vertex *AssignmentCollectorVertex) {
	vertex.Orphan = true
	iter := t.forest.GetChildren(vertex.VertexID())
	for iter.HasNext() {
		t.markOrphanFork(iter.NextVertex().(*AssignmentCollectorVertex))
	}
}

// GetCollectorsUpToLevel returns all collectors that satisfy interval [from; to)
func (t *AssignmentCollectorTree) GetCollectorsByInterval(from, to uint64) []*AssignmentCollectorVertex {
	var vertices []*AssignmentCollectorVertex
	t.lock.RLock()
	defer t.lock.RUnlock()

	if from < t.forest.LowestLevel {
		from = t.forest.LowestLevel
	}

	for l := from; l < to; l++ {
		iter := t.forest.GetVerticesAtLevel(l)
		for iter.HasNext() {
			vertices = append(vertices, iter.NextVertex().(*AssignmentCollectorVertex))
		}
	}

	return vertices
}

// GetOrCreateCollector performs lazy initialization of AssignmentCollector using double checked locking
// Returns, (AssignmentCollector, true or false whenever it was created, error)
func (t *AssignmentCollectorTree) GetOrCreateCollector(result *flow.ExecutionResult) (*AssignmentCollector, bool, error) {
	resultID := result.ID()
	// first let's check if we have a collector already
	cachedCollector := t.GetCollector(resultID)
	if cachedCollector != nil {
		return cachedCollector.Collector, false, nil
	}

	// fast check shows that there is no collector, need to create one
	t.lock.Lock()
	defer t.lock.Unlock()

	// we need to check again, since it's possible that after checking for existing collector but before taking a lock
	// new collector was created by concurrent goroutine
	vertex, found := t.forest.GetVertex(resultID)
	if found {
		return vertex.(*AssignmentCollectorVertex).Collector, false, nil
	}

	collector, err := t.onCreateCollector(result)
	if err != nil {
		return nil, false, fmt.Errorf("could not create assignment collector for %v: %w", resultID, err)
	}

	vertex = &AssignmentCollectorVertex{
		Collector: collector,
		Orphan:    false,
	}

	err = t.forest.VerifyVertex(vertex)
	if err != nil {
		return nil, false, fmt.Errorf("failed to store assignment collector into the tree: %w", err)
	}

	t.forest.AddVertex(vertex)
	return collector, true, nil
}

// PruneUpToHeight prunes all results for all assignment collectors with height up to but
// NOT INCLUDING `limit`. Errors if limit is lower than
// the previous value (as we cannot recover previously pruned results).
func (t *AssignmentCollectorTree) PruneUpToHeight(limit uint64) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	err := t.forest.PruneUpToLevel(limit)
	if err != nil {
		return fmt.Errorf("pruning Levelled Forest up to height (aka level) %d failed: %w", limit, err)
	}
	return nil
}
