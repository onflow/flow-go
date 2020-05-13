package stdmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// TestPendingCollectionLRUEject evaluates the ejection mechanism of PendingCollections mempool
// every time the mempool gets full, the oldest entry should be ejected
func TestPendingCollectionLRUEject(t *testing.T) {
	// creates a mempool with capacity of 3
	p, err := NewPendingCollections(3)
	require.Nil(t, err)

	// generates 4 execution receipts and adds them to the mempool
	colls := make([]*flow.Collection, 4)
	for i := 0; i < 4; i++ {
		coll := unittest.CollectionFixture(1)
		pc := verification.NewPendingCollection(&coll, unittest.IdentifierFixture())
		require.True(t, p.Add(pc))
		require.True(t, p.Has(pc.ID()))
		colls[i] = &coll
	}

	for i := 0; i < 4; i++ {
		if i == 0 {
			// first item should be ejected to make the room for surplus item
			assert.False(t, p.Has(colls[i].ID()))
			continue
		}
		// other pending receipts should be available in mempool
		assert.True(t, p.Has(colls[i].ID()))
	}
}
