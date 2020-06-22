package flow_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestGenesisEncodingJSON(t *testing.T) {
	identities := unittest.IdentityListFixture(8)
	genesis := flow.Genesis(identities, flow.Mainnet)
	genesisID := genesis.ID()
	data, err := json.Marshal(genesis)
	require.NoError(t, err)
	var decoded flow.Block
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	decodedID := decoded.ID()
	assert.Equal(t, genesisID, decodedID)
	assert.Equal(t, genesis, &decoded)
}

func TestGenesisDecodingMsgpack(t *testing.T) {
	identities := unittest.IdentityListFixture(8)
	genesis := flow.Genesis(identities, flow.Mainnet)
	genesisID := genesis.ID()
	data, err := msgpack.Marshal(genesis)
	require.NoError(t, err)
	var decoded flow.Block
	err = msgpack.Unmarshal(data, &decoded)
	require.NoError(t, err)
	decodedID := decoded.ID()
	assert.Equal(t, genesisID, decodedID)
	assert.Equal(t, genesis, &decoded)
}

func TestBlockEncodingJSON(t *testing.T) {
	block := unittest.BlockFixture()
	blockID := block.ID()
	data, err := json.Marshal(block)
	require.NoError(t, err)
	var decoded flow.Block
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	decodedID := decoded.ID()
	assert.Equal(t, blockID, decodedID)
	assert.Equal(t, block, decoded)
}

func TestBlockEncodingMsgpack(t *testing.T) {
	block := unittest.BlockFixture()
	blockID := block.ID()
	data, err := msgpack.Marshal(block)
	require.NoError(t, err)
	var decoded flow.Block
	err = msgpack.Unmarshal(data, &decoded)
	require.NoError(t, err)
	decodedID := decoded.ID()
	assert.Equal(t, blockID, decodedID)
	assert.Equal(t, block, decoded)
}

func TestNilProducesSameHashAsEmptySlice(t *testing.T) {

	nilPayload := flow.Payload{
		Identities: nil,
		Guarantees: nil,
		Seals:      nil,
	}

	slicePayload := flow.Payload{
		Identities: make([]*flow.Identity, 0),
		Guarantees: make([]*flow.CollectionGuarantee, 0),
		Seals:      make([]*flow.Seal, 0),
	}

	assert.Equal(t, nilPayload.Hash(), slicePayload.Hash())
}

func TestOrderingChangesHash(t *testing.T) {

	identities := unittest.IdentityListFixture(5)

	payload1 := flow.Payload{
		Identities: identities,
	}

	payload2 := flow.Payload{
		Identities: []*flow.Identity{identities[3], identities[2], identities[4], identities[1], identities[0]},
	}

	assert.NotEqual(t, payload1.Hash(), payload2.Hash())
}
