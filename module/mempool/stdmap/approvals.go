// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"encoding/binary"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/model"
)

// Approvals implements the result approvals memory pool of the consensus nodes,
// used to store result approvals and to generate block seals. Approvals are
// indexed by chunk and approver to facilitate the chunk-matching algorithm.
// The underyling key value store is as follows:
//
// [chunk_key] => ( [approver_id] => *ResultApproval )
//
// where chunk_key is an identifier obtained by combining the approval's result
// ID and chunk index.
type Approvals struct {
	// Concurrency: the mempool internally re-uses the backend's lock

	backend *Backend
	size    uint
}

// key computes the composite key used to index an approval in the backend. It
// hashes the resultID and the chunkIndex together.
func key(resultID flow.Identifier, chunkIndex uint64) flow.Identifier {
	chunkIndexBytes := flow.Identifier{} // null value: zero-filled
	binary.LittleEndian.PutUint64(chunkIndexBytes[:], chunkIndex)
	return flow.ConcatSum(resultID, chunkIndexBytes) // compute composite identifier
}

// NewApprovals creates a new memory pool for result approvals.
func NewApprovals(limit uint, opts ...OptionFunc) (*Approvals, error) {
	mempool := &Approvals{
		size:    0,
		backend: NewBackend(append(opts, WithLimit(limit))...),
	}

	adjustSizeOnEjection := func(entity flow.Entity) {
		// uncaught type assertion; should never panic as the mempool only stores ApprovalMapEntity:
		approvalMapEntity := entity.(*model.ApprovalMapEntity)
		mempool.size -= uint(len(approvalMapEntity.Approvals))
	}
	err := mempool.backend.RegisterEjectionCallback(adjustSizeOnEjection)
	if err != nil {
		return nil, fmt.Errorf("failed to register ejection callback with Approvals mempool's backend")
	}

	return mempool, nil
}

// Add adds a result approval to the mempool.
func (a *Approvals) Add(approval *flow.ResultApproval) (bool, error) {

	// determine the lookup key for the corresponding chunk
	chunkKey := key(approval.Body.ExecutionResultID, approval.Body.ChunkIndex)

	appended := false
	err := a.backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {

		var chunkApprovals map[flow.Identifier]*flow.ResultApproval

		entity, ok := backdata[chunkKey]
		if !ok {
			// no record with key is available in the mempool, initialise chunkApprovals.
			chunkApprovals = make(map[flow.Identifier]*flow.ResultApproval)
			backdata[chunkKey] = &model.ApprovalMapEntity{
				ChunkKey:   chunkKey,
				ResultID:   approval.Body.ExecutionResultID,
				ChunkIndex: approval.Body.ChunkIndex,
				Approvals:  chunkApprovals,
			}
		} else {
			// uncaught type assertion; should never panic as the mempool only stores ApprovalMapEntity:
			chunkApprovals = entity.(*model.ApprovalMapEntity).Approvals
			if _, ok := chunkApprovals[approval.Body.ApproverID]; ok {
				// approval is already associated with the chunk key and approver => no need to append
				return nil
			}
		}

		// appends approval to the map
		chunkApprovals[approval.Body.ApproverID] = approval
		appended = true
		a.size++
		return nil
	})

	return appended, err
}

// RemApproval removes a specific approval.
func (a *Approvals) RemApproval(approval *flow.ResultApproval) (bool, error) {
	// determine the lookup key for the corresponding chunk
	chunkKey := key(approval.Body.ExecutionResultID, approval.Body.ChunkIndex)

	removed := false
	err := a.backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		entity, ok := backdata[chunkKey]
		if !ok {
			// no approvals for this chunk
			return nil
		}
		// uncaught type assertion; should never panic as the mempool only stores ApprovalMapEntity:
		chunkApprovals := entity.(*model.ApprovalMapEntity).Approvals

		if _, ok := chunkApprovals[approval.Body.ApproverID]; !ok {
			// no approval for this chunk and approver
			return nil
		}
		if len(chunkApprovals) == 1 {
			// special case: there is only a single approval stored for this chunkKey
			// => remove entire map with all approvals for this chunk
			delete(backdata, chunkKey)
		} else {
			// remove item from map
			delete(chunkApprovals, approval.Body.ApproverID)
		}

		removed = true
		a.size--
		return nil
	})

	return removed, err
}

// RemChunk will remove all the approvals corresponding to the chunk.
func (a *Approvals) RemChunk(resultID flow.Identifier, chunkIndex uint64) (bool, error) {
	chunkKey := key(resultID, chunkIndex)

	removed := false
	err := a.backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		entity, exists := backdata[chunkKey]
		if !exists {
			return nil
		}
		// uncaught type assertion; should never panic as the mempool only stores ApprovalMapEntity:
		approvalMapEntity := entity.(*model.ApprovalMapEntity)

		delete(backdata, chunkKey)
		a.size -= uint(len(approvalMapEntity.Approvals))
		removed = true
		return nil
	})

	return removed, err
}

// Get fetches approvals for a specific chunk
func (a *Approvals) ByChunk(resultID flow.Identifier, chunkIndex uint64) map[flow.Identifier]*flow.ResultApproval {
	// determine the lookup key for the corresponding chunk
	chunkKey := key(resultID, chunkIndex)

	// To guarantee concurrency safety, we need to copy the map via a locked operation in the backend.
	// Otherwise, another routine might concurrently modify the map stored for the same resultID.
	approvals := make(map[flow.Identifier]*flow.ResultApproval)
	_ = a.backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		entity, exists := backdata[chunkKey]
		if !exists {
			return nil
		}
		// uncaught type assertion; should never panic as the mempool only stores ApprovalMapEntity:
		for i, app := range entity.(*model.ApprovalMapEntity).Approvals {
			approvals[i] = app
		}
		return nil
	}) // error return impossible

	return approvals
}

// All will return all approvals in the memory pool.
func (a *Approvals) All() []*flow.ResultApproval {
	res := make([]*flow.ResultApproval, 0)

	_ = a.backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		for _, entity := range backdata {
			// uncaught type assertion; should never panic as the mempool only stores ApprovalMapEntity:
			for _, approval := range entity.(*model.ApprovalMapEntity).Approvals {
				res = append(res, approval)
			}
		}
		return nil
	}) // error return impossible

	return res
}

// Size returns the number of approvals in the mempool.
func (a *Approvals) Size() uint {
	// To guarantee concurrency safety, i.e. that the read retrieves the latest size value,
	// we need run utilize the backend's lock.
	a.backend.RLock()
	defer a.backend.RUnlock()
	return a.size
}
