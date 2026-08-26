// Package compact_execution_state provides the compact-execution-state utility command.
// This file contains the end-to-end integration test for the compaction pipeline:
//
//  1. Build a real ledger WAL with multiple trie updates using register-format keys.
//  2. Compact the WAL using the shared helpers (trim + checkpoint extraction).
//  3. Bootstrap a register store from the compacted checkpoint (SealedCheckpointSource path).
//  4. Verify the register store reflects the sealed state.
package compact_execution_state_test

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	utilledger "github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	mtrietrie "github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	flowWAL "github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/pebble"
	storagepebble "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
)

// makeRegisterKey returns a ledger key in register format (owner + key parts) so that
// IndexCheckpointFile can convert it to a flow.RegisterID.
func makeRegisterKey(n uint16) ledger.Key {
	owner := make([]byte, 8)
	binary.BigEndian.PutUint16(owner, n)
	k := make([]byte, 8)
	binary.BigEndian.PutUint16(k, n)
	return ledger.Key{KeyParts: []ledger.KeyPart{
		{Type: ledger.KeyPartOwner, Value: owner},
		{Type: ledger.KeyPartKey, Value: k},
	}}
}

// TestCompactionAndStorehouseBootstrap is the end-to-end integration test for the
// compact-execution-state pipeline.  It exercises:
//
//  1. Building a real ledger WAL with multiple trie updates using register-format keys.
//  2. Locating the target root hash in the WAL via backward scan.
//  3. Trimming the WAL at the target root hash.
//  4. Extracting a single-trie V6 checkpoint.
//  5. Verifying the checkpoint invariants (exactly 1 trie, correct root hash).
//  6. Bootstrapping a register store (Pebble DB) from the checkpoint.
//  7. Querying the register store to verify the sealed state is accessible.
func TestCompactionAndStorehouseBootstrap(t *testing.T) {
	unittest.RunWithTempDir(t, func(base string) {
		walDir := filepath.Join(base, "wal")
		backupDir := filepath.Join(base, "backup")
		trimTmpDir := filepath.Join(base, "trim-tmp")
		checkpointDir := filepath.Join(base, "checkpoint")
		registerDir := filepath.Join(base, "registers")

		for _, dir := range []string{walDir, backupDir, trimTmpDir, checkpointDir, registerDir} {
			require.NoError(t, os.MkdirAll(dir, 0o755))
		}

		log := unittest.Logger()

		// ── Step 1: build a real ledger WAL with three trie updates ───────────────
		//
		// In the WAL format TrieUpdate.RootHash is the PARENT (pre-update) trie hash.
		// The sealed state is state 1 (after the second update).
		// We need a third update so that the sealed state hash appears as the
		// parent hash of a WAL record — SearchRootHashBackward looks for records
		// where update.RootHash equals the target.
		//
		// Trie progression:
		//   emptyTrie (H_empty) →[update 0]→ trie0 (H0) →[update 1]→ trie1/SEALED (H1) →[update 2]→ trie2 (H2)
		//
		// WAL records:
		//   segment 0: {RootHash=H_empty, paths=[path0], payloads=[p0]}  → produces H0
		//   segment 1: {RootHash=H0,      paths=[path1], payloads=[p1]}  → produces H1 (sealed)
		//   segment 2: {RootHash=H1,      paths=[path2], payloads=[p2]}  → produces H2
		//
		// SearchRootHashBackward(H1) finds segment 2 (RootHash=H1 is the parent there).
		// TrimWALSegmentToHash(2, H1) keeps the record in segment 2 (it uses H1 as parent).
		// ReadTrie replays segments 0,1,2(trimmed) and the forest then contains H0 and H1.

		const (
			forestCapacity = 10
			pathByteSize   = 32
			segmentSize    = 32 * 1024 // 32 KB minimum: one record per segment
		)

		emptyTrieHash := ledger.RootHash(mtrietrie.NewEmptyMTrie().RootHash())

		type step struct {
			parentHash ledger.RootHash
			newTrie    *mtrietrie.MTrie
			newHash    ledger.RootHash
			path       ledger.Path
			payload    *ledger.Payload
			key        ledger.Key
			value      ledger.Value
		}

		steps := make([]step, 3)
		currentTrie := mtrietrie.NewEmptyMTrie()
		for i := range 3 {
			key := makeRegisterKey(uint16(i + 1))
			// Use large values so each WAL record fills a 32 KB segment.
			value := make(ledger.Value, 25*1024)
			value[0] = byte(i + 1)

			path := testutils.PathByUint16(uint16(i))
			payload := ledger.NewPayload(key, value)

			newTrie, _, err := mtrietrie.NewTrieWithUpdatedRegisters(
				currentTrie, []ledger.Path{path}, []ledger.Payload{*payload}, true)
			require.NoError(t, err)

			steps[i] = step{
				parentHash: ledger.RootHash(currentTrie.RootHash()),
				newTrie:    newTrie,
				newHash:    ledger.RootHash(newTrie.RootHash()),
				path:       path,
				payload:    payload,
				key:        key,
				value:      value,
			}
			currentTrie = newTrie
		}
		_ = emptyTrieHash

		// Write WAL records: RootHash = parent hash, Paths/Payloads = the delta.
		diskWAL, err := flowWAL.NewDiskWAL(
			zerolog.Nop(), nil, &metrics.NoopCollector{},
			walDir, forestCapacity, pathByteSize, segmentSize)
		require.NoError(t, err)

		for i := range 3 {
			update := &ledger.TrieUpdate{
				RootHash: steps[i].parentHash, // parent (pre-update) hash
				Paths:    []ledger.Path{steps[i].path},
				Payloads: []*ledger.Payload{steps[i].payload},
			}
			_, _, err = diskWAL.RecordUpdate(update)
			require.NoError(t, err)
		}
		<-diskWAL.Done()

		// The sealed state is the trie produced by step 1.
		sealedStep := steps[1]
		sealedHash := flow.StateCommitment(sealedStep.newHash) // H1

		// ── Step 2: locate the target root hash in the WAL ───────────────────────
		// SearchRootHashBackward finds the last record where update.RootHash == H1.
		// That is the record in segment 2 (the post-sealed step).

		segment, _, err := common.SearchRootHashBackward(
			sealedStep.newHash, walDir, common.DefaultWALFrom, common.DefaultWALTo)
		require.NoError(t, err)

		// ── Step 3: trim the WAL at the target root hash ─────────────────────────

		newSegFile, err := common.TrimWALSegmentToHash(walDir, segment, sealedStep.newHash, trimTmpDir)
		require.NoError(t, err)
		require.NotEmpty(t, newSegFile)

		require.NoError(t, common.BackupAndReplaceWALSegment(segment, walDir, backupDir, newSegFile))

		// ── Step 4: extract a single-trie V6 checkpoint ──────────────────────────

		t2, err := utilledger.ReadTrie(walDir, sealedHash)
		require.NoError(t, err)
		require.NotNil(t, t2)
		require.Equal(t, sealedStep.newHash, ledger.RootHash(t2.RootHash()),
			"ReadTrie must return the trie at the sealed state")

		checkpointName := flowWAL.NumberToFilename(segment)
		require.NoError(t, flowWAL.StoreCheckpointV6Concurrently(
			[]*mtrietrie.MTrie{t2}, checkpointDir, checkpointName, log))

		// ── Step 5: verify checkpoint invariants ─────────────────────────────────

		checkpointNums, _, err := flowWAL.ListCheckpoints(checkpointDir)
		require.NoError(t, err)
		require.Len(t, checkpointNums, 1, "expected exactly one numbered checkpoint")

		latestName := flowWAL.NumberToFilename(checkpointNums[0])
		tries, err := flowWAL.OpenAndReadCheckpointV6(checkpointDir, latestName, log)
		require.NoError(t, err)
		require.Len(t, tries, 1, "sealed checkpoint must contain exactly 1 trie")

		checkpointHash := ledger.RootHash(tries[0].RootHash())
		require.Equal(t, sealedStep.newHash, checkpointHash,
			"checkpoint root hash must equal sealed state commitment")

		// ── Step 6: bootstrap a register store from the checkpoint ───────────────

		checkpointFile := filepath.Join(checkpointDir, latestName)
		sealedHeight := uint64(42) // arbitrary height representing the sealed block

		pdb, err := storagepebble.OpenRegisterPebbleDB(log, registerDir)
		require.NoError(t, err)
		defer func() { require.NoError(t, pdb.Close()) }()

		bootstrap, err := pebble.NewRegisterBootstrap(
			pdb, checkpointFile, sealedHeight, checkpointHash, log)
		require.NoError(t, err)

		require.NoError(t, bootstrap.IndexCheckpointFile(context.Background(), 2))

		// ── Step 7: query the register store to verify sealed state ──────────────

		reg, err := storagepebble.NewRegisters(pdb, storagepebble.PruningDisabled)
		require.NoError(t, err)

		require.Equal(t, sealedHeight, reg.LatestHeight())
		require.Equal(t, sealedHeight, reg.FirstHeight())

		// Verify that both step 0 and step 1 registers are accessible at sealedHeight,
		// since the sealed state accumulates all prior updates.
		for _, s := range steps[:2] {
			require.Len(t, s.key.KeyParts, 2)
			registerID := flow.RegisterID{
				Owner: string(s.key.KeyParts[0].Value),
				Key:   string(s.key.KeyParts[1].Value),
			}
			val, err := reg.Get(registerID, sealedHeight)
			require.NoError(t, err, "register from sealed state must be readable")
			require.Equal(t, []byte(s.value), val, "register value must match sealed state")
		}

		// Step 2 register must NOT be present at sealedHeight (it was added post-seal).
		s2 := steps[2]
		require.Len(t, s2.key.KeyParts, 2)
		postSealID := flow.RegisterID{
			Owner: string(s2.key.KeyParts[0].Value),
			Key:   string(s2.key.KeyParts[1].Value),
		}
		_, err = reg.Get(postSealID, sealedHeight)
		require.Error(t, err, "post-seal register must not be present at sealedHeight")
	})
}
