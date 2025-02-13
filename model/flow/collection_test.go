package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/encoding/rlp"
	"github.com/onflow/flow-go/model/fingerprint"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestLightCollectionFingerprint(t *testing.T) {
	col := unittest.CollectionFixture(2)
	colID := col.ID()
	data := fingerprint.Fingerprint(col.Light())
	var decoded flow.LightCollection
	rlp.NewMarshaler().MustUnmarshal(data, &decoded)
	decodedID := decoded.ID()
	assert.Equal(t, colID, decodedID)
	assert.Equal(t, col.Light(), decoded)
}

// TestLightCollectionID_Malleability confirms that the LightCollection struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestLightCollectionID_Malleability(t *testing.T) {
	lightCollectionFixture := func() *flow.LightCollection {
		return &flow.LightCollection{
			Transactions: unittest.IdentifierListFixture(5),
		}
	}

	t.Run("LightCollection", func(t *testing.T) {
		unittest.RequireEntityNotMalleable(t, lightCollectionFixture())
	})
}
