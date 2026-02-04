package approvals

import "github.com/onflow/flow-go/model/flow"

// The functions in this file expose internal state for testing purposes only.
// They are only available to tests in the approvals_test package.

// GetCollectorState returns the underlying AssignmentCollectorState for testing.
func (asm *AssignmentCollectorStateMachine) GetCollectorState() AssignmentCollectorState {
	return asm.atomicLoadCollector()
}

// HasApprovalBeenProcessed checks if an approval from a given approver for a specific chunk
// has been processed for a given incorporated block. This is used for testing to verify
// that approvals were correctly transferred from CachingAssignmentCollector to VerifyingAssignmentCollector.
func (ac *VerifyingAssignmentCollector) HasApprovalBeenProcessed(incorporatedBlockID flow.Identifier, chunkIndex uint64, approverID flow.Identifier) bool {
	ac.lock.RLock()
	collector, ok := ac.collectors[incorporatedBlockID]
	ac.lock.RUnlock()
	if !ok {
		return false
	}

	if chunkIndex >= uint64(len(collector.chunkCollectors)) {
		return false
	}

	chunkCollector := collector.chunkCollectors[chunkIndex]
	chunkCollector.lock.Lock()
	defer chunkCollector.lock.Unlock()
	return chunkCollector.chunkApprovals.HasSigned(approverID)
}

// HasIncorporatedResult checks if an incorporated result has been processed for a given incorporated block.
func (ac *VerifyingAssignmentCollector) HasIncorporatedResult(incorporatedBlockID flow.Identifier) bool {
	ac.lock.RLock()
	defer ac.lock.RUnlock()
	_, ok := ac.collectors[incorporatedBlockID]
	return ok
}
