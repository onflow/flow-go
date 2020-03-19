package run

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"testing"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestBlockEncodingYAML(t *testing.T) {
	block := unittest.BlockFixture()
	block.ParentSigners = []flow.Identifier{}
	block.ParentStakingSigs = []crypto.Signature{}
	block.ParentRandomBeaconSig = crypto.Signature{}
	block.Seals = []*flow.Seal{}
	bz, err := yaml.Marshal(block)
	assert.NoError(t, err)
	fmt.Printf("%v", block)
	// assert.Equal(t, fmt.Sprintf("%v\n", block), string(bz))
	var actual flow.Block
	err = yaml.Unmarshal(bz, &actual)
	assert.NoError(t, err)
	assert.Equal(t, block, actual)
}

func TestBlockEncodingGob(t *testing.T) {
	block := unittest.BlockFixture()

	gob.Register(crypto.PubKeyECDSA{})
	gob.Register(crypto.PubKeyBLS_BLS12381{})

	staking, err := crypto.GeneratePrivateKey(crypto.BLS_BLS12381, []byte{0x01, 0x02})
	require.NoError(t, err)
	block.Identities[0].StakingPubKey = staking.PublicKey()

	var bz bytes.Buffer
	enc := gob.NewEncoder(&bz)
	dec := gob.NewDecoder(&bz)

	err = enc.Encode(block)
	require.NoError(t, err)

	var actual flow.Block
	err = dec.Decode(&actual)
	require.NoError(t, err)
	assert.Equal(t, block, actual)

	assert.Equal(t, block.Identities[0].StakingPubKey, actual.Identities[0].StakingPubKey)
	assert.Equal(t, block.Identities[0].StakingPubKey, actual.Identities[0].NetworkPubKey)
}
