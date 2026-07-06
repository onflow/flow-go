package rollback_trie_to_height

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	prometheusWAL "github.com/onflow/wal/wal"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// RegisterGetter reads the value of a register as of a given block height. It returns the most
// recent value at or below the given height.
//
// [github.com/onflow/flow-go/storage/pebble.Registers] (the execution node's Storehouse register
// store) satisfies this interface.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: when the register has no value at or below the given height (i.e. the
//     register did not exist yet as of that height).
//   - [storage.ErrHeightNotIndexed]: when the given height is outside the store's indexed range.
type RegisterGetter interface {
	Get(id flow.RegisterID, height uint64) (flow.RegisterValue, error)
}

// CollectWrittenRegisters scans the WAL update records in segments [from, to] and returns the union
// of registers written by any of them, keyed by their trie [ledger.Path].
//
// The returned set is a superset of the registers that differ between any two states covered by the
// scanned segments: it includes every register written at least once in the range. This is
// intentional and harmless for reconstruction — rewriting a register that did not actually change
// with its historical value is a no-op on the trie root (see [BuildRollbackTrieUpdate]).
//
// Keying by path (rather than by [flow.RegisterID]) lets the caller build a [ledger.TrieUpdate]
// directly from the WAL-recorded paths, avoiding any dependency on the path-finder version used to
// produce those paths.
//
// No error returns are expected during normal operation.
func CollectWrittenRegisters(
	logger zerolog.Logger,
	dir string,
	from, to int,
) (map[ledger.Path]flow.RegisterID, error) {
	sr, err := prometheusWAL.NewSegmentsRangeReader(logger, prometheusWAL.SegmentRange{
		Dir:   dir,
		First: from,
		Last:  to,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot create WAL segments reader for [%d,%d]: %w", from, to, err)
	}
	defer sr.Close()

	reader := prometheusWAL.NewReader(sr)

	written := make(map[ledger.Path]flow.RegisterID)
	records := 0
	for reader.Next() {
		operation, _, update, err := wal.Decode(reader.Record())
		if err != nil {
			return nil, fmt.Errorf("cannot decode WAL record: %w", err)
		}
		if operation != wal.WALUpdate {
			continue
		}
		records++

		for i, path := range update.Paths {
			// A given path always maps to the same register, so the first sighting is sufficient.
			if _, ok := written[path]; ok {
				continue
			}
			key, err := update.Payloads[i].Key()
			if err != nil {
				return nil, fmt.Errorf("cannot read key from WAL payload: %w", err)
			}
			regID, err := convert.LedgerKeyToRegisterID(key)
			if err != nil {
				return nil, fmt.Errorf("cannot convert ledger key to register ID: %w", err)
			}
			written[path] = regID
		}
	}
	if err := reader.Err(); err != nil {
		return nil, fmt.Errorf("error while reading WAL segments [%d,%d]: %w", from, to, err)
	}

	logger.Info().
		Int("update_records", records).
		Int("distinct_registers", len(written)).
		Msgf("collected written registers from WAL segments [%d,%d]", from, to)

	return written, nil
}

// BuildRollbackTrieUpdate constructs a single [ledger.TrieUpdate] that, when applied on top of the
// base trie identified by baseRoot, rolls that trie back to the historical state at targetHeight.
//
// The update's paths are the registers written by any block in the scanned WAL segment range
// [from, to] (see [CollectWrittenRegisters]); its payloads are those registers' values as of
// targetHeight, read from getter. A register that has no value at targetHeight (getter returns
// [storage.ErrNotFound]) is written back as an empty payload, which deletes it from the trie —
// correctly modelling a register that was only created after targetHeight.
//
// Because a trie's root hash is a pure function of its register key/value content, applying the
// historical value for every register that could have changed yields a trie whose root equals the
// target state's commitment. Registers that did not actually change are rewritten with their
// unchanged value, which does not affect the root. The caller must verify the resulting root
// against the known target state commitment before trusting the update.
//
// The returned update's RootHash is set to baseRoot so that, on replay, the ledger forest looks up
// the base trie and applies these writes to it.
//
// No error returns are expected during normal operation.
func BuildRollbackTrieUpdate(
	logger zerolog.Logger,
	dir string,
	from, to int,
	baseRoot ledger.RootHash,
	getter RegisterGetter,
	targetHeight uint64,
) (*ledger.TrieUpdate, error) {
	written, err := CollectWrittenRegisters(logger, dir, from, to)
	if err != nil {
		return nil, fmt.Errorf("could not collect written registers: %w", err)
	}

	paths := make([]ledger.Path, 0, len(written))
	payloads := make([]*ledger.Payload, 0, len(written))
	deletions := 0
	for path, regID := range written {
		value, err := getter.Get(regID, targetHeight)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				// The register did not exist at or below the target height. Reset it to empty,
				// which deletes it from the reconstructed trie.
				value = nil
				deletions++
			} else {
				// Any other error (including storage.ErrHeightNotIndexed, meaning targetHeight is
				// outside the register store's retained range) is fatal for reconstruction.
				return nil, fmt.Errorf("could not read register %s at height %d: %w", regID, targetHeight, err)
			}
		}

		paths = append(paths, path)
		payloads = append(payloads, ledger.NewPayload(convert.RegisterIDToLedgerKey(regID), ledger.Value(value)))
	}

	logger.Info().
		Int("registers", len(paths)).
		Int("deletions", deletions).
		Uint64("target_height", targetHeight).
		Str("base_root", baseRoot.String()).
		Msg("built rollback trie update")

	return &ledger.TrieUpdate{
		RootHash: baseRoot,
		Paths:    paths,
		Payloads: payloads,
	}, nil
}
