package lib

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

const resultApprovalTimeout = 120 * time.Second

// ResultApprovalState keeps track of the result approval messages getting disseminated through
// test net network.
type ResultApprovalState struct {
	sync.RWMutex
	resultApprovals map[flow.Identifier][]*flow.ResultApproval
}

func NewResultApprovalState() *ResultApprovalState {
	return &ResultApprovalState{
		RWMutex:         sync.RWMutex{},
		resultApprovals: make(map[flow.Identifier][]*flow.ResultApproval),
	}
}

// Add adds a result approval captured from the testnet to the result approval tracker.
func (r *ResultApprovalState) Add(sender flow.Identifier, approval *flow.ResultApproval) {
	r.Lock()
	defer r.Unlock()

	if _, ok := r.resultApprovals[sender]; !ok {
		r.resultApprovals[sender] = make([]*flow.ResultApproval, 0)
	}
	r.resultApprovals[sender] = append(r.resultApprovals[sender], approval)
}

// WaitForResultApproval waits until a result approval for execution result id from the verification node for
// the chunk index within a timeout. It returns the captured result approval.
func (r *ResultApprovalState) WaitForResultApproval(t *testing.T, verNodeID, resultID flow.Identifier, chunkIndex uint64) *flow.ResultApproval {
	approvals := r.WaitForTotalApprovalsFrom(t, flow.IdentifierList{verNodeID}, resultID, chunkIndex, 1)
	if len(approvals) > 0 {
		return approvals[0]
	}
	return nil
}

// WaitForTotalApprovalsFrom waits until "count" number of result approval for the given execution result id and
// the chunk index is disseminated in the network from any subset of the given (verification) ids.
// It returns the captured result approvals.
func (r *ResultApprovalState) WaitForTotalApprovalsFrom(
	t *testing.T,
	verificationIds flow.IdentifierList,
	resultID flow.Identifier,
	chunkIndex uint64,
	count int) []*flow.ResultApproval {

	receivedApprovalIds := flow.IdentifierList{}
	receivedApprovals := make([]*flow.ResultApproval, 0)

	require.Eventually(t, func() bool {
		r.RLock()
		defer r.RUnlock()

		for _, verificationId := range verificationIds {
			approvals, ok := r.resultApprovals[verificationId]
			if !ok {
				continue
			}

			for _, approval := range approvals {
				if !bytes.Equal(approval.Body.ExecutionResultID[:], resultID[:]) {
					continue // execution result IDs do not match
				}
				if chunkIndex != approval.Body.ChunkIndex {
					continue // chunk indices do not match
				}
				approvalId := approval.ID()
				if !receivedApprovalIds.Contains(approvalId) {
					receivedApprovalIds = append(receivedApprovalIds, approvalId)
					receivedApprovals = append(receivedApprovals, approval)
				}

				if len(receivedApprovalIds) == count {
					return true
				}
			}
		}

		return false
	}, resultApprovalTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive enough approval for chunk %d of result ID %x",
			chunkIndex,
			resultID))

	return receivedApprovals
}
