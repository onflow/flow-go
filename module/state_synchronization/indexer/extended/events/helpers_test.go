package events

import (
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// ==========================================================================
// AddressFromOptional Tests
// ==========================================================================

func TestAddressFromOptional(t *testing.T) {
	t.Run("valid address", func(t *testing.T) {
		expected := unittest.RandomAddressFixture()
		opt := cadence.NewOptional(cadence.NewAddress(expected))
		addr, err := AddressFromOptional(opt)
		require.NoError(t, err)
		assert.Equal(t, expected, addr)
	})

	t.Run("nil optional value", func(t *testing.T) {
		opt := cadence.NewOptional(nil)
		addr, err := AddressFromOptional(opt)
		require.NoError(t, err)
		assert.Equal(t, flow.Address{}, addr)
	})

	t.Run("non-address value in optional returns error", func(t *testing.T) {
		opt := cadence.NewOptional(cadence.String("not an address"))
		_, err := AddressFromOptional(opt)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected type")
	})
}

// ==========================================================================
// PathFromOptional Tests
// ==========================================================================

func TestPathFromOptional(t *testing.T) {
	t.Run("valid path", func(t *testing.T) {
		path := cadence.Path{Domain: common.PathDomainStorage, Identifier: "flowToken"}
		opt := cadence.NewOptional(path)
		result, err := PathFromOptional(opt)
		require.NoError(t, err)
		assert.Equal(t, path.String(), result)
	})

	t.Run("nil optional value", func(t *testing.T) {
		opt := cadence.NewOptional(nil)
		result, err := PathFromOptional(opt)
		require.NoError(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("non-path value in optional returns error", func(t *testing.T) {
		opt := cadence.NewOptional(cadence.String("not a path"))
		_, err := PathFromOptional(opt)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected type")
	})
}

// ==========================================================================
// DecodePayload Tests
// ==========================================================================

func TestDecodePayload_InvalidPayload(t *testing.T) {
	event := flow.Event{
		Type:    "A.1234.SomeContract.SomeEvent",
		Payload: []byte("garbage"),
	}
	_, err := DecodePayload(event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode CCF payload")
}

func TestDecodePayload_NonEventPayload(t *testing.T) {
	// Encode a valid Cadence value that is not an Event.
	payload, err := ccf.Encode(cadence.String("not an event"))
	require.NoError(t, err)

	event := flow.Event{
		Type:    "A.1234.SomeContract.SomeEvent",
		Payload: payload,
	}
	_, err = DecodePayload(event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not an event")
}

// ==========================================================================
// Contract Event Decoder Tests
// ==========================================================================

// makeContractCadenceEvent builds a cadence.Event for a contract lifecycle event type
// (e.g. "flow.AccountContractAdded") with the given address, code hash, and contract name.
func makeContractCadenceEvent(eventTypeName string, addr flow.Address, codeHash []byte, contractName string) cadence.Event {
	hashValues := make([]cadence.Value, 32)
	for i, b := range codeHash {
		hashValues[i] = cadence.UInt8(b)
	}
	eventType := cadence.NewEventType(
		nil,
		eventTypeName,
		[]cadence.Field{
			{Identifier: "address", Type: cadence.AddressType},
			{Identifier: "codeHash", Type: cadence.NewConstantSizedArrayType(32, cadence.UInt8Type)},
			{Identifier: "contract", Type: cadence.StringType},
		},
		nil,
	)
	return cadence.NewEvent([]cadence.Value{
		cadence.NewAddress(addr),
		cadence.NewArray(hashValues).WithType(cadence.NewConstantSizedArrayType(32, cadence.UInt8Type)),
		cadence.String(contractName),
	}).WithType(eventType)
}

func TestDecodeAccountContractAdded(t *testing.T) {
	addr := unittest.RandomAddressFixture()
	codeHash := make([]byte, 32)
	for i := range codeHash {
		codeHash[i] = byte(i)
	}

	t.Run("valid event decodes all fields", func(t *testing.T) {
		event := makeContractCadenceEvent("flow.AccountContractAdded", addr, codeHash, "MyContract")
		result, err := DecodeAccountContractAdded(event)
		require.NoError(t, err)
		assert.Equal(t, addr, result.Address)
		assert.Equal(t, "MyContract", result.ContractName)
		assert.Equal(t, codeHash, result.CodeHash)
	})

	t.Run("non-array code hash returns error", func(t *testing.T) {
		// Build an event where codeHash is a String instead of [UInt8; 32].
		eventType := cadence.NewEventType(nil, "flow.AccountContractAdded", []cadence.Field{
			{Identifier: "address", Type: cadence.AddressType},
			{Identifier: "codeHash", Type: cadence.StringType},
			{Identifier: "contract", Type: cadence.StringType},
		}, nil)
		event := cadence.NewEvent([]cadence.Value{
			cadence.NewAddress(addr),
			cadence.String("notanarrayofbytes"),
			cadence.String("MyContract"),
		}).WithType(eventType)

		_, err := DecodeAccountContractAdded(event)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "codeHash")
	})
}

func TestDecodeAccountContractUpdated(t *testing.T) {
	addr := unittest.RandomAddressFixture()
	codeHash := make([]byte, 32)
	for i := range codeHash {
		codeHash[i] = byte(i + 1)
	}

	t.Run("valid event decodes all fields", func(t *testing.T) {
		event := makeContractCadenceEvent("flow.AccountContractUpdated", addr, codeHash, "UpdatedContract")
		result, err := DecodeAccountContractUpdated(event)
		require.NoError(t, err)
		assert.Equal(t, addr, result.Address)
		assert.Equal(t, "UpdatedContract", result.ContractName)
		assert.Equal(t, codeHash, result.CodeHash)
	})
}

func TestDecodeAccountContractRemoved(t *testing.T) {
	addr := unittest.RandomAddressFixture()
	codeHash := make([]byte, 32)
	for i := range codeHash {
		codeHash[i] = byte(i + 2)
	}

	t.Run("valid event decodes all fields", func(t *testing.T) {
		event := makeContractCadenceEvent("flow.AccountContractRemoved", addr, codeHash, "RemovedContract")
		result, err := DecodeAccountContractRemoved(event)
		require.NoError(t, err)
		assert.Equal(t, addr, result.Address)
		assert.Equal(t, "RemovedContract", result.ContractName)
		assert.Equal(t, codeHash, result.CodeHash)
	})
}

// ==========================================================================
// decodeCodeHashValue Tests
// ==========================================================================

func TestDecodeCodeHashValue(t *testing.T) {
	t.Run("non-array returns error", func(t *testing.T) {
		_, err := decodeCodeHashValue(cadence.String("notanarray"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected cadence.Array")
	})

	t.Run("array with non-UInt8 element returns error", func(t *testing.T) {
		val := cadence.NewArray([]cadence.Value{cadence.String("notabyte")})
		_, err := decodeCodeHashValue(val)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected cadence.UInt8")
	})

	t.Run("valid UInt8 array roundtrips correctly", func(t *testing.T) {
		hash := make([]byte, 32)
		for i := range hash {
			hash[i] = byte(i)
		}
		vals := make([]cadence.Value, 32)
		for i, b := range hash {
			vals[i] = cadence.UInt8(b)
		}
		result, err := decodeCodeHashValue(cadence.NewArray(vals))
		require.NoError(t, err)
		assert.Equal(t, hash, result)
	})
}
