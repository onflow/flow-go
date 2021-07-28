package receiptstate

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

const ReceiptTimeout = 60 * time.Second
const StateTimeout = 120 * time.Second

type ReceiptState struct {
	sync.RWMutex

	// receipts contains execution receipts are indexed by blockID, then by executorID
	receipts map[flow.Identifier]map[flow.Identifier]*flow.ExecutionReceipt
}

func NewReceiptState() *ReceiptState {
	return &ReceiptState{
		RWMutex:  sync.RWMutex{},
		receipts: make(map[flow.Identifier]map[flow.Identifier]*flow.ExecutionReceipt),
	}
}

func (rs *ReceiptState) Add(er *flow.ExecutionReceipt) {
	rs.Lock() // avoiding concurrent map access
	defer rs.Unlock()

	if rs.receipts[er.ExecutionResult.BlockID] == nil {
		rs.receipts[er.ExecutionResult.BlockID] = make(map[flow.Identifier]*flow.ExecutionReceipt)
	}

	rs.receipts[er.ExecutionResult.BlockID][er.ExecutorID] = er
}

// WaitForReceiptFromAny waits for an execution receipt for the given blockID from any execution node and returns it
func (rs *ReceiptState) WaitForReceiptFromAny(t *testing.T, blockID flow.Identifier) *flow.ExecutionReceipt {
	require.Eventually(t, func() bool {
		rs.RLock() // avoiding concurrent map access
		defer rs.RUnlock()

		return len(rs.receipts[blockID]) > 0
	}, ReceiptTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive execution receipt for block ID %x from any node within %v seconds", blockID,
			ReceiptTimeout))
	for _, r := range rs.receipts[blockID] {
		return r
	}
	panic("there needs to be an entry in rs.receipts[blockID]")
}

// WaitForReceiptFrom waits for an execution receipt for the given blockID and the given executorID and returns it
func (rs *ReceiptState) WaitForReceiptFrom(t *testing.T, blockID, executorID flow.Identifier) *flow.ExecutionReceipt {
	var r *flow.ExecutionReceipt
	require.Eventually(t, func() bool {
		rs.RLock() // avoiding concurrent map access
		defer rs.RUnlock()

		var ok bool
		r, ok = rs.receipts[blockID][executorID]
		return ok
	}, ReceiptTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive execution receipt for block ID %x from %x within %v seconds", blockID, executorID,
			ReceiptTimeout))
	return r
}
