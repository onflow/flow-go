package complete

import (
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/ledger"
	realWAL "github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module"
)

// PayloadlessLedgerWithCompactor bundles a [PayloadlessLedger] with its
// [PayloadlessCompactor] so callers can treat the pair as a single
// ReadyDoneAware component. It is the payloadless analog of
// [LedgerWithCompactor].
//
// Embedding *PayloadlessLedger automatically delegates the public ledger
// methods (Set, Get*, Has*, Prove, etc.). Ready and Done are overridden so the
// compactor's lifecycle is coordinated with the ledger's.
type PayloadlessLedgerWithCompactor struct {
	*PayloadlessLedger
	compactor *PayloadlessCompactor
	logger    zerolog.Logger
}

// NewPayloadlessLedgerWithCompactor constructs a payloadless ledger and a
// payloadless compactor wired together against the shared [realWAL.LedgerWAL].
//
// Boot-time bootstrap:
//
//  1. Load the latest V7 (payloadless) checkpoint from `diskWAL`'s directory,
//     if one exists, seeding the ledger's forest with its tries.
//  2. Replay WAL segments newer than that checkpoint into the forest, so
//     in-memory state is recovered up to the last durable update.
//
// Steady-state:
//
//   - Each [PayloadlessLedger.Set] sends a [WALPayloadlessTrieUpdate] to the
//     compactor, which writes to the WAL via [realWAL.LedgerWAL.RecordUpdate].
//   - Every `CheckpointDistance` segments (or on `triggerCheckpoint`) the
//     compactor snapshots the rolling trie queue into a V7 checkpoint.
//   - The compactor enforces `CheckpointsToKeep` against V7 files.
//
// All returned errors indicate the bundle can't be created and the caller
// should treat them as unrecoverable.
func NewPayloadlessLedgerWithCompactor(
	diskWAL realWAL.LedgerWAL,
	ledgerCapacity int,
	compactorConfig *ledger.CompactorConfig,
	triggerCheckpoint *atomic.Bool,
	metrics module.LedgerMetrics,
	logger zerolog.Logger,
	pathFinderVersion uint8,
) (*PayloadlessLedgerWithCompactor, error) {
	l, err := NewPayloadlessLedger(
		diskWAL,
		ledgerCapacity,
		metrics,
		logger,
		pathFinderVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create payloadless ledger: %w", err)
	}

	logger = logger.With().Str("ledger_mod", "complete-payloadless").Logger()

	// Bootstrap from the latest V7 checkpoint on disk, if any, and replay any
	// WAL segments newer than that checkpoint. Both steps are no-ops on a fresh
	// node.
	checkpointer, err := diskWAL.NewCheckpointer()
	if err != nil {
		return nil, fmt.Errorf("failed to create checkpointer: %w", err)
	}
	latestV7 := latestV7CheckpointNum(checkpointer, logger)
	if latestV7 >= 0 {
		v7Name := realWAL.NumberToFilenameV7(latestV7)
		tries, err := realWAL.OpenAndReadCheckpointV7(checkpointer.Dir(), v7Name, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to load V7 checkpoint %s: %w", v7Name, err)
		}
		if err := l.AddTries(tries); err != nil {
			return nil, fmt.Errorf("failed to seed payloadless forest from V7 checkpoint: %w", err)
		}
		logger.Info().
			Int("latest_v7", latestV7).
			Int("trie_count", len(tries)).
			Msg("payloadless ledger seeded from V7 checkpoint")
	}

	// Pause WAL recording while we replay so we don't re-log existing records.
	diskWAL.PauseRecord()
	if err := diskWAL.ReplaySegmentsForPayloadlessForest(l.forest, latestV7); err != nil {
		diskWAL.UnpauseRecord()
		return nil, fmt.Errorf("failed to replay WAL segments onto payloadless forest: %w", err)
	}
	diskWAL.UnpauseRecord()

	compactor, err := NewPayloadlessCompactor(
		l,
		diskWAL,
		logger.With().Str("subcomponent", "payloadless-compactor").Logger(),
		compactorConfig.CheckpointCapacity,
		compactorConfig.CheckpointDistance,
		compactorConfig.CheckpointsToKeep,
		triggerCheckpoint,
		compactorConfig.Metrics,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create payloadless compactor: %w", err)
	}

	return &PayloadlessLedgerWithCompactor{
		PayloadlessLedger: l,
		compactor:         compactor,
		logger:            logger,
	}, nil
}

// Ready waits for both the ledger and the compactor to be ready. Overrides
// the embedded [PayloadlessLedger.Ready] so the compactor lifecycle is part of
// the readiness contract.
func (lwc *PayloadlessLedgerWithCompactor) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		defer close(ready)
		<-lwc.PayloadlessLedger.Ready()
		<-lwc.compactor.Ready()
		lwc.logger.Info().Msg("payloadless ledger with compactor ready")
	}()
	return ready
}

// Done shuts the bundle down. The ledger closes its trie-update channel so the
// compactor can drain it; the compactor then closes the WAL. Overrides the
// embedded [PayloadlessLedger.Done].
func (lwc *PayloadlessLedgerWithCompactor) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)

		lwc.logger.Info().Msg("stopping payloadless ledger with compactor...")

		// Close the trie-update channel so the compactor's drain loop terminates.
		<-lwc.PayloadlessLedger.Done()

		// Then wait for the compactor (which finalizes the WAL).
		<-lwc.compactor.Done()

		lwc.logger.Info().Msg("payloadless ledger with compactor stopped")
	}()
	return done
}
