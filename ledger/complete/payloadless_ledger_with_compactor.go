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
// Boot-time recovery (loading the latest V7 checkpoint and replaying newer WAL
// segments) is performed by [NewPayloadlessLedger], mirroring how the V6
// [NewLedgerWithCompactor] delegates recovery to [NewLedger].
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
	// A compactor requires a real WAL to record updates and write checkpoints.
	// In-memory construction (nil WAL) must go through NewPayloadlessLedger.
	if diskWAL == nil {
		return nil, fmt.Errorf("payloadless ledger with compactor requires a non-nil WAL")
	}

	logger = logger.With().Str("ledger_mod", "complete-payloadless").Logger()

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
