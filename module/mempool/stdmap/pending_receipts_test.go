package stdmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/verification"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// TestLRUEject evaluates the ejection mechanism of PendingReceipts mempool
// every time the mempool gets full, the oldest entry should be ejected
func TestLRUEject(t *testing.T) {
	// creates a mempool with capacity of 3
	p, err := NewPendingReceipts(3)
	require.Nil(t, err)

	// generates 4 execution receipts and adds them to the mempool
	r1 := verification.NewPendingReceipt(unittest.ExecutionReceiptFixture(), unittest.IdentifierFixture())
	require.True(t, p.Add(r1))
	require.True(t, p.Has(r1.Receipt.ID()))

	r2 := verification.NewPendingReceipt(unittest.ExecutionReceiptFixture(), unittest.IdentifierFixture())
	require.True(t, p.Add(r2))
	require.True(t, p.Has(r2.Receipt.ID()))

	r3 := verification.NewPendingReceipt(unittest.ExecutionReceiptFixture(), unittest.IdentifierFixture())
	require.True(t, p.Add(r3))
	require.True(t, p.Has(r3.Receipt.ID()))

	r4 := verification.NewPendingReceipt(unittest.ExecutionReceiptFixture(), unittest.IdentifierFixture())
	require.True(t, p.Add(r4))
	require.True(t, p.Has(r4.Receipt.ID()))

	// at the end of inserting 4 items into a mempool with capacity of 3, the first item
	// should be ejected
	assert.False(t, p.Has(r1.Receipt.ID()))
	assert.True(t, p.Has(r2.Receipt.ID()))
	assert.True(t, p.Has(r3.Receipt.ID()))
	assert.True(t, p.Has(r4.Receipt.ID()))
}
