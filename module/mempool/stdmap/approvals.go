// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Approvals implements the result approvals memory pool of the consensus nodes,
// used to store result approvals and to generate block seals.
type Approvals struct {
	*Backend
	byResult map[flow.Identifier](map[flow.Identifier]struct{})
	byBlock  map[flow.Identifier](map[flow.Identifier]struct{})
}

// NewApprovals creates a new memory pool for result approvals.
func NewApprovals() (*Approvals, error) {
	a := &Approvals{
		Backend:  NewBackend(),
		byResult: make(map[flow.Identifier](map[flow.Identifier]struct{})),
		byBlock:  make(map[flow.Identifier](map[flow.Identifier]struct{})),
	}

	return a, nil
}

// Add adds an result approval to the mempool.
func (a *Approvals) Add(approval *flow.ResultApproval) error {
	err := a.Backend.Add(approval)
	if err != nil {
		return err
	}
	resultID := approval.ResultApprovalBody.ExecutionResultID
	forResult, hasResult := a.byResult[resultID]
	if !hasResult {
		forResult = make(map[flow.Identifier]struct{})
		a.byResult[resultID] = forResult
	}
	blockID := approval.ResultApprovalBody.BlockID
	forBlock, hasBlock := a.byBlock[blockID]
	if !hasBlock {
		forBlock = make(map[flow.Identifier]struct{})
		a.byBlock[blockID] = forBlock
	}
	approvalID := approval.ID()
	forBlock[resultID] = struct{}{}
	forResult[approvalID] = struct{}{}
	return nil
}

// Rem will remove a receipt by ID.
func (a *Approvals) Rem(approvalID flow.Identifier) bool {
	entity, err := a.Backend.ByID(approvalID)
	if err != nil {
		return false
	}
	_ = a.Backend.Rem(approvalID)
	approval := entity.(*flow.ResultApproval)
	resultID := approval.ResultApprovalBody.ExecutionResultID
	forResult := a.byResult[resultID]
	delete(forResult, approvalID)
	if len(forResult) > 0 {
		return true
	}
	delete(a.byResult, resultID)
	blockID := approval.ResultApprovalBody.BlockID
	forBlock := a.byBlock[blockID]
	delete(forBlock, resultID)
	if len(forBlock) > 0 {
		return true
	}
	delete(a.byBlock, blockID)
	return true
}

// ByResultID returns an approval by receipt ID.
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

// DropForBlock drops all execution receipts for the given block.
func (a *Approvals) DropForBlock(blockID flow.Identifier) {
	forBlock, hasBlock := a.byBlock[blockID]
	if !hasBlock {
		return
	}
	for resultID := range forBlock {
		forResult, hasResult := a.byResult[resultID]
		if !hasResult {
			return
		}
		for approvalID := range forResult {
			_ = a.Backend.Rem(approvalID)
		}
		delete(a.byResult, resultID)
	}
	delete(a.byBlock, blockID)
}
