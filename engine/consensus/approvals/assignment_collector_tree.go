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

func (v *AssignmentCollectorVertex) VertexID() flow.Identifier { return v.Collector.ResultID }
func (v *AssignmentCollectorVertex) Level() uint64             { return v.Collector.BlockHeight }
func (v *AssignmentCollectorVertex) Parent() (flow.Identifier, uint64) {
	return v.Collector.result.PreviousResultID, v.Collector.BlockHeight - 1
}

// NewCollector is a factory method to generate an AssignmentCollector for an execution result
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
	// if parent is orphan then all children should be orphan as well
	// we can skip marking the child nodes.
	if vertex.Orphan {
		return
	}

	vertex.Orphan = true
	iter := t.forest.GetChildren(vertex.VertexID())
	for iter.HasNext() {
		t.markOrphanFork(iter.NextVertex().(*AssignmentCollectorVertex))
	}
}

// GetCollectorsByInterval returns all collectors that satisfy interval [from; to)
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

	collector, err := t.onCreateCollector(result)
	if err != nil {
		return nil, false, fmt.Errorf("could not create assignment collector for %v: %w", resultID, err)
	}
	vertex := &AssignmentCollectorVertex{
		Collector: collector,
		Orphan:    false,
	}

	// fast check shows that there is no collector, need to create one
	t.lock.Lock()
	defer t.lock.Unlock()

	// we need to check again, since it's possible that after checking for existing collector but before taking a lock
	// new collector was created by concurrent goroutine
	v, found := t.forest.GetVertex(resultID)
	if found {
		return v.(*AssignmentCollectorVertex).Collector, false, nil
	}
	parent, parentFound := t.forest.GetVertex(result.PreviousResultID)
	if parentFound {
		vertex.Orphan = parent.(*AssignmentCollectorVertex).Orphan
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
// Returns list of resultIDs that were pruned
func (t *AssignmentCollectorTree) PruneUpToHeight(limit uint64) ([]flow.Identifier, error) {
	var pruned []flow.Identifier
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.forest.GetSize() > 0 {
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
	return pruned, nil
}
