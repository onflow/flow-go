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

// NewPayloadlessLedger creates a payloadless ledger instance.
//
// This is the payloadless-mode counterpart of [NewLedger]. It mirrors that
// function's signature so call sites in cmd/execution_builder.go can switch
// between the two without changing how config is plumbed. The argument types
// are deliberately identical (same Config struct, same triggerCheckpoint).
//
// TODO: payloadless WAL is not implemented yet. This factory currently
// returns an in-memory payloadless ledger that does not persist updates to
// a WAL. config.Triedir, config.CheckpointDistance, config.CheckpointsToKeep
// and triggerCheckpoint are accepted for API parity but ignored.
//
// TODO: payloadless checkpoint loading is not implemented yet. The factory
// does not read any checkpoint file at boot, so the trie starts empty on
// every startup. To make payloadless nodes survive a restart, one of the
// following must land:
//
//  1. A native payloadless checkpoint format with its own writer, reader,
//     and bootstrap path.
//  2. A conversion path that reads the existing full V6 mtrie checkpoint
//     (the format LoadBootstrapper copies into triedir) and ingests its
//     (path, value) pairs into the payloadless trie at boot. This unblocks
//     payloadless boot from existing on-disk state without committing to a
//     payloadless checkpoint format.
//
// Until one of those is in place, --payloadless mode is suitable for
// short-lived experimental nodes only; the trie has no state on first boot
// and loses all state on restart.
//
// TODO: remote payloadless ledger client. When config.LedgerServiceAddr is
// set, this factory should construct a remote.PayloadlessClient (Spec 004).
// For now config.LedgerServiceAddr is ignored.
func NewPayloadlessLedger(config Config, triggerCheckpoint *atomic.Bool) (ledger.PayloadlessLedger, error) {
	_ = triggerCheckpoint // TODO: drive payloadless checkpoint generation once a format exists

	config.Logger.Warn().
		Str("triedir", config.Triedir).
		Msg("payloadless ledger has no WAL or checkpoint support yet; " +
			"trie state will not survive restart and will not be loaded from disk")

	return complete.NewPayloadlessLedger(
		int(config.MTrieCacheSize),
		config.LedgerMetrics,
		config.Logger.With().Str("subcomponent", "ledger").Logger(),
		complete.DefaultPathFinderVersion,
	)
}
