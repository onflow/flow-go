package indexer

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/utils/unittest"
)

// TrieUpdateRandomLedgerPayloadsFixture creates a test trie update with multiple test payloads
// for use in testing register persistence functionality.
func TrieUpdateRandomLedgerPayloadsFixture(t *testing.T) *ledger.TrieUpdate {
	return TrieUpdateWithPayloadsFixture(
		t,
		[]*ledger.Payload{
			LedgerRandomPayloadFixture(t),
			LedgerRandomPayloadFixture(t),
			LedgerRandomPayloadFixture(t),
			LedgerRandomPayloadFixture(t),
		})
}

// TrieUpdateWithPayloadsFixture creates a trie update from the provided payloads.
// It extracts keys and values from payloads and constructs a proper ledger update
// and trie update structure for testing purposes.
func TrieUpdateWithPayloadsFixture(t *testing.T, payloads []*ledger.Payload) *ledger.TrieUpdate {
	keys := make([]ledger.Key, 0)
	values := make([]ledger.Value, 0)
	for _, payload := range payloads {
		key, err := payload.Key()
		require.NoError(t, err)
		keys = append(keys, key)
		values = append(values, payload.Value())
	}

	update, err := ledger.NewUpdate(ledger.DummyState, keys, values)
	require.NoError(t, err)
	trie, err := pathfinder.UpdateToTrieUpdate(update, complete.DefaultPathFinderVersion)
	require.NoError(t, err)
	return trie
}

// LedgerRandomPayloadFixture creates a single test payload with a random owner, key, and value
// for use in ledger and register testing scenarios.
func LedgerRandomPayloadFixture(t *testing.T) *ledger.Payload {
	owner := unittest.RandomAddressFixture()
	key := make([]byte, 8)
	_, err := rand.Read(key)
	require.NoError(t, err)
	val := make([]byte, 8)
	_, err = rand.Read(val)
	require.NoError(t, err)
	return LedgerPayloadFixture(owner.String(), fmt.Sprintf("%x", key), val)
}

// LedgerPayloadFixture creates a ledger payload with the specified owner, key, and value.
// It constructs a proper ledger key with owner and key parts and returns a payload
// suitable for testing ledger operations.
func LedgerPayloadFixture(owner string, key string, value []byte) *ledger.Payload {
	k := ledger.Key{
		KeyParts: []ledger.KeyPart{
			{
				Type:  ledger.KeyPartOwner,
				Value: []byte(owner),
			},
			{
				Type:  ledger.KeyPartKey,
				Value: []byte(key),
			},
		},
	}

	return ledger.NewPayload(k, value)
}
