package factory

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/ledger/remote"
	"github.com/onflow/flow-go/module"
)

// Config holds configuration for creating a ledger instance.
type Config struct {
	// Remote ledger service configuration
	LedgerServiceAddr string // gRPC address for remote ledger service (empty means use local ledger)

	// Local ledger configuration
	Triedir                              string
	MTrieCacheSize                       uint32
	CheckpointDistance                   uint
	CheckpointsToKeep                    uint
	TriggerCheckpointOnNextSegmentFinish *atomic.Bool
	MetricsRegisterer                    prometheus.Registerer
	WALMetrics                           module.WALMetrics
	LedgerMetrics                        module.LedgerMetrics
	Logger                               zerolog.Logger
}

// Result holds the result of creating a ledger instance.
type Result struct {
	Ledger ledger.Ledger
	WAL    *wal.DiskWAL // Only set for local ledger, nil for remote
}

// NewLedger creates a ledger instance based on the configuration.
// If LedgerServiceAddr is set, it creates a remote ledger client.
// Otherwise, it creates a local ledger with WAL and compactor.
func NewLedger(config Config) (*Result, error) {
	var factory ledger.Factory
	var diskWal wal.LedgerWAL

	// Check if remote ledger service is configured
	if config.LedgerServiceAddr != "" {
		// Use remote ledger service
		config.Logger.Info().
			Str("ledger_service_addr", config.LedgerServiceAddr).
			Msg("using remote ledger service")

		factory = remote.NewRemoteLedgerFactory(
			config.LedgerServiceAddr,
			config.Logger.With().Str("subcomponent", "ledger").Logger(),
		)
	} else {
		// Use local ledger with WAL
		config.Logger.Info().
			Str("triedir", config.Triedir).
			Msg("using local ledger")

		// Create WAL
		var err error
		diskWal, err = wal.NewDiskWAL(
			config.Logger.With().Str("subcomponent", "wal").Logger(),
			config.MetricsRegisterer,
			config.WALMetrics,
			config.Triedir,
			int(config.MTrieCacheSize),
			pathfinder.PathByteSize,
			wal.SegmentSize,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize wal: %w", err)
		}

		// Create compactor config
		compactorConfig := &ledger.CompactorConfig{
			CheckpointCapacity:                   uint(config.MTrieCacheSize),
			CheckpointDistance:                   config.CheckpointDistance,
			CheckpointsToKeep:                    config.CheckpointsToKeep,
			TriggerCheckpointOnNextSegmentFinish: config.TriggerCheckpointOnNextSegmentFinish,
			Metrics:                              config.WALMetrics,
		}

		// Use factory to create ledger with internal compactor
		factory = complete.NewLocalLedgerFactory(
			diskWal,
			int(config.MTrieCacheSize),
			compactorConfig,
			config.LedgerMetrics,
			config.Logger.With().Str("subcomponent", "ledger").Logger(),
			complete.DefaultPathFinderVersion,
		)
	}

	ledgerStorage, err := factory.NewLedger()
	if err != nil {
		return nil, fmt.Errorf("failed to create ledger: %w", err)
	}

	// Type assert to get the concrete DiskWAL type (only for local ledger)
	var diskWAL *wal.DiskWAL
	if diskWal != nil {
		var ok bool
		diskWAL, ok = diskWal.(*wal.DiskWAL)
		if !ok {
			return nil, fmt.Errorf("expected *wal.DiskWAL but got %T", diskWal)
		}
	}

	return &Result{
		Ledger: ledgerStorage,
		WAL:    diskWAL,
	}, nil
}
