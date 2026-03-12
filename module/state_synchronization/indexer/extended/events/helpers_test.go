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

const testBlockHeight = uint64(100)

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
