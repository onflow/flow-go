package transfers

import (
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const testBlockHeight = uint64(100)

// ==========================================================================
// addressFromOptional Tests
// ==========================================================================

func TestAddressFromOptional(t *testing.T) {
	t.Run("valid address", func(t *testing.T) {
		expected := unittest.RandomAddressFixture()
		opt := cadence.NewOptional(cadence.NewAddress(expected))
		addr, err := addressFromOptional(opt)
		require.NoError(t, err)
		assert.Equal(t, expected, addr)
	})

	t.Run("nil optional value", func(t *testing.T) {
		opt := cadence.NewOptional(nil)
		addr, err := addressFromOptional(opt)
		require.NoError(t, err)
		assert.Equal(t, flow.Address{}, addr)
	})

	t.Run("non-address value in optional returns error", func(t *testing.T) {
		opt := cadence.NewOptional(cadence.String("not an address"))
		_, err := addressFromOptional(opt)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected type")
	})
}

// ==========================================================================
// DecodeEvent Tests
// ==========================================================================

func TestDecodeEvent_InvalidPayload(t *testing.T) {
	event := flow.Event{
		Type:    "A.1234.SomeContract.SomeEvent",
		Payload: []byte("garbage"),
	}
	_, err := DecodeEvent(event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode CCF payload")
}

func TestDecodeEvent_NonEventPayload(t *testing.T) {
	// Encode a valid Cadence value that is not an Event.
	payload, err := ccf.Encode(cadence.String("not an event"))
	require.NoError(t, err)

	event := flow.Event{
		Type:    "A.1234.SomeContract.SomeEvent",
		Payload: payload,
	}
	_, err = DecodeEvent(event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not an event")
}
