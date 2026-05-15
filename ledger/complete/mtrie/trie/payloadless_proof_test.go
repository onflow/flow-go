package trie_test

import (
	crand "crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/model/flow"
)

// randomBytes generates n random bytes using crypto/rand
func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := crand.Read(b)
	if err != nil {
		panic("random generation failed")
	}
	return b
}

// TestReconstructPayloadlessProof tests the naive decode → verify → modify → encode approach.
func TestReconstructPayloadlessProof(t *testing.T) {
	t.Run("basic reconstruction", func(t *testing.T) {
		// Create test data
		owner := randomBytes(8)
		key := "test_key"
		value := randomBytes(100)

		registerID := flow.NewRegisterID(flow.BytesToAddress(owner), key)
		ledgerKey := convert.RegisterIDToLedgerKey(registerID)
		payload := ledger.NewPayload(ledgerKey, value)

		path, err := pathfinder.KeyToPath(ledgerKey, 1)
		require.NoError(t, err)

		// Create payloadless trie
		payloadlessTrie := trie.NewEmptyMTrieWithPayloadless(true)
		payloadlessTrie, _, err = trie.NewTrieWithUpdatedRegisters(payloadlessTrie, []ledger.Path{path}, []ledger.Payload{*payload}, true)
		require.NoError(t, err)

		// Get proof from payloadless trie and encode it
		directProofs := payloadlessTrie.UnsafeProofs([]ledger.Path{path})
		encodedProof := ledger.EncodeTrieBatchProof(directProofs)

		// Create value reader
		valueReader := func(regID flow.RegisterID) (flow.RegisterValue, error) {
			if regID == registerID {
				return value, nil
			}
			return nil, nil
		}

		// Reconstruct the proof
		reconstructedBytes, err := trie.ReconstructPayloadlessProof(encodedProof, valueReader)
		require.NoError(t, err)

		// Decode the reconstructed proof
		reconstructedProof, err := ledger.DecodeTrieBatchProof(reconstructedBytes)
		require.NoError(t, err)

		// Verify the reconstructed proof has actual values
		require.Equal(t, 1, reconstructedProof.Size())
		proof := reconstructedProof.Proofs[0]
		require.True(t, proof.Inclusion)
		require.Equal(t, ledger.Value(value), proof.Payload.Value())
	})

	t.Run("multiple registers", func(t *testing.T) {
		numRegisters := 5
		registerMap := make(map[flow.RegisterID]flow.RegisterValue)
		paths := make([]ledger.Path, numRegisters)
		payloads := make([]ledger.Payload, numRegisters)

		// Create test data
		for i := 0; i < numRegisters; i++ {
			owner := randomBytes(8)
			key := randomBytes(16)
			value := randomBytes(50 + i*10)

			registerID := flow.NewRegisterID(flow.BytesToAddress(owner), string(key))
			registerMap[registerID] = value

			ledgerKey := convert.RegisterIDToLedgerKey(registerID)
			payload := ledger.NewPayload(ledgerKey, value)

			path, err := pathfinder.KeyToPath(ledgerKey, 1)
			require.NoError(t, err)

			paths[i] = path
			payloads[i] = *payload
		}

		// Create payloadless trie
		payloadlessTrie := trie.NewEmptyMTrieWithPayloadless(true)
		payloadlessTrie, _, err := trie.NewTrieWithUpdatedRegisters(payloadlessTrie, paths, payloads, true)
		require.NoError(t, err)

		// Get proof and encode
		directProofs := payloadlessTrie.UnsafeProofs(paths)
		encodedProof := ledger.EncodeTrieBatchProof(directProofs)

		// Create value reader
		valueReader := func(regID flow.RegisterID) (flow.RegisterValue, error) {
			return registerMap[regID], nil
		}

		// Reconstruct
		reconstructedBytes, err := trie.ReconstructPayloadlessProof(encodedProof, valueReader)
		require.NoError(t, err)

		// Verify
		reconstructedProof, err := ledger.DecodeTrieBatchProof(reconstructedBytes)
		require.NoError(t, err)
		require.Equal(t, numRegisters, reconstructedProof.Size())

		for _, proof := range reconstructedProof.Proofs {
			require.True(t, proof.Inclusion)
			// Value should be larger than hash size
			require.Greater(t, proof.Payload.Value().Size(), hash.HashLen)
		}
	})

	t.Run("consistency verification catches mismatch", func(t *testing.T) {
		// Create test data
		owner := randomBytes(8)
		key := "test_key"
		value := randomBytes(100)

		registerID := flow.NewRegisterID(flow.BytesToAddress(owner), key)
		ledgerKey := convert.RegisterIDToLedgerKey(registerID)
		payload := ledger.NewPayload(ledgerKey, value)

		path, err := pathfinder.KeyToPath(ledgerKey, 1)
		require.NoError(t, err)

		// Create payloadless trie
		payloadlessTrie := trie.NewEmptyMTrieWithPayloadless(true)
		payloadlessTrie, _, err = trie.NewTrieWithUpdatedRegisters(payloadlessTrie, []ledger.Path{path}, []ledger.Payload{*payload}, true)
		require.NoError(t, err)

		// Get proof and encode
		directProofs := payloadlessTrie.UnsafeProofs([]ledger.Path{path})
		encodedProof := ledger.EncodeTrieBatchProof(directProofs)

		// Create value reader that returns WRONG value
		wrongValue := randomBytes(100)
		valueReader := func(regID flow.RegisterID) (flow.RegisterValue, error) {
			if regID == registerID {
				return wrongValue, nil // Return wrong value!
			}
			return nil, nil
		}

		// Reconstruction should fail with hash mismatch
		_, err = trie.ReconstructPayloadlessProof(encodedProof, valueReader)
		require.Error(t, err)
		require.ErrorIs(t, err, trie.ErrPayloadHashMismatch)
	})

	t.Run("non-inclusion proofs pass through unchanged", func(t *testing.T) {
		// Create some existing registers
		owner := randomBytes(8)
		key := "existing_key"
		value := randomBytes(100)

		registerID := flow.NewRegisterID(flow.BytesToAddress(owner), key)
		ledgerKey := convert.RegisterIDToLedgerKey(registerID)
		payload := ledger.NewPayload(ledgerKey, value)

		existingPath, err := pathfinder.KeyToPath(ledgerKey, 1)
		require.NoError(t, err)

		// Create a non-existing path
		nonExistingOwner := randomBytes(8)
		nonExistingKey := "non_existing_key"
		nonExistingRegisterID := flow.NewRegisterID(flow.BytesToAddress(nonExistingOwner), nonExistingKey)
		nonExistingLedgerKey := convert.RegisterIDToLedgerKey(nonExistingRegisterID)
		nonExistingPath, err := pathfinder.KeyToPath(nonExistingLedgerKey, 1)
		require.NoError(t, err)

		// Create payloadless trie with only the existing register
		payloadlessTrie := trie.NewEmptyMTrieWithPayloadless(true)
		payloadlessTrie, _, err = trie.NewTrieWithUpdatedRegisters(payloadlessTrie, []ledger.Path{existingPath}, []ledger.Payload{*payload}, true)
		require.NoError(t, err)

		// Get proof for both paths (one exists, one doesn't)
		allPaths := []ledger.Path{existingPath, nonExistingPath}
		directProofs := payloadlessTrie.UnsafeProofs(allPaths)
		encodedProof := ledger.EncodeTrieBatchProof(directProofs)

		// Create value reader
		valueReader := func(regID flow.RegisterID) (flow.RegisterValue, error) {
			if regID == registerID {
				return value, nil
			}
			return nil, nil
		}

		// Reconstruct
		reconstructedBytes, err := trie.ReconstructPayloadlessProof(encodedProof, valueReader)
		require.NoError(t, err)

		// Verify
		reconstructedProof, err := ledger.DecodeTrieBatchProof(reconstructedBytes)
		require.NoError(t, err)
		require.Equal(t, 2, reconstructedProof.Size())

		// First proof should be inclusion with actual value
		require.True(t, reconstructedProof.Proofs[0].Inclusion)
		require.Equal(t, ledger.Value(value), reconstructedProof.Proofs[0].Payload.Value())

		// Second proof should be non-inclusion
		require.False(t, reconstructedProof.Proofs[1].Inclusion)
	})
}
