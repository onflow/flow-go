package complete

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module"
)

// LocalLedgerFactory creates in-process ledger instances with compactor.
type LocalLedgerFactory struct {
	wal               wal.LedgerWAL
	capacity          int
	compactorConfig   *ledger.CompactorConfig
	metrics           module.LedgerMetrics
	logger            zerolog.Logger
	pathFinderVersion uint8
}

// NewLocalLedgerFactory creates a new factory for local ledger instances.
func NewLocalLedgerFactory(
	ledgerWAL wal.LedgerWAL,
	capacity int,
	compactorConfig *ledger.CompactorConfig,
	metrics module.LedgerMetrics,
	logger zerolog.Logger,
	pathFinderVersion uint8,
) ledger.Factory {
	return &LocalLedgerFactory{
		wal:               ledgerWAL,
		capacity:          capacity,
		compactorConfig:   compactorConfig,
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
		f.metrics,
		f.logger,
		f.pathFinderVersion,
	)
	if err != nil {
		return nil, err
	}
	return ledgerWithCompactor, nil
}
