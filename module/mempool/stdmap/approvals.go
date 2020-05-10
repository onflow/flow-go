// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Approvals implements the result approvals memory pool of the consensus nodes,
// used to store result approvals and to generate block seals.
type Approvals struct {
	*Backend
}

// NewApprovals creates a new memory pool for result approvals.
func NewApprovals(limit uint) (*Approvals, error) {

	// initialize the approval memory pool with the lookups
	a := &Approvals{
		Backend: NewBackend(WithLimit(limit)),
	}

	return a, nil
}

// Add adds an result approval to the mempool.
func (a *Approvals) Add(approval *flow.ResultApproval) bool {

	// we need to register all of our lookups first, so that the hook triggered
	// when ejecting entities can properly remove them in case it selects the
	// just added entity
	a.register(approval)

	// then, we can add the entity to the backend
	added := a.Backend.Add(approval)

	return added
}

// Rem will remove a approval by ID.
func (a *Approvals) Rem(approvalID flow.Identifier) bool {
	removed := a.Backend.Rem(approvalID)
	return removed
}

	// we need the approval to do full cleanup of lookups
	entity, removed := a.Backend.ByID(approvalID)
	if !removed {
		return false
	}
	approval := entity.(*flow.ResultApproval)
	return approval, true
}

// ByID will retrieve an approval by ID.
func (a *Approvals) ByID(approvalID flow.Identifier) (*flow.ResultApproval, bool) {
	entity, exists := a.Backend.ByID(approvalID)
	if !exists {
		return nil, false
	}
	approval := entity.(*flow.ResultApproval)
	return approval, true
}

// All will return all execution receipts in the memory pool.
func (a *Approvals) All() []*flow.ResultApproval {
	entities := a.Backend.All()
	approvals := make([]*flow.ResultApproval, 0, len(entities))
	for _, entity := range entities {
		approvals = append(approvals, entity.(*flow.ResultApproval))
	}
	return approvals
}

// ByResultID returns an approval by approval ID.
func (a *Approvals) ByResultID(resultID flow.Identifier) []*flow.ResultApproval {
	forResult, hasResult := a.byResult[resultID]
	if !hasResult {
		return nil
	}
	approvals := make([]*flow.ResultApproval, 0, len(forResult))
	for approvalID := range forResult {
		entity, _ := a.Backend.ByID(approvalID)
		approvals = append(approvals, entity.(*flow.ResultApproval))
	}
	return approvals
}

// DropForResult drops all execution receipts for the given block.
func (a *Approvals) DropForResult(resultID flow.Identifier) []flow.Identifier {
	var approvalIDs []flow.Identifier
	for _, approval := range a.All() {
		if approval.Body.ExecutionResultID == resultID {
			_ = a.Rem(approval.ID())
			approvalIDs = append(approvalIDs, approval.ID())
		}
	}
	return approvalIDs
}
