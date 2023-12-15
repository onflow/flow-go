package convert_test

import (
	"testing"

	"github.com/stretchr/testify/require"

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
				Type:  convert.KeyPartOwner,
				Value: []byte(expectedRegisterID.Owner),
			},
			{
				Type:  convert.KeyPartKey,
				Value: []byte("key"),
			},
		},
	}

	registerID, err := convert.LedgerKeyToRegisterID(key)
	require.NoError(t, err)
	require.Equal(t, expectedRegisterID, registerID)
}

func TestLedgerKeyToRegisterID_Global(t *testing.T) {
	key := ledger.Key{
		KeyParts: []ledger.KeyPart{
			{
				Type:  convert.KeyPartOwner,
				Value: []byte(""),
			},
			{
				Type:  convert.KeyPartKey,
				Value: []byte("uuid"),
			},
		},
	}

	expectedRegisterID := flow.UUIDRegisterID(0)
	registerID, err := convert.LedgerKeyToRegisterID(key)
	require.NoError(t, err)
	require.Equal(t, expectedRegisterID, registerID)
}

func TestLedgerKeyToRegisterID_Error(t *testing.T) {
	key := ledger.Key{
		KeyParts: []ledger.KeyPart{
			{
				Type:  999, // Invalid type
				Value: []byte("owner"),
			},
			{
				Type:  convert.KeyPartKey,
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
				Type: convert.KeyPartOwner,
				// Note: the owner field is extended to address length during NewRegisterID
				// so we have to do the same here
				Value: []byte(registerID.Owner),
			},
			{
				Type:  convert.KeyPartKey,
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
				Type:  convert.KeyPartOwner,
				Value: []byte(""),
			},
			{
				Type:  convert.KeyPartKey,
				Value: []byte("uuid"),
			},
		},
	}

	key := convert.RegisterIDToLedgerKey(registerID)
	require.Equal(t, expectedKey, key)
}
