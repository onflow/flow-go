// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
)

// Approvals implements the result approvals memory pool of the consensus nodes,
// used to store result approvals and to generate block seals.
type Approvals struct {
	*Backend
	byResult map[flow.Identifier]flow.Identifier
}

// NewApprovals creates a new memory pool for result approvals.
func NewApprovals() (*Approvals, error) {
	a := &Approvals{
		Backend:  NewBackend(),
		byResult: make(map[flow.Identifier]flow.Identifier),
	}

	return a, nil
}

// Add adds an result approval to the mempool.
func (a *Approvals) Add(approval *flow.ResultApproval) error {
	err := a.Backend.Add(approval)
	if err != nil {
		return err
	}
	a.byResult[approval.ResultApprovalBody.ExecutionResultID] = approval.ID()
	return nil
}

// Rem removes a result approval from the mempool.
func (a *Approvals) Rem(approvalID flow.Identifier) bool {
	approval, err := a.ByID(approvalID)
	if err != nil {
		return false
	}
	ok := a.Backend.Rem(approvalID)
	if !ok {
		return false
	}
	delete(a.byResult, approval.ResultApprovalBody.ExecutionResultID)
	return true
}

// ByID returns the result approval with the given ID from the mempool.
func (a *Approvals) ByID(approvalID flow.Identifier) (*flow.ResultApproval, error) {
	entity, err := a.Backend.ByID(approvalID)
	if err != nil {
		return nil, err
	}
	approval, ok := entity.(*flow.ResultApproval)
	if !ok {
		panic(fmt.Sprintf("invalid entity in approval pool (%T)", entity))
	}
	return approval, nil
}

// ByResultID returns an approval by receipt ID.
func (a *Approvals) ByResultID(resultID flow.Identifier) (*flow.ResultApproval, error) {
	approvalID, ok := a.byResult[resultID]
	if !ok {
		return nil, mempool.ErrEntityNotFound
	}
	return a.ByID(approvalID)
}

// All returns all result approvals from the pool.
func (a *Approvals) All() []*flow.ResultApproval {
	entities := a.Backend.All()
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
