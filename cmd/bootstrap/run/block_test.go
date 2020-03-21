package run

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
)

func TestBlockEncodingJSON(t *testing.T) {
	block := unittest.BlockFixture()
	block.ParentSigners = []flow.Identifier{}
	block.ParentStakingSigs = []crypto.Signature{}
	block.ParentRandomBeaconSig = crypto.Signature{}
	block.Seals = []*flow.Seal{}
	bz, err := json.Marshal(block)
	assert.NoError(t, err)
	fmt.Printf("%v", block)
	var actual flow.Block
	err = json.Unmarshal(bz, &actual)
	assert.NoError(t, err)
	assert.Equal(t, block, actual)
}
