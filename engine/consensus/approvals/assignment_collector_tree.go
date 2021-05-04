package approvals

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/forest"
	"sync"
)

type assignmentCollectorVertex = AssignmentCollector

/* Methods implementing LevelledForest's Vertex interface */

func (rsr *assignmentCollectorVertex) VertexID() flow.Identifier { return rsr.ResultID }
func (rsr *assignmentCollectorVertex) Level() uint64             { return rsr.BlockHeight }
func (rsr *assignmentCollectorVertex) Parent() (flow.Identifier, uint64) {
	return rsr.result.PreviousResultID, rsr.BlockHeight - 1
}

type NewCollectorFactoryMethod = func(result *flow.ExecutionResult) (*AssignmentCollector, error)

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

func (t *AssignmentCollectorTree) GetCollector(resultID flow.Identifier) *AssignmentCollector {
	t.lock.RLock()
	defer t.lock.RUnlock()
	vertex, found := t.forest.GetVertex(resultID)
	if !found {
		return nil
	}
	return vertex.(*assignmentCollectorVertex)
}

// GetOrCreateCollector performs lazy initialization of AssignmentCollector using double checked locking
// Returns, (AssignmentCollector, true or false whenever it was created, error)
func (t *AssignmentCollectorTree) GetOrCreateCollector(result *flow.ExecutionResult) (*AssignmentCollector, bool, error) {
	resultID := result.ID()
	// first let's check if we have a collector already
	collector := t.GetCollector(resultID)
	if collector != nil {
		return collector, false, nil
	}

	// fast check shows that there is no collector, need to create one
	t.lock.Lock()
	defer t.lock.Unlock()

	// we need to check again, since it's possible that after checking for existing collector but before taking a lock
	// new collector was created by concurrent goroutine
	vertex, found := t.forest.GetVertex(resultID)
	if found {
		return vertex.(*assignmentCollectorVertex), false, nil
	}

	collector, err := t.onCreateCollector(result)
	if err != nil {
		return nil, false, fmt.Errorf("could not create assignment collector for %v: %w", resultID, err)
	}

	err = t.forest.VerifyVertex(collector)
	if err != nil {
		return nil, false, fmt.Errorf("failed to store assignment collector into the tree: %w", err)
	}

	t.forest.AddVertex(collector)
	return collector, true, nil
}

func (t *AssignmentCollectorTree) PruneUpToHeight(limit uint64) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	err := t.forest.PruneUpToLevel(limit)
	if err != nil {
		return fmt.Errorf("pruning Levelled Forest up to height (aka level) %d failed: %w", limit, err)
	}
	return nil
}
