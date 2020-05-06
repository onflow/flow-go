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

func TestPayloadEncodeJSON(t *testing.T) {
	payload := unittest.PayloadFixture()
	payloadHash := payload.Hash()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var decoded flow.Payload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	decodedHash := decoded.Hash()
	assert.Equal(t, payloadHash, decodedHash)
	assert.Equal(t, payload, &decoded)
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
	assert.Equal(t, payload, &decoded)
}
