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
	"github.com/onflow/flow-go/ledger/partial/ptrie"
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

	t.Run("wrong value reader result returns error", func(t *testing.T) {
		// This test verifies that when the value reader returns a wrong value,
		// reconstruction returns an error instead of silently producing a proof
		// that will fail verification. This helps catch data inconsistencies early.

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

		// Reconstruction should return an error because the wrong value
		// doesn't hash to the stored hash and is not equal to the stored hash
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

// TestPayloadlessProofPSMTReconstruction verifies that a reconstructed payloadless proof
// can be used by PSMT (Partial Sparse Merkle Tree) to verify the correct root hash.
// This is the critical test for verification nodes that use PSMT to verify execution results.
func TestPayloadlessProofPSMTReconstruction(t *testing.T) {
	t.Run("PSMT reconstructs correct root hash from payloadless proof", func(t *testing.T) {
		// Create test data
		owner := randomBytes(8)
		key := "test_key"
		value := randomBytes(100)

		registerID := flow.NewRegisterID(flow.BytesToAddress(owner), key)
		ledgerKey := convert.RegisterIDToLedgerKey(registerID)
		payload := ledger.NewPayload(ledgerKey, value)

		path, err := pathfinder.KeyToPath(ledgerKey, 1)
		require.NoError(t, err)

		// Create a REGULAR (non-payloadless) trie to get the expected root hash
		regularTrie := trie.NewEmptyMTrie()
		regularTrie, _, err = trie.NewTrieWithUpdatedRegisters(regularTrie, []ledger.Path{path}, []ledger.Payload{*payload}, false)
		require.NoError(t, err)
		expectedRootHash := regularTrie.RootHash()

		// Create a PAYLOADLESS trie with the same data
		payloadlessTrie := trie.NewEmptyMTrieWithPayloadless(true)
		payloadlessTrie, _, err = trie.NewTrieWithUpdatedRegisters(payloadlessTrie, []ledger.Path{path}, []ledger.Payload{*payload}, true)
		require.NoError(t, err)

		// CRITICAL: Payloadless trie must have the same root hash as regular trie
		require.Equal(t, expectedRootHash, payloadlessTrie.RootHash(),
			"payloadless trie root hash must match regular trie root hash")

		// Get proof from payloadless trie
		directProofs := payloadlessTrie.UnsafeProofs([]ledger.Path{path})
		encodedProof := ledger.EncodeTrieBatchProof(directProofs)

		// Create value reader that returns actual values
		valueReader := func(regID flow.RegisterID) (flow.RegisterValue, error) {
			if regID == registerID {
				return value, nil
			}
			return nil, nil
		}

		// Reconstruct the proof with actual values
		reconstructedBytes, err := trie.ReconstructPayloadlessProof(encodedProof, valueReader)
		require.NoError(t, err)

		// Decode the reconstructed proof
		reconstructedProof, err := ledger.DecodeTrieBatchProof(reconstructedBytes)
		require.NoError(t, err)

		// CRITICAL TEST: Use PSMT to verify the reconstructed proof produces the correct root hash
		// This is exactly what verification nodes do to verify execution results
		psmt, err := ptrie.NewPSMT(expectedRootHash, reconstructedProof)
		require.NoError(t, err, "PSMT should be able to reconstruct the trie from the proof")
		require.Equal(t, expectedRootHash, psmt.RootHash(),
			"PSMT root hash must match expected root hash")
	})

	t.Run("PSMT reconstructs correct root hash with multiple registers", func(t *testing.T) {
		numRegisters := 10
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

		// Create regular trie to get expected root hash
		regularTrie := trie.NewEmptyMTrie()
		regularTrie, _, err := trie.NewTrieWithUpdatedRegisters(regularTrie, paths, payloads, false)
		require.NoError(t, err)
		expectedRootHash := regularTrie.RootHash()

		// Create payloadless trie
		payloadlessTrie := trie.NewEmptyMTrieWithPayloadless(true)
		payloadlessTrie, _, err = trie.NewTrieWithUpdatedRegisters(payloadlessTrie, paths, payloads, true)
		require.NoError(t, err)

		// Verify root hashes match
		require.Equal(t, expectedRootHash, payloadlessTrie.RootHash())

		// Get proof from payloadless trie
		directProofs := payloadlessTrie.UnsafeProofs(paths)
		encodedProof := ledger.EncodeTrieBatchProof(directProofs)

		// Create value reader
		valueReader := func(regID flow.RegisterID) (flow.RegisterValue, error) {
			return registerMap[regID], nil
		}

		// Reconstruct the proof
		reconstructedBytes, err := trie.ReconstructPayloadlessProof(encodedProof, valueReader)
		require.NoError(t, err)

		// Decode the reconstructed proof
		reconstructedProof, err := ledger.DecodeTrieBatchProof(reconstructedBytes)
		require.NoError(t, err)

		// Verify PSMT can reconstruct the correct root hash
		psmt, err := ptrie.NewPSMT(expectedRootHash, reconstructedProof)
		require.NoError(t, err, "PSMT should be able to reconstruct the trie from the proof")
		require.Equal(t, expectedRootHash, psmt.RootHash())
	})

	t.Run("root hash consistency between regular and payloadless tries", func(t *testing.T) {
		// This test verifies the fundamental assumption that payloadless tries
		// have the same root hash as regular tries with the same data.
		// If this fails, the entire payloadless mode is broken.

		for i := 0; i < 5; i++ {
			// Create random test data
			numRegisters := 1 + i*3 // 1, 4, 7, 10, 13 registers
			paths := make([]ledger.Path, numRegisters)
			payloads := make([]ledger.Payload, numRegisters)

			for j := 0; j < numRegisters; j++ {
				owner := randomBytes(8)
				key := randomBytes(16)
				value := randomBytes(50 + j*10)

				ledgerKey := convert.RegisterIDToLedgerKey(
					flow.NewRegisterID(flow.BytesToAddress(owner), string(key)))
				payload := ledger.NewPayload(ledgerKey, value)

				path, err := pathfinder.KeyToPath(ledgerKey, 1)
				require.NoError(t, err)

				paths[j] = path
				payloads[j] = *payload
			}

			// Create regular trie
			regularTrie := trie.NewEmptyMTrie()
			regularTrie, _, err := trie.NewTrieWithUpdatedRegisters(regularTrie, paths, payloads, false)
			require.NoError(t, err)

			// Create payloadless trie
			payloadlessTrie := trie.NewEmptyMTrieWithPayloadless(true)
			payloadlessTrie, _, err = trie.NewTrieWithUpdatedRegisters(payloadlessTrie, paths, payloads, true)
			require.NoError(t, err)

			// Root hashes MUST match
			require.Equal(t, regularTrie.RootHash(), payloadlessTrie.RootHash(),
				"root hash mismatch with %d registers", numRegisters)
		}
	})

	t.Run("root hash consistency after single update to existing trie", func(t *testing.T) {
		// This is a minimal test to isolate the root hash mismatch bug.
		// We start with one register, then add a second register.

		// First register
		owner1 := randomBytes(8)
		key1 := randomBytes(16)
		value1 := randomBytes(100)
		registerID1 := flow.NewRegisterID(flow.BytesToAddress(owner1), string(key1))
		ledgerKey1 := convert.RegisterIDToLedgerKey(registerID1)
		payload1 := ledger.NewPayload(ledgerKey1, value1)
		path1, err := pathfinder.KeyToPath(ledgerKey1, 1)
		require.NoError(t, err)

		// Create initial tries (both regular and payloadless)
		regularTrie := trie.NewEmptyMTrie()
		regularTrie, _, err = trie.NewTrieWithUpdatedRegisters(regularTrie, []ledger.Path{path1}, []ledger.Payload{*payload1}, false)
		require.NoError(t, err)

		payloadlessTrie := trie.NewEmptyMTrieWithPayloadless(true)
		payloadlessTrie, _, err = trie.NewTrieWithUpdatedRegisters(payloadlessTrie, []ledger.Path{path1}, []ledger.Payload{*payload1}, true)
		require.NoError(t, err)

		// Verify initial root hashes match
		require.Equal(t, regularTrie.RootHash(), payloadlessTrie.RootHash(), "initial root hash mismatch")
		rh0 := regularTrie.RootHash()
		ph0 := payloadlessTrie.RootHash()
		t.Logf("After round 0: regular=%x, payloadless=%x", rh0[:8], ph0[:8])

		// Second register (different path)
		owner2 := randomBytes(8)
		key2 := randomBytes(16)
		value2 := randomBytes(100)
		registerID2 := flow.NewRegisterID(flow.BytesToAddress(owner2), string(key2))
		ledgerKey2 := convert.RegisterIDToLedgerKey(registerID2)
		payload2 := ledger.NewPayload(ledgerKey2, value2)
		path2, err := pathfinder.KeyToPath(ledgerKey2, 1)
		require.NoError(t, err)

		// Update both tries with second register
		regularTrie, _, err = trie.NewTrieWithUpdatedRegisters(regularTrie, []ledger.Path{path2}, []ledger.Payload{*payload2}, false)
		require.NoError(t, err)

		payloadlessTrie, _, err = trie.NewTrieWithUpdatedRegisters(payloadlessTrie, []ledger.Path{path2}, []ledger.Payload{*payload2}, true)
		require.NoError(t, err)

		// Verify root hashes still match after second update
		rh1 := regularTrie.RootHash()
		ph1 := payloadlessTrie.RootHash()
		t.Logf("After round 1: regular=%x, payloadless=%x", rh1[:8], ph1[:8])
		require.Equal(t, regularTrie.RootHash(), payloadlessTrie.RootHash(), "root hash mismatch after adding second register")
	})

	t.Run("PSMT works after multiple updates to payloadless trie", func(t *testing.T) {
		// This test simulates what happens in production:
		// 1. Start with a payloadless trie
		// 2. Apply multiple updates
		// 3. Generate proofs
		// 4. Verify PSMT can reconstruct correct root hash

		// Track all register values for the value reader
		registerMap := make(map[flow.RegisterID]flow.RegisterValue)

		// Start with empty tries
		regularTrie := trie.NewEmptyMTrie()
		payloadlessTrie := trie.NewEmptyMTrieWithPayloadless(true)

		// Apply 3 rounds of updates
		for round := 0; round < 3; round++ {
			// Create updates for this round
			numUpdates := 5
			paths := make([]ledger.Path, numUpdates)
			payloads := make([]ledger.Payload, numUpdates)

			for i := 0; i < numUpdates; i++ {
				owner := randomBytes(8)
				key := randomBytes(16)
				value := randomBytes(50 + round*10 + i*5)

				registerID := flow.NewRegisterID(flow.BytesToAddress(owner), string(key))
				registerMap[registerID] = value

				ledgerKey := convert.RegisterIDToLedgerKey(registerID)
				payload := ledger.NewPayload(ledgerKey, value)

				path, err := pathfinder.KeyToPath(ledgerKey, 1)
				require.NoError(t, err)

				paths[i] = path
				payloads[i] = *payload
			}

			// Apply updates to both tries
			var err error
			regularTrie, _, err = trie.NewTrieWithUpdatedRegisters(regularTrie, paths, payloads, false)
			require.NoError(t, err)

			payloadlessTrie, _, err = trie.NewTrieWithUpdatedRegisters(payloadlessTrie, paths, payloads, true)
			require.NoError(t, err)

			// Verify root hashes match after each update
			require.Equal(t, regularTrie.RootHash(), payloadlessTrie.RootHash(),
				"root hash mismatch after round %d", round)
		}

		// Collect all paths for proof generation
		var allPaths []ledger.Path
		for regID := range registerMap {
			ledgerKey := convert.RegisterIDToLedgerKey(regID)
			path, err := pathfinder.KeyToPath(ledgerKey, 1)
			require.NoError(t, err)
			allPaths = append(allPaths, path)
		}

		expectedRootHash := payloadlessTrie.RootHash()

		// Generate proof from payloadless trie
		directProofs := payloadlessTrie.UnsafeProofs(allPaths)
		encodedProof := ledger.EncodeTrieBatchProof(directProofs)

		// Create value reader
		valueReader := func(regID flow.RegisterID) (flow.RegisterValue, error) {
			return registerMap[regID], nil
		}

		// Reconstruct the proof
		reconstructedBytes, err := trie.ReconstructPayloadlessProof(encodedProof, valueReader)
		require.NoError(t, err)

		// Decode the reconstructed proof
		reconstructedProof, err := ledger.DecodeTrieBatchProof(reconstructedBytes)
		require.NoError(t, err)

		// Verify PSMT can reconstruct the correct root hash
		psmt, err := ptrie.NewPSMT(expectedRootHash, reconstructedProof)
		require.NoError(t, err, "PSMT should be able to reconstruct the trie from the proof after multiple updates")
		require.Equal(t, expectedRootHash, psmt.RootHash())
	})

	t.Run("stress test: many consecutive updates with proof verification", func(t *testing.T) {
		// This test simulates production-like conditions:
		// - Many consecutive updates (simulating ~100+ blocks)
		// - Verify root hash consistency at every step
		// - Verify proof reconstruction works at every step
		// This should help catch edge cases in trie compaction/update logic.

		// Track all register values for the value reader
		registerMap := make(map[flow.RegisterID]flow.RegisterValue)

		// Start with empty tries
		regularTrie := trie.NewEmptyMTrie()
		payloadlessTrie := trie.NewEmptyMTrieWithPayloadless(true)

		// Track all paths for final verification
		allPaths := make([]ledger.Path, 0)

		// Simulate 50 rounds of updates (each round = 1 block with multiple register updates)
		numRounds := 50
		for round := 0; round < numRounds; round++ {
			// Vary the number of updates per round
			numUpdates := 1 + (round % 5) // 1-5 updates per round
			paths := make([]ledger.Path, numUpdates)
			payloads := make([]ledger.Payload, numUpdates)

			for i := 0; i < numUpdates; i++ {
				owner := randomBytes(8)
				key := randomBytes(16)
				value := randomBytes(20 + (round%10)*10 + i*5) // Vary value size

				registerID := flow.NewRegisterID(flow.BytesToAddress(owner), string(key))
				registerMap[registerID] = value

				ledgerKey := convert.RegisterIDToLedgerKey(registerID)
				payload := ledger.NewPayload(ledgerKey, value)

				path, err := pathfinder.KeyToPath(ledgerKey, 1)
				require.NoError(t, err)

				paths[i] = path
				payloads[i] = *payload
				allPaths = append(allPaths, path)
			}

			// Apply updates to both tries (use same prune setting for both!)
			var err error
			regularTrie, _, err = trie.NewTrieWithUpdatedRegisters(regularTrie, paths, payloads, true)
			require.NoError(t, err)

			payloadlessTrie, _, err = trie.NewTrieWithUpdatedRegisters(payloadlessTrie, paths, payloads, true)
			require.NoError(t, err)

			// Verify root hashes match after each update
			regularRH := regularTrie.RootHash()
			payloadlessRH := payloadlessTrie.RootHash()
			if regularRH != payloadlessRH {
				t.Fatalf("root hash mismatch at round %d: regular=%x, payloadless=%x",
					round, regularRH[:8], payloadlessRH[:8])
			}

			// Every 10 rounds, verify proof reconstruction works
			if round > 0 && round%10 == 0 {
				// Sample a subset of paths for verification
				samplePaths := allPaths
				if len(samplePaths) > 20 {
					samplePaths = allPaths[:20]
				}

				expectedRootHash := payloadlessTrie.RootHash()
				directProofs := payloadlessTrie.UnsafeProofs(samplePaths)
				encodedProof := ledger.EncodeTrieBatchProof(directProofs)

				valueReader := func(regID flow.RegisterID) (flow.RegisterValue, error) {
					return registerMap[regID], nil
				}

				reconstructedBytes, err := trie.ReconstructPayloadlessProof(encodedProof, valueReader)
				require.NoError(t, err, "proof reconstruction failed at round %d", round)

				reconstructedProof, err := ledger.DecodeTrieBatchProof(reconstructedBytes)
				require.NoError(t, err)

				psmt, err := ptrie.NewPSMT(expectedRootHash, reconstructedProof)
				if err != nil {
					t.Fatalf("PSMT construction failed at round %d: %v", round, err)
				}
				require.Equal(t, expectedRootHash, psmt.RootHash(),
					"PSMT root hash mismatch at round %d", round)
			}
		}

		t.Logf("Stress test passed: %d rounds, %d total registers", numRounds, len(registerMap))
	})

	t.Run("stress test: updates to existing registers", func(t *testing.T) {
		// This test simulates updating existing registers (common in production).
		// Existing registers are updated when the same key is written multiple times.

		// Track all register values and paths
		type registerInfo struct {
			path    ledger.Path
			payload *ledger.Payload
		}
		registerInfos := make(map[flow.RegisterID]registerInfo)
		registerValues := make(map[flow.RegisterID]flow.RegisterValue)

		// Start with empty tries
		regularTrie := trie.NewEmptyMTrie()
		payloadlessTrie := trie.NewEmptyMTrieWithPayloadless(true)

		// Create a fixed set of register IDs that will be updated multiple times
		numFixedRegisters := 10
		fixedRegisterIDs := make([]flow.RegisterID, numFixedRegisters)
		for i := 0; i < numFixedRegisters; i++ {
			owner := randomBytes(8)
			key := randomBytes(16)
			fixedRegisterIDs[i] = flow.NewRegisterID(flow.BytesToAddress(owner), string(key))

			ledgerKey := convert.RegisterIDToLedgerKey(fixedRegisterIDs[i])
			path, err := pathfinder.KeyToPath(ledgerKey, 1)
			require.NoError(t, err)

			// Initialize with some value
			value := randomBytes(50)
			registerValues[fixedRegisterIDs[i]] = value
			payload := ledger.NewPayload(ledgerKey, value)
			registerInfos[fixedRegisterIDs[i]] = registerInfo{path: path, payload: payload}
		}

		// Initial update with all registers
		paths := make([]ledger.Path, numFixedRegisters)
		payloads := make([]ledger.Payload, numFixedRegisters)
		for i, regID := range fixedRegisterIDs {
			info := registerInfos[regID]
			paths[i] = info.path
			payloads[i] = *info.payload
		}

		var err error
		regularTrie, _, err = trie.NewTrieWithUpdatedRegisters(regularTrie, paths, payloads, true)
		require.NoError(t, err)
		payloadlessTrie, _, err = trie.NewTrieWithUpdatedRegisters(payloadlessTrie, paths, payloads, true)
		require.NoError(t, err)
		require.Equal(t, regularTrie.RootHash(), payloadlessTrie.RootHash(), "initial root hash mismatch")

		// Now update registers multiple times
		numRounds := 30
		for round := 0; round < numRounds; round++ {
			// Select a subset of registers to update (1-5 registers per round)
			numUpdates := 1 + (round % 5)
			updatePaths := make([]ledger.Path, numUpdates)
			updatePayloads := make([]ledger.Payload, numUpdates)

			for i := 0; i < numUpdates; i++ {
				// Pick a register to update (cycling through the fixed set)
				regID := fixedRegisterIDs[(round*numUpdates+i)%numFixedRegisters]
				info := registerInfos[regID]

				// Create a new value
				newValue := randomBytes(30 + round + i*5)
				registerValues[regID] = newValue

				ledgerKey := convert.RegisterIDToLedgerKey(regID)
				payload := ledger.NewPayload(ledgerKey, newValue)

				updatePaths[i] = info.path
				updatePayloads[i] = *payload
			}

			// Apply updates
			regularTrie, _, err = trie.NewTrieWithUpdatedRegisters(regularTrie, updatePaths, updatePayloads, true)
			require.NoError(t, err)
			payloadlessTrie, _, err = trie.NewTrieWithUpdatedRegisters(payloadlessTrie, updatePaths, updatePayloads, true)
			require.NoError(t, err)

			// Verify root hashes match
			regularRH := regularTrie.RootHash()
			payloadlessRH := payloadlessTrie.RootHash()
			if regularRH != payloadlessRH {
				t.Fatalf("root hash mismatch at round %d: regular=%x, payloadless=%x",
					round, regularRH[:8], payloadlessRH[:8])
			}

			// Verify proof reconstruction every 5 rounds
			if round > 0 && round%5 == 0 {
				// Collect all paths
				allPaths := make([]ledger.Path, 0, len(registerInfos))
				for _, info := range registerInfos {
					allPaths = append(allPaths, info.path)
				}

				expectedRootHash := payloadlessTrie.RootHash()
				directProofs := payloadlessTrie.UnsafeProofs(allPaths)
				encodedProof := ledger.EncodeTrieBatchProof(directProofs)

				valueReader := func(regID flow.RegisterID) (flow.RegisterValue, error) {
					return registerValues[regID], nil
				}

				reconstructedBytes, err := trie.ReconstructPayloadlessProof(encodedProof, valueReader)
				require.NoError(t, err, "proof reconstruction failed at round %d", round)

				reconstructedProof, err := ledger.DecodeTrieBatchProof(reconstructedBytes)
				require.NoError(t, err)

				psmt, err := ptrie.NewPSMT(expectedRootHash, reconstructedProof)
				if err != nil {
					t.Fatalf("PSMT construction failed at round %d: %v", round, err)
				}
				require.Equal(t, expectedRootHash, psmt.RootHash(),
					"PSMT root hash mismatch at round %d", round)
			}
		}

		t.Logf("Update test passed: %d rounds, %d registers updated multiple times", numRounds, numFixedRegisters)
	})

	t.Run("test proof reconstruction with serialized/deserialized trie nodes", func(t *testing.T) {
		// This test simulates what happens when a trie is loaded from a checkpoint:
		// 1. Create a payloadless trie
		// 2. Get the encoded node data (simulating checkpoint save)
		// 3. Create a new trie from the encoded data (simulating checkpoint load)
		// 4. Generate proofs and verify PSMT reconstruction

		// Track all register values
		registerMap := make(map[flow.RegisterID]flow.RegisterValue)

		// Create initial registers
		numRegisters := 10
		paths := make([]ledger.Path, numRegisters)
		payloads := make([]ledger.Payload, numRegisters)

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

		// Create regular trie for comparison
		regularTrie := trie.NewEmptyMTrie()
		regularTrie, _, err = trie.NewTrieWithUpdatedRegisters(regularTrie, paths, payloads, true)
		require.NoError(t, err)

		// Verify initial root hashes match
		require.Equal(t, regularTrie.RootHash(), payloadlessTrie.RootHash(), "initial root hash mismatch")

		originalRootHash := payloadlessTrie.RootHash()

		// Generate proof from payloadless trie
		directProofs := payloadlessTrie.UnsafeProofs(paths)
		encodedProof := ledger.EncodeTrieBatchProof(directProofs)

		// Create value reader
		valueReader := func(regID flow.RegisterID) (flow.RegisterValue, error) {
			return registerMap[regID], nil
		}

		// Reconstruct the proof
		reconstructedBytes, err := trie.ReconstructPayloadlessProof(encodedProof, valueReader)
		require.NoError(t, err, "proof reconstruction failed")

		// Decode the reconstructed proof
		reconstructedProof, err := ledger.DecodeTrieBatchProof(reconstructedBytes)
		require.NoError(t, err)

		// Verify PSMT can reconstruct the correct root hash
		psmt, err := ptrie.NewPSMT(originalRootHash, reconstructedProof)
		require.NoError(t, err, "PSMT should be able to construct from reconstructed proof")
		require.Equal(t, originalRootHash, psmt.RootHash(), "PSMT root hash should match original")

		t.Logf("Serialization test passed with %d registers", numRegisters)
	})

	t.Run("PSMT with random subset of registers - production scenario", func(t *testing.T) {
		// This test simulates the exact production scenario:
		// - EN has many registers in the trie
		// - Proofs are generated for only a RANDOM SUBSET (touched during execution)
		// - VN builds PSMT from this subset and verifies root hash
		//
		// This is different from other tests that use ALL registers or first N registers.

		// Track all register values
		type regInfo struct {
			id    flow.RegisterID
			path  ledger.Path
			value flow.RegisterValue
		}
		allRegisters := make([]regInfo, 0, 100)

		// Create 100 registers (simulating a realistic trie size)
		paths := make([]ledger.Path, 100)
		payloads := make([]ledger.Payload, 100)

		for i := 0; i < 100; i++ {
			owner := randomBytes(8)
			key := randomBytes(16)
			value := randomBytes(50 + (i % 50))

			registerID := flow.NewRegisterID(flow.BytesToAddress(owner), string(key))
			ledgerKey := convert.RegisterIDToLedgerKey(registerID)
			payload := ledger.NewPayload(ledgerKey, value)
			path, err := pathfinder.KeyToPath(ledgerKey, 1)
			require.NoError(t, err)

			paths[i] = path
			payloads[i] = *payload
			allRegisters = append(allRegisters, regInfo{
				id:    registerID,
				path:  path,
				value: value,
			})
		}

		// Create payloadless trie
		payloadlessTrie := trie.NewEmptyMTrieWithPayloadless(true)
		payloadlessTrie, _, err := trie.NewTrieWithUpdatedRegisters(payloadlessTrie, paths, payloads, true)
		require.NoError(t, err)

		// Create regular trie for comparison
		regularTrie := trie.NewEmptyMTrie()
		regularTrie, _, err = trie.NewTrieWithUpdatedRegisters(regularTrie, paths, payloads, true)
		require.NoError(t, err)

		// Verify root hashes match
		expectedRootHash := regularTrie.RootHash()
		require.Equal(t, expectedRootHash, payloadlessTrie.RootHash(), "root hash mismatch")

		// Now test with random subsets of various sizes (simulating different chunk sizes)
		subsetSizes := []int{1, 5, 10, 20, 50, 75, 99}
		for _, subsetSize := range subsetSizes {
			// Generate a random subset by shuffling indices
			indices := make([]int, len(allRegisters))
			for i := range indices {
				indices[i] = i
			}
			// Simple shuffle using the random bytes we already have
			for i := len(indices) - 1; i > 0; i-- {
				j := int(randomBytes(1)[0]) % (i + 1)
				indices[i], indices[j] = indices[j], indices[i]
			}
			selectedIndices := indices[:subsetSize]

			// Collect paths and create value reader for selected registers
			selectedPaths := make([]ledger.Path, subsetSize)
			registerMap := make(map[flow.RegisterID]flow.RegisterValue)
			for i, idx := range selectedIndices {
				reg := allRegisters[idx]
				selectedPaths[i] = reg.path
				registerMap[reg.id] = reg.value
			}

			// Generate proofs for ONLY the selected subset
			directProofs := payloadlessTrie.UnsafeProofs(selectedPaths)
			encodedProof := ledger.EncodeTrieBatchProof(directProofs)

			// Create value reader for the subset
			valueReader := func(regID flow.RegisterID) (flow.RegisterValue, error) {
				return registerMap[regID], nil
			}

			// Reconstruct the proof
			reconstructedBytes, err := trie.ReconstructPayloadlessProof(encodedProof, valueReader)
			require.NoError(t, err, "proof reconstruction failed for subset size %d", subsetSize)

			// Decode the reconstructed proof
			reconstructedProof, err := ledger.DecodeTrieBatchProof(reconstructedBytes)
			require.NoError(t, err)

			// Verify PSMT can reconstruct the correct root hash
			psmt, err := ptrie.NewPSMT(expectedRootHash, reconstructedProof)
			require.NoError(t, err, "PSMT construction failed for subset size %d", subsetSize)
			require.Equal(t, expectedRootHash, psmt.RootHash(),
				"PSMT root hash mismatch for subset size %d: expected %s, got %s",
				subsetSize, expectedRootHash, psmt.RootHash())
		}

		t.Logf("Random subset test passed: tested subsets of sizes %v out of %d total registers",
			subsetSizes, len(allRegisters))
	})
}
