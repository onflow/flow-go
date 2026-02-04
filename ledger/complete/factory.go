package complete

import (
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module"
)

// LocalLedgerFactory creates in-process ledger instances with compactor.
type LocalLedgerFactory struct {
	wal               wal.LedgerWAL
	capacity          int
	compactorConfig   *ledger.CompactorConfig
	triggerCheckpoint *atomic.Bool
	metrics           module.LedgerMetrics
	logger            zerolog.Logger
	pathFinderVersion uint8
}

// NewLocalLedgerFactory creates a new factory for local ledger instances.
// triggerCheckpoint is a runtime control signal to trigger checkpoint on next segment finish.
func NewLocalLedgerFactory(
	ledgerWAL wal.LedgerWAL,
	capacity int,
	compactorConfig *ledger.CompactorConfig,
	triggerCheckpoint *atomic.Bool,
	metrics module.LedgerMetrics,
	logger zerolog.Logger,
	pathFinderVersion uint8,
) ledger.Factory {
	return &LocalLedgerFactory{
		wal:               ledgerWAL,
		capacity:          capacity,
		compactorConfig:   compactorConfig,
		triggerCheckpoint: triggerCheckpoint,
		metrics:           metrics,
		logger:            logger,
		pathFinderVersion: pathFinderVersion,
	}
}

func (f *LocalLedgerFactory) NewLedger() (ledger.Ledger, error) {
	ledgerWithCompactor, err := NewLedgerWithCompactor(
		f.wal,
		f.capacity,
		f.compactorConfig,
		f.triggerCheckpoint,
		f.metrics,
		f.logger,
		f.pathFinderVersion,
	)
	if err != nil {
		return nil, err
	}
	return ledgerWithCompactor, nil
}
