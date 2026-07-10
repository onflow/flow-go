package pebble

// This file implements rolling the register store back to a target height.
//
// It is intentionally a package-level function that operates on the raw *pebble.DB,
// rather than a method on Registers. Many modules hold a Registers (or
// storage.RegisterIndex) reference during normal operation but must never roll a live
// store back — exposing rollback as a method would put a destructive, offline-only
// operation within reach of all of them. Gating it behind direct *pebble.DB access
// confines it to the offline util that opens the DB exclusively (pebble takes an
// exclusive directory lock, so it cannot run while a node holds the DB open), and keeps
// it out of the register module's public surface.
//
// Placing it in this package also lets it reuse the unexported key helpers
// (newLookupKey, latestHeightKey, encodedUint64) directly, rather than duplicating the
// register key encoding elsewhere.

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/cockroachdb/pebble/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// UpdatedRegistersProvider returns the register IDs that were updated at the given block
// height. It is the inverse source of what the storehouse background indexer used to
// populate the register store: the same execution-data trie updates that were written at
// `height` identify the keys to remove when rolling that height back.
//
// It MUST return an error (aborting the rollback) if the updates for a height cannot be
// determined — e.g. the execution data or execution result is unavailable. Returning an
// incomplete set would leave stale register entries above the target height and corrupt
// the store, so a missing height must never be silently skipped.
type UpdatedRegistersProvider func(height uint64) ([]flow.RegisterID, error)

// maxSanitySampleSize bounds the number of registers snapshotted for the before/after
// read-back sanity check, keeping its memory and time cost bounded for large rollbacks.
const maxSanitySampleSize = 10_000

// RollbackRegisterStoreToHeight rolls the register store in `db` back to `targetHeight`.
// It removes every register update stored at a height strictly greater than
// `targetHeight`, and lowers the store's latest indexed height to exactly `targetHeight`.
// All removals plus the latest-height update are applied in a single atomic pebble batch:
// either the store transitions fully from its current latest height to `targetHeight`, or
// (on crash before commit) it is left untouched and the run can be repeated.
//
// The set of keys to remove at each height is obtained from `updatedRegisters`, which must
// yield exactly the register IDs the indexer wrote at that height (see the type doc). The
// height range walked is `(targetHeight, latest]`.
//
// As a runtime guard, a sample of the affected registers is read at `targetHeight` before
// and after the rollback; because a read at `targetHeight` never observes updates above it,
// those values must be identical afterwards. A mismatch, or a latest height that is not
// exactly `targetHeight` after commit, is reported as a fatal error.
//
// CAUTION: This is an offline operation. The caller must guarantee no other process has the
// register DB open (pebble's exclusive directory lock enforces this in practice).
//
// Expected error returns during normal operation:
//   - [storage.ErrNotBootstrapped]: if the register db has not been bootstrapped
func RollbackRegisterStoreToHeight(
	log zerolog.Logger,
	db *pebble.DB,
	targetHeight uint64,
	updatedRegisters UpdatedRegistersProvider,
) error {
	first, latest, err := ReadHeightsFromBootstrappedDB(db)
	if err != nil {
		return fmt.Errorf("cannot read heights from register db: %w", err)
	}

	log.Info().
		Uint64("first_height", first).
		Uint64("latest_height", latest).
		Uint64("target_height", targetHeight).
		Msg("rolling back register store")

	// Preconditions: the target height must lie within the indexed range.
	if targetHeight > latest {
		return fmt.Errorf("cannot roll back to height %d above latest indexed height %d: would leave a gap in indexed heights", targetHeight, latest)
	}
	if targetHeight < first {
		return fmt.Errorf("cannot roll back to height %d below first indexed height %d", targetHeight, first)
	}
	if targetHeight == latest {
		log.Info().Msgf("register store already at height %d, nothing to roll back", targetHeight)
		return nil
	}

	// read-only Registers used only for the before/after read-back sanity check.
	regs, err := NewRegisters(db, PruningDisabled)
	if err != nil {
		return fmt.Errorf("cannot open registers for sanity check: %w", err)
	}

	batch := db.NewBatch()
	defer batch.Close()

	totalHeights := latest - targetHeight
	sample := make([]flow.RegisterID, 0, maxSanitySampleSize)
	var removedTotal uint64

	// Walk heights from latest down to targetHeight+1, staging the deletion of every
	// register updated at each height.
	for height := latest; height > targetHeight; height-- {
		regIDs, err := updatedRegisters(height)
		if err != nil {
			return fmt.Errorf("cannot get updated registers at height %d: %w", height, err)
		}

		for _, regID := range regIDs {
			err = batch.Delete(newLookupKey(height, regID).Bytes(), nil)
			if err != nil {
				return fmt.Errorf("cannot stage delete for register %v at height %d: %w", regID, height, err)
			}
			if len(sample) < maxSanitySampleSize {
				sample = append(sample, regID)
			}
		}

		removedTotal += uint64(len(regIDs))
		log.Info().Msgf("staged removal of %d registers at height %d (%d/%d heights)",
			len(regIDs), height, latest-height+1, totalHeights)
	}

	// Snapshot the sampled registers at targetHeight BEFORE committing. A read at
	// targetHeight already ignores entries above it, so this is the expected post-rollback
	// value.
	before, err := snapshotRegisters(regs, sample, targetHeight)
	if err != nil {
		return fmt.Errorf("cannot snapshot registers before rollback: %w", err)
	}

	// Lower the latest indexed height as part of the same atomic batch.
	err = batch.Set(latestHeightKey, encodedUint64(targetHeight), nil)
	if err != nil {
		return fmt.Errorf("cannot stage latest height update to %d: %w", targetHeight, err)
	}

	err = batch.Commit(pebble.Sync)
	if err != nil {
		return fmt.Errorf("cannot commit rollback batch: %w", err)
	}

	log.Info().Msgf("committed rollback: removed %d register updates across heights (%d, %d]",
		removedTotal, targetHeight, latest)

	// Sanity check 1: latest height must now be exactly targetHeight.
	_, latestAfter, err := ReadHeightsFromBootstrappedDB(db)
	if err != nil {
		return fmt.Errorf("cannot read heights after rollback: %w", err)
	}
	if latestAfter != targetHeight {
		return fmt.Errorf("sanity check failed: latest height after rollback is %d, expected %d", latestAfter, targetHeight)
	}

	// Sanity check 2: values at targetHeight must be unchanged by the rollback.
	after, err := snapshotRegisters(regs, sample, targetHeight)
	if err != nil {
		return fmt.Errorf("cannot snapshot registers after rollback: %w", err)
	}
	err = compareSnapshots(before, after)
	if err != nil {
		return fmt.Errorf("sanity check failed: register values at height %d changed during rollback: %w", targetHeight, err)
	}

	log.Info().Msgf("sanity check passed: %d sampled registers unchanged at height %d, latest height is %d",
		len(before), targetHeight, targetHeight)

	return nil
}

// registerSnapshot records the state of a single register read at a height: either its
// value, or that it was not found.
type registerSnapshot struct {
	value    flow.RegisterValue
	notFound bool
}

// snapshotRegisters reads each register in regIDs at the given height and records its value
// or not-found state. Duplicate register IDs are read once.
//
// No error returns are expected during normal operation.
func snapshotRegisters(regs *Registers, regIDs []flow.RegisterID, height uint64) (map[flow.RegisterID]registerSnapshot, error) {
	snap := make(map[flow.RegisterID]registerSnapshot, len(regIDs))
	for _, regID := range regIDs {
		if _, ok := snap[regID]; ok {
			continue
		}

		val, err := regs.Get(regID, height)
		if errors.Is(err, storage.ErrNotFound) {
			snap[regID] = registerSnapshot{notFound: true}
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("cannot read register %v at height %d: %w", regID, height, err)
		}
		snap[regID] = registerSnapshot{value: val}
	}
	return snap, nil
}

// compareSnapshots verifies that every register in `before` has an identical state in
// `after` (same not-found status, same value bytes).
//
// Expected error returns during normal operation: none. A returned error indicates the
// rollback disturbed state at or below the target height, i.e. corruption.
func compareSnapshots(before, after map[flow.RegisterID]registerSnapshot) error {
	for regID, b := range before {
		a, ok := after[regID]
		if !ok {
			return fmt.Errorf("register %v missing from post-rollback snapshot", regID)
		}
		if b.notFound != a.notFound {
			return fmt.Errorf("register %v presence changed: before found=%v, after found=%v", regID, !b.notFound, !a.notFound)
		}
		if !b.notFound && !bytes.Equal(b.value, a.value) {
			return fmt.Errorf("register %v value changed", regID)
		}
	}
	return nil
}
