// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Approvals implements the result approvals memory pool of the consensus nodes,
// used to store result approvals and to generate block seals.
type Approvals struct {
	*backend
}

// NewApprovals creates a new memory pool for result approvals.
func NewApprovals() (*Approvals, error) {
	a := &Approvals{
		backend: newBackend(),
	}

	return a, nil
}

// Add adds an result approval to the mempool.
func (a *Approvals) Add(approval *flow.ResultApproval) error {
	return a.backend.Add(approval)
}

// Get returns the result approval with the given ID from the mempool.
func (a *Approvals) Get(approvalID flow.Identifier) (*flow.ResultApproval, error) {
	entity, err := a.backend.Get(approvalID)
	if err != nil {
		return nil, err
	}
	approval, ok := entity.(*flow.ResultApproval)
	if !ok {
		panic(fmt.Sprintf("invalid entity in approval pool (%T)", entity))
	}
	return approval, nil
}

// All returns all result approvals from the pool.
func (a *Approvals) All() []*flow.ResultApproval {
	entities := a.backend.All()
	approvals := make([]*flow.ResultApproval, 0, len(entities))
	for _, entity := range entities {
		approval, ok := entity.(*flow.ResultApproval)
		if !ok {
			panic(fmt.Sprintf("invalid entity in approval pool (%T)", entity))
		}
		approvals = append(approvals, approval)
	}
	return approvals
}
