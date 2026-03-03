package remote

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestClientSetEmptyUpdate verifies that the Client.Set method handles empty updates
// correctly without making an RPC call. This matches the behavior of the local ledger
// implementations (ledger/complete/ledger.go and ledger/partial/ledger.go).
//
// When a transaction/collection doesn't modify any registers (read-only), the update
// will have zero keys. The client should return the same state without making an RPC
// call, avoiding the "keys cannot be empty" error from the server.
func TestClientSetEmptyUpdate(t *testing.T) {
	// Create an empty update (no keys, no values)
	state := ledger.State(unittest.StateCommitmentFixture())
	update, err := ledger.NewUpdate(state, []ledger.Key{}, []ledger.Value{})
	require.NoError(t, err)
	require.Equal(t, 0, update.Size(), "update should have zero keys")

	// Create a client with no actual connection (we shouldn't need it for empty updates)
	// The client will panic if it tries to make an RPC call, which verifies we don't call the server
	client := &Client{
		// All fields are nil/zero - any RPC call would panic
	}

	// Call Set with empty update
	newState, trieUpdate, err := client.Set(update)

	// Verify no error
	require.NoError(t, err, "Set with empty update should not return error")

	// Verify the state is unchanged
	assert.Equal(t, state, newState, "new state should equal original state for empty update")

	// Verify the trie update is valid but empty
	require.NotNil(t, trieUpdate, "trie update should not be nil")
	assert.Equal(t, ledger.RootHash(state), trieUpdate.RootHash, "trie update root hash should match state")
	assert.Empty(t, trieUpdate.Paths, "trie update should have no paths for empty update")
	assert.Empty(t, trieUpdate.Payloads, "trie update should have no payloads for empty update")
}

// TestClientSetEmptyUpdateMatchesLocalLedger verifies that the remote client's
// empty update handling produces the same result as the local ledger implementations.
func TestClientSetEmptyUpdateMatchesLocalLedger(t *testing.T) {
	state := ledger.State(unittest.StateCommitmentFixture())
	update, err := ledger.NewUpdate(state, []ledger.Key{}, []ledger.Value{})
	require.NoError(t, err)

	// Get result from remote client (without actual connection)
	client := &Client{}
	remoteState, remoteTrieUpdate, err := client.Set(update)
	require.NoError(t, err)

	// The expected result matches what local ledger implementations return:
	// - State unchanged
	// - TrieUpdate with RootHash equal to state, empty Paths and Payloads
	expectedTrieUpdate := &ledger.TrieUpdate{
		RootHash: ledger.RootHash(state),
		Paths:    []ledger.Path{},
		Payloads: []*ledger.Payload{},
	}

	assert.Equal(t, state, remoteState, "state should be unchanged")
	assert.Equal(t, expectedTrieUpdate.RootHash, remoteTrieUpdate.RootHash)
	assert.Equal(t, len(expectedTrieUpdate.Paths), len(remoteTrieUpdate.Paths))
	assert.Equal(t, len(expectedTrieUpdate.Payloads), len(remoteTrieUpdate.Payloads))
}
