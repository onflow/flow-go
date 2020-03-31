// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Approvals implements the result approvals memory pool of the consensus nodes,
// used to store result approvals and to generate block seals.
type Approvals struct {
	sync.Mutex
	*Backend
	byResult map[flow.Identifier](map[flow.Identifier]struct{})
	byBlock map[flow.Identifier](map[flow.Identifier]struct{})
}

// NewApprovals creates a new memory pool for result approvals.
func NewApprovals(limit uint) (*Approvals, error) {

	// initialize the approval memory pool with the lookups
	a := &Approvals{
		byResult: make(map[flow.Identifier](map[flow.Identifier]struct{})),
		byBlock: make(map[flow.Identifier](map[flow.Identifier]struct{})),
	}

	// create a hook that will clean up lookups on removal of an entity
	eject := func(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
		entityID, entity := EjectTrueRandom(entities)
		approval := entity.(*flow.ResultApproval)
		a.cleanup(entityID, approval)
		return entityID, entity
	}

	// create the backend with the desired eject function
	a.Backend = NewBackend(
		WithLimit(limit),
		WithEject(eject),
	)

	return a, nil
}

// Add adds an result approval to the mempool.
func (a *Approvals) Add(approval *flow.ResultApproval) error {

	// we need to register all of our lookups first, so that the hook triggered
	// when ejecting entities can properly remove them in case it selects the
	// just added entity
	a.register(approval)

	// then, we can add the entity to the backend
	err := a.Backend.Add(approval)
	if err != nil {
		return err
	}
	return nil
}

// Rem will remove a receipt by ID.
func (a *Approvals) Rem(approvalID flow.Identifier) bool {

	// we need the approval to do full cleanup of lookups
	entity, err := a.Backend.ByID(approvalID)
	if err != nil {
		return false
	}

	// remove the approval from the backend
	a.Backend.Rem(approvalID)

	// clean up the lookups related to this approval
	a.cleanup(approvalID, entity.(*flow.ResultApproval))

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

// register will add the lookup entries for an approval.
func (a *Approvals) register(approval *flow.ResultApproval) {
	a.Lock()
	defer a.Unlock()
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
}

// cleanup will clean up the lookup maps after remaving a result approval.
func (a *Approvals) cleanup(approvalID flow.Identifier, approval *flow.ResultApproval) {
	a.Lock()
	defer a.Unlock()
	resultID := approval.ResultApprovalBody.ExecutionResultID
	forResult := a.byResult[resultID]
	delete(forResult, approvalID)
	if len(forResult) > 0 {
		return
	}
	delete(a.byResult, resultID)
	blockID := approval.ResultApprovalBody.BlockID
	forBlock := a.byBlock[blockID]
	delete(forBlock, resultID)
	if len(forBlock) > 0 {
		return
	}
	delete(a.byBlock, blockID)
}
