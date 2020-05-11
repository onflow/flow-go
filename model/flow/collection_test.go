package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/encoding/rlp"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestLightCollectionEncodingRLP(t *testing.T) {
	col := unittest.CollectionFixture(2)
	colID := col.ID()
	data := col.Light().Encode()
	var decoded flow.LightCollection
	rlp.NewEncoder().MustDecode(data, &decoded)
	decodedID := decoded.ID()
	assert.Equal(t, colID, decodedID)
	assert.Equal(t, col.Light(), decoded)
}
