// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"encoding/binary"
	"fmt"

	"github.com/onflow/flow-go/crypto/hash"
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
	backend *Backend
	size    *uint
}

// key computes the composite key used to index an approval in the backend. It
// hashes the resultID and the chunkIndex together.
func key(resultID flow.Identifier, chunkIndex uint64) flow.Identifier {

	// convert chunkIndex into Identifier
	hasher := hash.NewSHA3_256()
	chunkIndexBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(chunkIndexBytes, chunkIndex)
	chunkIndexHash := hasher.ComputeHash(chunkIndexBytes)
	chunkIndexID := flow.HashToID(chunkIndexHash)

	// compute composite identifier
	return flow.ConcatSum(resultID, chunkIndexID)
}

// NewApprovals creates a new memory pool for result approvals.
func NewApprovals(limit uint) (*Approvals, error) {
	var size uint
	ejector := NewSizeEjector(&size)
	a := &Approvals{
		size: &size,
		backend: NewBackend(
			WithLimit(limit),
			WithEject(ejector.Eject),
		),
	}
	return a, nil
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
			backdata[chunkKey] = model.ApprovalMapEntity{
				ChunkKey:   chunkKey,
				ResultID:   approval.Body.ExecutionResultID,
				ChunkIndex: approval.Body.ChunkIndex,
				Approvals:  chunkApprovals,
			}
		} else {
			approvalMapEntity, ok := entity.(model.ApprovalMapEntity)
			if !ok {
				return fmt.Errorf("unexpected entity type %T", entity)
			}

			chunkApprovals = approvalMapEntity.Approvals
			if _, ok := chunkApprovals[approval.Body.ApproverID]; ok {
				// approval is already associated with the chunk key and
				// approver, no need to append
				return nil
			}
		}

		// appends approval to the map
		chunkApprovals[approval.Body.ApproverID] = approval
		appended = true
		*a.size++
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
		var chunkApprovals map[flow.Identifier]*flow.ResultApproval

		entity, ok := backdata[chunkKey]
		if !ok {
			// no approvals for this chunk
			return nil
		}
		approvalMapEntity, ok := entity.(model.ApprovalMapEntity)
		if !ok {
			return fmt.Errorf("unexpected entity type %T", entity)
		}

		chunkApprovals = approvalMapEntity.Approvals
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
		*a.size--
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

		approvalMapEntity, ok := entity.(model.ApprovalMapEntity)
		if !ok {
			return fmt.Errorf("unexpected entity type %T", entity)
		}

		*a.size = *a.size - uint(len(approvalMapEntity.Approvals))

		delete(backdata, chunkKey)

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
	err := a.backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		entity, exists := backdata[chunkKey]
		if !exists {
			return nil
		}
		approvalMapEntity, ok := entity.(model.ApprovalMapEntity)
		if !ok {
			return fmt.Errorf("unexpected entity type %T", entity)
		}
		for i, app := range approvalMapEntity.Approvals {
			approvals[i] = app
		}
		return nil
	})
	if err != nil {
		// The current implementation never reaches this path, as it only stores
		// ApprovalMapEntity as entities in the mempool. Reaching this error
		// condition implies this code was inconsistently modified.
		panic("unexpected internal error in IncorporatedResults mempool: " + err.Error())
	}

	return approvals
}

// All will return all approvals in the memory pool.
func (a *Approvals) All() []*flow.ResultApproval {
	res := make([]*flow.ResultApproval, 0)

	err := a.backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		for _, entity := range backdata {
			approvalMapEntity, ok := entity.(model.ApprovalMapEntity)
			if !ok {
				// should never happen: as the mempool only stores ApprovalMapEntity
				return fmt.Errorf("unexpected entity type %T", entity)
			}
			for _, approval := range approvalMapEntity.Approvals {
				res = append(res, approval)
			}
		}
		return nil
	})
	if err != nil {
		// The current implementation never reaches this path, as it only stores
		// ApprovalMapEntity as entities in the mempool. Reaching this error
		// condition implies this code was inconsistently modified.
		panic("unexpected internal error in IncorporatedResults mempool: " + err.Error())
	}

	return res
}

// Size returns the number of approvals in the mempool.
func (a *Approvals) Size() uint {
	// To guarantee concurrency safety, i.e. that the read retrieves the latest size value,
	// we need run utilize the backend's lock.
	a.backend.RLock()
	defer a.backend.RUnlock()
	return *a.size
}
