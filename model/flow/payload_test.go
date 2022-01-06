package flow_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestPayloadEncodeEmptyJSON(t *testing.T) {
	// nil slices
	payload := unittest.PayloadFixture()
	payloadHash1 := payload.Hash()
	encoded1, err := json.Marshal(payload)
	require.NoError(t, err)
	var decoded flow.Payload
	err = json.Unmarshal(encoded1, &decoded)
	require.NoError(t, err)
	assert.Equal(t, payloadHash1, decoded.Hash())
	assert.Equal(t, payload, decoded)

	// empty slices converted to nil
	payload.Seals = []*flow.Seal{}
	payloadHash2 := payload.Hash()
	assert.Equal(t, payloadHash2, payloadHash1)
	encoded2, err := json.Marshal(payload)
	assert.Equal(t, `{"Guarantees":null,"Seals":null,"Receipts":null,"Results":null}`, string(encoded2))
	assert.Equal(t, string(encoded1), string(encoded2))
	require.NoError(t, err)
	err = json.Unmarshal(encoded2, &decoded)
	require.NoError(t, err)
	require.Nil(t, decoded.Seals)
	assert.Equal(t, payloadHash2, decoded.Hash())
}

func TestPayloadEncodeJSON(t *testing.T) {
	payload := unittest.PayloadFixture()
	payload.Seals = []*flow.Seal{{}}
	payloadHash := payload.Hash()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var decoded flow.Payload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, payloadHash, decoded.Hash())
	assert.Equal(t, payload, decoded)
}

func TestPayloadEncodingMsgpack(t *testing.T) {
	payload := unittest.PayloadFixture()
	payloadHash := payload.Hash()
	data, err := msgpack.Marshal(payload)
	require.NoError(t, err)
	var decoded flow.Payload
	err = msgpack.Unmarshal(data, &decoded)
	require.NoError(t, err)
	decodedHash := decoded.Hash()
	assert.Equal(t, payloadHash, decodedHash)
	assert.Equal(t, payload, decoded)
}
