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
	payload := *flow.NewEmptyPayload()
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
	assert.Equal(t, `{"Guarantees":null,"Seals":null,"Receipts":null,"Results":null,"ProtocolStateID":"0000000000000000000000000000000000000000000000000000000000000000"}`, string(encoded2))
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

// TestNewPayload verifies the behavior of the NewPayload constructor.
func TestNewPayload(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		payload := unittest.PayloadFixture(
			unittest.WithProtocolStateID(unittest.IdentifierFixture()),
		)

		res, err := flow.NewPayload(flow.UntrustedPayload(payload))
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("zero ProtocolStateID rejected", func(t *testing.T) {
		payload := unittest.PayloadFixture()
		payload.ProtocolStateID = flow.ZeroID

		res, err := flow.NewPayload(flow.UntrustedPayload(payload))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "ProtocolStateID must not be zero")
	})

	t.Run("nil Guarantee element rejected", func(t *testing.T) {
		untrusted := flow.UntrustedPayload(unittest.PayloadFixture(
			unittest.WithProtocolStateID(unittest.IdentifierFixture()),
		))
		untrusted.Guarantees = []*flow.CollectionGuarantee{nil}

		res, err := flow.NewPayload(untrusted)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "guarantee at index 0 is nil")
	})

	t.Run("nil Seal element rejected", func(t *testing.T) {
		untrusted := flow.UntrustedPayload(unittest.PayloadFixture(
			unittest.WithProtocolStateID(unittest.IdentifierFixture()),
		))
		untrusted.Seals = []*flow.Seal{nil}

		res, err := flow.NewPayload(untrusted)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "seal at index 0 is nil")
	})

	t.Run("nil Receipt element rejected", func(t *testing.T) {
		untrusted := flow.UntrustedPayload(unittest.PayloadFixture(
			unittest.WithProtocolStateID(unittest.IdentifierFixture()),
		))
		untrusted.Receipts = flow.ExecutionReceiptStubList{nil}

		res, err := flow.NewPayload(untrusted)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "receipt at index 0 is nil")
	})

	t.Run("nil Result element rejected", func(t *testing.T) {
		untrusted := flow.UntrustedPayload(unittest.PayloadFixture(
			unittest.WithProtocolStateID(unittest.IdentifierFixture()),
		))
		untrusted.Results = flow.ExecutionResultList{nil}

		res, err := flow.NewPayload(untrusted)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "result at index 0 is nil")
	})
}
