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
	LedgerServiceAddr     string // gRPC address for remote ledger service (empty means use local ledger)
	LedgerMaxRequestSize  uint   // Maximum request message size in bytes for remote ledger client (0 = default 1 GiB)
	LedgerMaxResponseSize uint   // Maximum response message size in bytes for remote ledger client (0 = default 1 GiB)

	// Local ledger configuration
	Triedir            string
	MTrieCacheSize     uint32
	CheckpointDistance uint
	CheckpointsToKeep  uint
	MetricsRegisterer  prometheus.Registerer
	WALMetrics         module.WALMetrics
	LedgerMetrics      module.LedgerMetrics
	Logger             zerolog.Logger
}

// NewLedger creates a ledger instance based on the configuration.
// If LedgerServiceAddr is set, it creates a remote ledger client.
// Otherwise, it creates a local ledger with WAL and compactor.
// triggerCheckpoint is a runtime control signal to trigger checkpoint on next segment finish (can be nil for remote ledger).
func NewLedger(config Config, triggerCheckpoint *atomic.Bool) (ledger.Ledger, error) {
	if config.LedgerServiceAddr != "" {
		return newRemoteLedger(config)
	}
	return newLocalLedger(config, triggerCheckpoint)
}

// newRemoteLedger creates a remote ledger client that connects to a ledger service.
func newRemoteLedger(config Config) (ledger.Ledger, error) {
	config.Logger.Info().
		Str("ledger_service_addr", config.LedgerServiceAddr).
		Msg("using remote ledger service")

	factory := remote.NewRemoteLedgerFactory(
		config.LedgerServiceAddr,
		config.Logger.With().Str("subcomponent", "ledger").Logger(),
		config.LedgerMaxRequestSize,
		config.LedgerMaxResponseSize,
	)

	ledgerStorage, err := factory.NewLedger()
	if err != nil {
		return nil, fmt.Errorf("failed to create remote ledger: %w", err)
	}

	return ledgerStorage, nil
}

// newLocalLedger creates a local ledger with WAL and compactor.
func newLocalLedger(config Config, triggerCheckpoint *atomic.Bool) (ledger.Ledger, error) {
	// the local ledger service is used when:
	// 1. execution node is running ledger in local
	// 2. the standalone ledger service is running it in local

	config.Logger.Info().
		Str("triedir", config.Triedir).
		Msg("using local ledger")

	// Create WAL
	diskWal, err := wal.NewDiskWAL(
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
		CheckpointCapacity: uint(config.MTrieCacheSize),
		CheckpointDistance: config.CheckpointDistance,
		CheckpointsToKeep:  config.CheckpointsToKeep,
		Metrics:            config.WALMetrics,
	}

	// Use factory to create ledger with internal compactor
	factory := complete.NewLocalLedgerFactory(
		diskWal,
		int(config.MTrieCacheSize),
		compactorConfig,
		triggerCheckpoint,
		config.LedgerMetrics,
		config.Logger.With().Str("subcomponent", "ledger").Logger(),
		complete.DefaultPathFinderVersion,
	)

	ledgerStorage, err := factory.NewLedger()
	if err != nil {
		return nil, fmt.Errorf("failed to create local ledger: %w", err)
	}

	return ledgerStorage, nil
}
