package stdmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestPendingReceiptsLRUEject evaluates the ejection mechanism of PendingReceipts mempool
// every time the mempool gets full, the oldest entry should be ejected
func TestPendingReceiptsLRUEject(t *testing.T) {
	var total uint = 4
	// creates a mempool with capacity one less than `total`
	p, err := NewReceiptDataPacks(total - 1)
	require.Nil(t, err)

	// generates `total` execution receipts and adds them to the mempool
	receipts := make([]*flow.ExecutionReceipt, total)
	for i := 0; i < int(total); i++ {
		receipt := unittest.ExecutionReceiptFixture()
		pr := &verification.ReceiptDataPack{
			Receipt:  receipt,
			OriginID: unittest.IdentifierFixture(),
		}
		require.True(t, p.Add(pr))
		require.True(t, p.Has(pr.ID()))
		receipts[i] = receipt
	}

	for i := 0; i < int(total); i++ {
		if i == 0 {
			// first item should be ejected to make the room for surplus item
			assert.False(t, p.Has(receipts[i].ID()))
			continue
		}
		// other receipt data packs should be available in mempool
		assert.True(t, p.Has(receipts[i].ID()))
	}
}
