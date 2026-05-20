package committer_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	ledgermock "github.com/onflow/flow-go/ledger/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestLedgerViewCommitter(t *testing.T) {

	// verify after committing a snapshot, proof will be generated,
	// and changes are saved in storage snapshot
	t.Run("CommitView should return proof and statecommitment", func(t *testing.T) {

		l := ledgermock.NewLedger(t)
		committer := committer.NewLedgerViewCommitter(l, trace.NewNoopTracer())

		// CommitDelta will call ledger.Set and ledger.Prove

		reg := unittest.MakeOwnerReg("key1", "val1")
		startState := unittest.StateCommitmentFixture()

		update, err := ledger.NewUpdate(ledger.State(startState), []ledger.Key{convert.RegisterIDToLedgerKey(reg.Key)}, []ledger.Value{reg.Value})
		require.NoError(t, err)

		expectedTrieUpdate, err := pathfinder.UpdateToTrieUpdate(update, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		endState := unittest.StateCommitmentFixture()
		require.NotEqual(t, startState, endState)

		// mock ledger.Set
		l.On("Set", mock.Anything).
			Return(func(update *ledger.Update) (newState ledger.State, trieUpdate *ledger.TrieUpdate, err error) {
				if update.State().Equals(ledger.State(startState)) {
					return ledger.State(endState), expectedTrieUpdate, nil
				}
				return ledger.DummyState, nil, fmt.Errorf("wrong update")
			}).
			Once()

			// mock ledger.Prove
		expectedProof := ledger.Proof([]byte{2, 3, 4})
		l.On("Prove", mock.Anything).
			Return(func(query *ledger.Query) (proof ledger.Proof, err error) {
				if query.Size() != 1 {
					return nil, fmt.Errorf("wrong query size: %v", query.Size())
				}

				k := convert.RegisterIDToLedgerKey(reg.Key)
				if !query.Keys()[0].Equals(&k) {
					return nil, fmt.Errorf("in correct query key for prove: %v", query.Keys()[0])
				}

				return expectedProof, nil
			}).
			Once()

			// previous block's storage snapshot
		oldReg := unittest.MakeOwnerReg("key1", "oldvalue")
		previousBlockSnapshot := storehouse.NewExecutingBlockSnapshot(
			snapshot.MapStorageSnapshot{
				oldReg.Key: oldReg.Value,
			},
			flow.StateCommitment(update.State()),
		)

		// this block's register updates
		blockUpdates := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				reg.Key: oldReg.Value,
			},
		}

		newCommit, proof, trieUpdate, newStorageSnapshot, err := committer.CommitView(
			blockUpdates,
			previousBlockSnapshot,
		)

		require.NoError(t, err)

		// verify CommitView returns expected proof and statecommitment
		require.Equal(t, previousBlockSnapshot.Commitment(), flow.StateCommitment(trieUpdate.RootHash))
		require.Equal(t, newCommit, newStorageSnapshot.Commitment())
		require.Equal(t, endState, newCommit)
		require.Equal(t, []uint8(expectedProof), proof)
		require.True(t, expectedTrieUpdate.Equals(trieUpdate))

	})

}

func TestPayloadlessLedgerViewCommitter(t *testing.T) {

	t.Run("payloadless committer reconstructs proof with actual values", func(t *testing.T) {
		// Create test register with proper address format for round-trip conversion
		owner := flow.BytesToAddress([]byte("owner"))
		key := "key1"
		value := []byte("test_value_for_payloadless")

		// Create register ID that will round-trip correctly through ledger key conversion
		registerID := flow.NewRegisterID(owner, key)
		ledgerKey := convert.RegisterIDToLedgerKey(registerID)

		// Verify round-trip works
		roundTrippedID, err := convert.LedgerKeyToRegisterID(ledgerKey)
		require.NoError(t, err)
		require.Equal(t, registerID, roundTrippedID)

		path, err := pathfinder.KeyToPath(ledgerKey, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		payload := ledger.NewPayload(ledgerKey, value)

		// Create a payloadless trie and generate a proof from it
		payloadlessTrie := trie.NewEmptyMTrieWithPayloadless(true)
		payloadlessTrie, _, err = trie.NewTrieWithUpdatedRegisters(
			payloadlessTrie,
			[]ledger.Path{path},
			[]ledger.Payload{*payload},
			true,
		)
		require.NoError(t, err)

		// Get proof from payloadless trie (contains hash values, not actual values)
		payloadlessProofs := payloadlessTrie.UnsafeProofs([]ledger.Path{path})
		encodedPayloadlessProof := ledger.EncodeTrieBatchProof(payloadlessProofs)

		// Verify the payloadless proof contains hash (32 bytes), not actual value
		require.Equal(t, 1, payloadlessProofs.Size())
		require.Equal(t, hash.HashLen, payloadlessProofs.Proofs[0].Payload.Value().Size())

		// Create mock ledger that returns the payloadless proof
		l := ledgermock.NewLedger(t)

		startState := unittest.StateCommitmentFixture()
		endState := unittest.StateCommitmentFixture()
		require.NotEqual(t, startState, endState)

		update, err := ledger.NewUpdate(ledger.State(startState), []ledger.Key{ledgerKey}, []ledger.Value{value})
		require.NoError(t, err)

		expectedTrieUpdate, err := pathfinder.UpdateToTrieUpdate(update, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		// Mock ledger.Set
		l.On("Set", mock.Anything).
			Return(func(update *ledger.Update) (newState ledger.State, trieUpdate *ledger.TrieUpdate, err error) {
				if update.State().Equals(ledger.State(startState)) {
					return ledger.State(endState), expectedTrieUpdate, nil
				}
				return ledger.DummyState, nil, fmt.Errorf("wrong update")
			}).
			Once()

		// Mock ledger.Prove to return the payloadless proof (with hash values)
		l.On("Prove", mock.Anything).
			Return(func(query *ledger.Query) (proof ledger.Proof, err error) {
				return encodedPayloadlessProof, nil
			}).
			Once()

		// Create payloadless committer
		payloadlessCommitter := committer.NewPayloadlessLedgerViewCommitter(l, trace.NewNoopTracer())

		// Create storage snapshot with actual values (simulating storehouse)
		// Use the same registerID that will be derived from the proof
		previousBlockSnapshot := storehouse.NewExecutingBlockSnapshot(
			snapshot.MapStorageSnapshot{
				registerID: value, // Actual value available in storehouse
			},
			flow.StateCommitment(update.State()),
		)

		// Block updates
		blockUpdates := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				registerID: value,
			},
		}

		// Commit the view
		newCommit, proof, trieUpdate, newStorageSnapshot, err := payloadlessCommitter.CommitView(
			blockUpdates,
			previousBlockSnapshot,
		)
		require.NoError(t, err)

		// Verify basic results
		require.Equal(t, endState, newCommit)
		require.Equal(t, newCommit, newStorageSnapshot.Commitment())
		require.True(t, expectedTrieUpdate.Equals(trieUpdate))

		// Decode the returned proof and verify it contains actual values (not hashes)
		decodedProof, err := ledger.DecodeTrieBatchProof(proof)
		require.NoError(t, err)
		require.Equal(t, 1, decodedProof.Size())

		// The reconstructed proof should have the actual value, not the hash
		actualValue := decodedProof.Proofs[0].Payload.Value()
		require.Equal(t, value, []byte(actualValue), "proof should contain actual value, not hash")
		require.NotEqual(t, hash.HashLen, actualValue.Size(), "proof value should not be hash length")
	})

	t.Run("payloadless committer reconstructs proof from baseStorageSnapshot", func(t *testing.T) {
		// This test verifies that proof reconstruction uses values from baseStorageSnapshot.
		// The proof is generated from baseState (pre-execution), so we need pre-execution values.

		// Create test register with proper address format
		owner := flow.BytesToAddress([]byte("owner"))
		key := "key1"
		originalValue := []byte("original_value")

		registerID := flow.NewRegisterID(owner, key)
		ledgerKey := convert.RegisterIDToLedgerKey(registerID)

		path, err := pathfinder.KeyToPath(ledgerKey, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		payload := ledger.NewPayload(ledgerKey, originalValue)

		// Create payloadless trie with original value
		payloadlessTrie := trie.NewEmptyMTrieWithPayloadless(true)
		payloadlessTrie, _, err = trie.NewTrieWithUpdatedRegisters(
			payloadlessTrie,
			[]ledger.Path{path},
			[]ledger.Payload{*payload},
			true,
		)
		require.NoError(t, err)

		// Get proof from payloadless trie
		payloadlessProofs := payloadlessTrie.UnsafeProofs([]ledger.Path{path})
		encodedPayloadlessProof := ledger.EncodeTrieBatchProof(payloadlessProofs)

		// Create mock ledger
		l := ledgermock.NewLedger(t)

		startState := unittest.StateCommitmentFixture()
		endState := unittest.StateCommitmentFixture()

		update, err := ledger.NewUpdate(ledger.State(startState), []ledger.Key{ledgerKey}, []ledger.Value{originalValue})
		require.NoError(t, err)

		expectedTrieUpdate, err := pathfinder.UpdateToTrieUpdate(update, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		l.On("Set", mock.Anything).
			Return(func(update *ledger.Update) (newState ledger.State, trieUpdate *ledger.TrieUpdate, err error) {
				return ledger.State(endState), expectedTrieUpdate, nil
			}).
			Once()

		l.On("Prove", mock.Anything).
			Return(func(query *ledger.Query) (proof ledger.Proof, err error) {
				return encodedPayloadlessProof, nil
			}).
			Once()

		payloadlessCommitter := committer.NewPayloadlessLedgerViewCommitter(l, trace.NewNoopTracer())

		// baseStorageSnapshot has the correct value for reconstruction
		// (this is the value that was used to create the hash in the payloadless trie)
		previousBlockSnapshot := storehouse.NewExecutingBlockSnapshot(
			snapshot.MapStorageSnapshot{
				registerID: originalValue, // Correct value from base state
			},
			flow.StateCommitment(update.State()),
		)

		// The WriteSet might have a different (new) value, but this shouldn't affect
		// proof reconstruction since we use baseStorageSnapshot
		newValue := []byte("new_value_being_written")
		blockUpdates := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				registerID: newValue,
			},
		}

		// Commit should succeed
		newCommit, proof, trieUpdate, newStorageSnapshot, err := payloadlessCommitter.CommitView(
			blockUpdates,
			previousBlockSnapshot,
		)
		require.NoError(t, err)

		// Verify basic results
		require.Equal(t, endState, newCommit)
		require.Equal(t, newCommit, newStorageSnapshot.Commitment())
		require.True(t, expectedTrieUpdate.Equals(trieUpdate))

		// The proof should be reconstructed with actual value from baseStorageSnapshot
		decodedProof, err := ledger.DecodeTrieBatchProof(proof)
		require.NoError(t, err)
		require.Equal(t, 1, decodedProof.Size())

		// The payload should have the original value (from baseStorageSnapshot), not the new value
		proofValue := decodedProof.Proofs[0].Payload.Value()
		require.Equal(t, originalValue, []byte(proofValue),
			"proof should contain actual value from baseStorageSnapshot")
	})

	t.Run("non-payloadless committer returns proof as-is", func(t *testing.T) {
		l := ledgermock.NewLedger(t)
		regularCommitter := committer.NewLedgerViewCommitter(l, trace.NewNoopTracer())

		reg := unittest.MakeOwnerReg("key1", "val1")
		startState := unittest.StateCommitmentFixture()

		update, err := ledger.NewUpdate(ledger.State(startState), []ledger.Key{convert.RegisterIDToLedgerKey(reg.Key)}, []ledger.Value{reg.Value})
		require.NoError(t, err)

		expectedTrieUpdate, err := pathfinder.UpdateToTrieUpdate(update, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		endState := unittest.StateCommitmentFixture()

		l.On("Set", mock.Anything).
			Return(func(update *ledger.Update) (newState ledger.State, trieUpdate *ledger.TrieUpdate, err error) {
				if update.State().Equals(ledger.State(startState)) {
					return ledger.State(endState), expectedTrieUpdate, nil
				}
				return ledger.DummyState, nil, fmt.Errorf("wrong update")
			}).
			Once()

		// Return a simple proof (not reconstructed)
		expectedProof := ledger.Proof([]byte{2, 3, 4})
		l.On("Prove", mock.Anything).
			Return(func(query *ledger.Query) (proof ledger.Proof, err error) {
				return expectedProof, nil
			}).
			Once()

		previousBlockSnapshot := storehouse.NewExecutingBlockSnapshot(
			snapshot.MapStorageSnapshot{
				reg.Key: reg.Value,
			},
			flow.StateCommitment(update.State()),
		)

		blockUpdates := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				reg.Key: reg.Value,
			},
		}

		_, proof, _, _, err := regularCommitter.CommitView(
			blockUpdates,
			previousBlockSnapshot,
		)

		require.NoError(t, err)
		// Non-payloadless committer returns proof exactly as received from ledger
		require.Equal(t, []byte(expectedProof), proof)
	})
}
