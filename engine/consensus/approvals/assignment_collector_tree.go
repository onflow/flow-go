package approvals

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/forest"
	"github.com/onflow/flow-go/storage"
)

// assignmentCollectorVertex is a helper structure that implements a LevelledForrest Vertex interface and encapsulates
// AssignmentCollector and information if collector is processable or not
type assignmentCollectorVertex struct {
	collector   *AssignmentCollector
	processable bool
}

/* Methods implementing LevelledForest's Vertex interface */

func (v *assignmentCollectorVertex) VertexID() flow.Identifier { return v.collector.ResultID }
func (v *assignmentCollectorVertex) Level() uint64             { return v.collector.BlockHeight }
func (v *assignmentCollectorVertex) Parent() (flow.Identifier, uint64) {
	return v.collector.result.PreviousResultID, v.collector.BlockHeight - 1
}

// NewCollector is a factory method to generate an AssignmentCollector for an execution result
type NewCollectorFactoryMethod = func(result *flow.ExecutionResult) (*AssignmentCollector, error)

// AssignmentCollectorTree is a mempool holding assignment collectors, which is aware of the tree structure
// formed by the execution results. The mempool supports pruning by height: only collectors
// descending from the latest finalized block are relevant.
// Safe for concurrent access. Internally, the mempool utilizes the LevelledForrest.
type AssignmentCollectorTree struct {
	forest              *forest.LevelledForest
	lock                sync.RWMutex
	onCreateCollector   NewCollectorFactoryMethod
	size                uint64
	lastSealedID        flow.Identifier
	lastFinalizedHeight uint64
	headers             storage.Headers
}

func NewAssignmentCollectorTree(lastSealed *flow.Header, headers storage.Headers, onCreateCollector NewCollectorFactoryMethod) *AssignmentCollectorTree {
	return &AssignmentCollectorTree{
		forest:            forest.NewLevelledForest(lastSealed.Height),
		lock:              sync.RWMutex{},
		onCreateCollector: onCreateCollector,
		size:              0,
		lastSealedID:      lastSealed.ID(),
		headers:           headers,
	}
}

// GetCollector returns collector by ID and whether it is processable or not
func (t *AssignmentCollectorTree) GetCollector(resultID flow.Identifier) (*AssignmentCollector, bool) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	vertex, found := t.forest.GetVertex(resultID)
	if !found {
		return nil, false
	}

	v := vertex.(*assignmentCollectorVertex)
	return v.collector, v.processable
}

// FinalizeForkAtLevel performs finalization of fork which is stored in leveled forest. When block is finalized we
// can mark other forks as orphan and stop processing approvals for it. Eventually all forks will be cleaned up by height
func (t *AssignmentCollectorTree) FinalizeForkAtLevel(finalized *flow.Header, sealed *flow.Header) {
	finalizedBlockID := finalized.ID()
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.lastFinalizedHeight >= finalized.Height {
		return
	}

	t.lastFinalizedHeight = finalized.Height
	t.lastSealedID = sealed.ID()
	iter := t.forest.GetVerticesAtLevel(finalized.Height)
	for iter.HasNext() {
		vertex := iter.NextVertex().(*assignmentCollectorVertex)
		if finalizedBlockID != vertex.collector.BlockID() {
			t.markForkProcessable(vertex, false)
		}
	}
}

// markForkProcessable takes starting vertex of some fork and marks it as processable in recursive manner
func (t *AssignmentCollectorTree) markForkProcessable(vertex *assignmentCollectorVertex, processable bool) {
	vertex.processable = processable
	iter := t.forest.GetChildren(vertex.VertexID())
	for iter.HasNext() {
		t.markForkProcessable(iter.NextVertex().(*assignmentCollectorVertex), processable)
	}
}

// GetCollectorsByInterval returns processable collectors that satisfy interval [from; to)
func (t *AssignmentCollectorTree) GetCollectorsByInterval(from, to uint64) []*AssignmentCollector {
	var vertices []*AssignmentCollector
	t.lock.RLock()
	defer t.lock.RUnlock()

	if from < t.forest.LowestLevel {
		from = t.forest.LowestLevel
	}

	for l := from; l < to; l++ {
		iter := t.forest.GetVerticesAtLevel(l)
		for iter.HasNext() {
			vertex := iter.NextVertex().(*assignmentCollectorVertex)
			if vertex.processable {
				vertices = append(vertices, vertex.collector)
			}
		}
	}

	return vertices
}

// LazyInitCollector is a helper structure that is used to return collector which is lazy initialized
type LazyInitCollector struct {
	Collector   *AssignmentCollector
	Processable bool // whether collector is processable
	Created     bool // whether collector was created or retrieved from cache
}

// GetOrCreateCollector performs lazy initialization of AssignmentCollector using double checked locking
// Returns, (AssignmentCollector, true or false whenever it was created, error)
func (t *AssignmentCollectorTree) GetOrCreateCollector(result *flow.ExecutionResult) (*LazyInitCollector, error) {
	resultID := result.ID()
	// first let's check if we have a collector already
	cachedCollector, processable := t.GetCollector(resultID)
	if cachedCollector != nil {
		return &LazyInitCollector{
			Collector:   cachedCollector,
			Processable: processable,
			Created:     false,
		}, nil
	}

	collector, err := t.onCreateCollector(result)
	if err != nil {
		return nil, fmt.Errorf("could not create assignment collector for %v: %w", resultID, err)
	}
	vertex := &assignmentCollectorVertex{
		collector:   collector,
		processable: false,
	}

	executedBlock, err := t.headers.ByBlockID(result.BlockID)
	if err != nil {
		return nil, fmt.Errorf("could not fetch executed block %v: %w", result.BlockID, err)
	}

	// fast check shows that there is no collector, need to create one
	t.lock.Lock()
	defer t.lock.Unlock()

	// we need to check again, since it's possible that after checking for existing collector but before taking a lock
	// new collector was created by concurrent goroutine
	v, found := t.forest.GetVertex(resultID)
	if found {
		return &LazyInitCollector{
			Collector:   v.(*assignmentCollectorVertex).collector,
			Processable: v.(*assignmentCollectorVertex).processable,
			Created:     false,
		}, nil
	}
	parent, parentFound := t.forest.GetVertex(result.PreviousResultID)
	if parentFound {
		vertex.processable = parent.(*assignmentCollectorVertex).processable
	} else if executedBlock.ParentID == t.lastSealedID {
		vertex.processable = true
	}

	err = t.forest.VerifyVertex(vertex)
	if err != nil {
		return nil, fmt.Errorf("failed to store assignment collector into the tree: %w", err)
	}

	t.forest.AddVertex(vertex)
	t.size += 1
	t.markForkProcessable(vertex, vertex.processable)
	return &LazyInitCollector{
		Collector:   vertex.collector,
		Processable: vertex.processable,
		Created:     true,
	}, nil
}

// PruneUpToHeight prunes all results for all assignment collectors with height up to but
// NOT INCLUDING `limit`. Errors if limit is lower than
// the previous value (as we cannot recover previously pruned results).
// Returns list of resultIDs that were pruned
func (t *AssignmentCollectorTree) PruneUpToHeight(limit uint64) ([]flow.Identifier, error) {
	var pruned []flow.Identifier
	t.lock.Lock()
	defer t.lock.Unlock()

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
