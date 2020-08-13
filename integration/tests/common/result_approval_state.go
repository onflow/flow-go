package common

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
)

const resultApprovalTimeout = 100 * time.Second

// ResultApprovalState keeps track of the result approval messages getting disseminated through
// test net network.
type ResultApprovalState struct {
	resultApprovals sync.Map
}

// Add adds a result approval captured from the testnet to the result approval tracker.
func (r *ResultApprovalState) Add(sender flow.Identifier, approval *flow.ResultApproval) {
	var list []*flow.ResultApproval

	// casts value to list if exists
	if value, ok := r.resultApprovals.Load(sender); ok {
		list = value.([]*flow.ResultApproval)
	}

	list = append(list, approval)
	r.resultApprovals.Store(sender, list)
}

// WaitForResultApproval waits until a result approval for execution result id from the verification node for
// the chunk index within a timeout. It returns the captured result approval.
func (r *ResultApprovalState) WaitForResultApproval(t *testing.T,
	verNodeID, resultID flow.Identifier,
	chunkIndex uint64) *flow.ResultApproval {

	var resultApproval *flow.ResultApproval
	require.Eventually(t, func() bool {
		list, ok := r.resultApprovals.Load(verNodeID)

		if !ok {
			return false
		}

		approvals := list.([]*flow.ResultApproval)

		for _, approval := range approvals {
			if !bytes.Equal(approval.Body.ExecutionResultID[:], resultID[:]) {
				continue // execution result IDs do not match
			}
			if chunkIndex != approval.Body.ChunkIndex {
				continue // chunk indices do not match
			}
			resultApproval = approval
			return true
		}

		return false

	}, resultApprovalTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive result approval for chunk %d of result ID %x from %x",
			chunkIndex,
			resultID,
			verNodeID))

	return resultApproval

}
