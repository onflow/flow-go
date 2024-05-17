package convert_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestLedgerKeyToRegisterID(t *testing.T) {
	expectedRegisterID := unittest.RegisterIDFixture()

	key := ledger.Key{
		KeyParts: []ledger.KeyPart{
			{
				Type:  ledger.KeyPartOwner,
				Value: []byte(expectedRegisterID.Owner),
			},
			{
				Type:  ledger.KeyPartKey,
				Value: []byte("key"),
			},
		},
	}

	registerID, err := convert.LedgerKeyToRegisterID(key)
	require.NoError(t, err)
	require.Equal(t, expectedRegisterID, registerID)

	p := ledger.NewPayload(key, ledger.Value("value"))

	address, err := util.PayloadToAddress(p)

	require.NoError(t, err)
	require.Equal(t, registerID.Owner, convert.AddressToRegisterOwner(address))
	require.Equal(t, registerID.Owner, string(address[:]))
}

func TestLedgerKeyToRegisterID_Global(t *testing.T) {
	key := ledger.Key{
		KeyParts: []ledger.KeyPart{
			{
				Type:  ledger.KeyPartOwner,
				Value: []byte(""),
			},
			{
				Type:  ledger.KeyPartKey,
				Value: []byte("uuid"),
			},
		},
	}

	expectedRegisterID := flow.UUIDRegisterID(0)
	registerID, err := convert.LedgerKeyToRegisterID(key)
	require.NoError(t, err)
	require.Equal(t, expectedRegisterID, registerID)

	p := ledger.NewPayload(key, ledger.Value("value"))

	address, err := util.PayloadToAddress(p)

	require.NoError(t, err)
	require.Equal(t, registerID.Owner, convert.AddressToRegisterOwner(address))
	require.NotEqual(t, registerID.Owner, string(address[:]))
}

func TestLedgerKeyToRegisterID_Error(t *testing.T) {
	key := ledger.Key{
		KeyParts: []ledger.KeyPart{
			{
				Type:  999, // Invalid type
				Value: []byte("owner"),
			},
			{
				Type:  ledger.KeyPartKey,
				Value: []byte("key"),
			},
		},
	}

	_, err := convert.LedgerKeyToRegisterID(key)
	require.Error(t, err)
	require.ErrorIs(t, err, convert.UnexpectedLedgerKeyFormat)
}

func TestRegisterIDToLedgerKey(t *testing.T) {
	registerID := unittest.RegisterIDFixture()
	expectedKey := ledger.Key{
		KeyParts: []ledger.KeyPart{
			{
				Type: ledger.KeyPartOwner,
				// Note: the owner field is extended to address length during NewRegisterID
				// so we have to do the same here
				Value: []byte(registerID.Owner),
			},
			{
				Type:  ledger.KeyPartKey,
				Value: []byte("key"),
			},
		},
	}

	key := convert.RegisterIDToLedgerKey(registerID)
	require.Equal(t, expectedKey, key)
}

func TestRegisterIDToLedgerKey_Global(t *testing.T) {
	registerID := flow.UUIDRegisterID(0)
	expectedKey := ledger.Key{
		KeyParts: []ledger.KeyPart{
			{
				Type:  ledger.KeyPartOwner,
				Value: []byte(""),
			},
			{
				Type:  ledger.KeyPartKey,
				Value: []byte("uuid"),
			},
		},
	}

	key := convert.RegisterIDToLedgerKey(registerID)
	require.Equal(t, expectedKey, key)
}

func TestPayloadToRegister(t *testing.T) {
	expected := unittest.RegisterIDFixture()
	t.Run("can convert", func(t *testing.T) {
		value := []byte("value")
		p := ledger.NewPayload(
			ledger.NewKey(
				[]ledger.KeyPart{
					ledger.NewKeyPart(ledger.KeyPartOwner, []byte(expected.Owner)),
					ledger.NewKeyPart(ledger.KeyPartKey, []byte(expected.Key)),
				},
			),
			value,
		)
		regID, regValue, err := convert.PayloadToRegister(p)
		require.NoError(t, err)
		require.Equal(t, expected, regID)
		require.Equal(t, value, regValue)
	})

	t.Run("global key", func(t *testing.T) {
		value := []byte("1")
		p := ledger.NewPayload(
			ledger.NewKey(
				[]ledger.KeyPart{
					ledger.NewKeyPart(ledger.KeyPartOwner, []byte("")),
					ledger.NewKeyPart(ledger.KeyPartKey, []byte("uuid")),
				},
			),
			value,
		)
		regID, regValue, err := convert.PayloadToRegister(p)
		require.NoError(t, err)
		require.Equal(t, flow.NewRegisterID(flow.EmptyAddress, "uuid"), regID)
		require.Equal(t, "", regID.Owner)
		require.Equal(t, "uuid", regID.Key)
		require.True(t, regID.IsInternalState())
		require.Equal(t, value, regValue)
	})

	t.Run("empty payload", func(t *testing.T) {
		p := ledger.EmptyPayload()
		_, _, err := convert.PayloadToRegister(p)
		require.Error(t, err)
	})
}
