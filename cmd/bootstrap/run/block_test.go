package run

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestBlockEncodingJSON(t *testing.T) {
	block := unittest.BlockFixture()
	block.ParentVoterIDs = []flow.Identifier{}
	block.ParentVoterSig = crypto.Signature{}
	block.Seals = []*flow.Seal{}
	bz, err := json.Marshal(block)
	assert.NoError(t, err)
	var actual flow.Block
	err = json.Unmarshal(bz, &actual)
	assert.NoError(t, err)
	assert.Equal(t, block, actual)
}
