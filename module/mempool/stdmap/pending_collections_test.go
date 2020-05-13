package stdmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// TODO consolidate with PendingReceipts to preserve DRY
// https://github.com/dapperlabs/flow-go/issues/3690
// TestPendingCollectionLRUEject evaluates the ejection mechanism of PendingCollections mempool
// every time the mempool gets full, the oldest entry should be ejected
func TestPendingCollectionLRUEject(t *testing.T) {
	var total uint = 4
	// creates a mempool with capacity one less than `total`
	p, err := NewPendingCollections(total - 1)
	require.Nil(t, err)

	// generates `total` collections and adds them to the mempool
	colls := make([]*flow.Collection, total)
	for i := 0; i < int(total); i++ {
		coll := unittest.CollectionFixture(1)
		pc := verification.NewPendingCollection(&coll, unittest.IdentifierFixture())
		require.True(t, p.Add(pc))
		require.True(t, p.Has(pc.ID()))
		colls[i] = &coll
	}

	for i := 0; i < int(total); i++ {
		if i == 0 {
			// first item should be ejected to make the room for surplus item
			assert.False(t, p.Has(colls[i].ID()))
			continue
		}
		// other pending collections should be available in mempool
		assert.True(t, p.Has(colls[i].ID()))
	}
}
