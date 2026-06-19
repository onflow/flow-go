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
	logger := config.Logger.With().Str("subcomponent", "ledger").Logger()
	logger.Info().
		Str("ledger_service_addr", config.LedgerServiceAddr).
		Msg("using remote ledger service")

	var opts []remote.ClientOption
	if config.LedgerMaxRequestSize > 0 {
		opts = append(opts, remote.WithMaxRequestSize(config.LedgerMaxRequestSize))
	}
	if config.LedgerMaxResponseSize > 0 {
		opts = append(opts, remote.WithMaxResponseSize(config.LedgerMaxResponseSize))
	}

	client, err := remote.NewClient(config.LedgerServiceAddr, logger, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote ledger: %w", err)
	}

	return client, nil
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

	// Create ledger with internal compactor
	ledgerStorage, err := complete.NewLedgerWithCompactor(
		diskWal,
		int(config.MTrieCacheSize),
		compactorConfig,
		triggerCheckpoint,
		config.LedgerMetrics,
		config.Logger.With().Str("subcomponent", "ledger").Logger(),
		complete.DefaultPathFinderVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create local ledger: %w", err)
	}

	return ledgerStorage, nil
}

// NewPayloadlessLedger creates a payloadless ledger instance based on the
// configuration. If LedgerServiceAddr is set, it creates a remote payloadless
// ledger client. Otherwise, it creates a local payloadless ledger with WAL
// and compactor.
//
// This is the payloadless-mode counterpart of [NewLedger]. The signature and
// dispatch shape mirror that function so call sites in
// cmd/execution_builder.go can switch between the two without changing how
// config is plumbed.
//
// triggerCheckpoint is a runtime control signal to trigger checkpoint on
// next segment finish (ignored by the remote client; can be nil).
func NewPayloadlessLedger(config Config, triggerCheckpoint *atomic.Bool) (ledger.PayloadlessLedger, error) {
	if config.LedgerServiceAddr != "" {
		return newRemotePayloadlessLedger(config)
	}
	return newLocalPayloadlessLedger(config, triggerCheckpoint)
}

// newRemotePayloadlessLedger creates a remote payloadless ledger client that
// connects to a payloadless ledger service over gRPC. The client's [Ready]
// method verifies the server is running in payloadless mode and crashes if
// it is not — i.e. a wrong-mode server is treated as a deployment error, not
// a retryable failure.
func newRemotePayloadlessLedger(config Config) (ledger.PayloadlessLedger, error) {
	logger := config.Logger.With().Str("subcomponent", "ledger").Logger()
	logger.Info().
		Str("ledger_service_addr", config.LedgerServiceAddr).
		Msg("using remote payloadless ledger service")

	var opts []remote.ClientOption
	if config.LedgerMaxRequestSize > 0 {
		opts = append(opts, remote.WithMaxRequestSize(config.LedgerMaxRequestSize))
	}
	if config.LedgerMaxResponseSize > 0 {
		opts = append(opts, remote.WithMaxResponseSize(config.LedgerMaxResponseSize))
	}

	client, err := remote.NewPayloadlessClient(config.LedgerServiceAddr, logger, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote payloadless ledger client: %w", err)
	}
	return client, nil
}

// newLocalPayloadlessLedger creates a local payloadless ledger with WAL and
// compactor, mirroring [newLocalLedger] for the full ledger.
//
// The factory opens a [wal.DiskWAL] over config.Triedir and returns a
// [complete.PayloadlessLedgerWithCompactor], which:
//
//	(a) seeds its forest from the latest V7 (payloadless) checkpoint;
//	(b) replays WAL segments newer than that checkpoint;
//	(c) records subsequent updates to the shared WAL; and
//	(d) emits a new V7 checkpoint every config.CheckpointDistance segments,
//	    pruning down to config.CheckpointsToKeep V7 files.
//
// Either a numbered V7 checkpoint or a V7 root checkpoint must be present in
// config.Triedir. If only V6 checkpoints exist (no V7 of either kind), the
// factory logs a hint pointing to the checkpoint-convert-v7 utility and refuses
// to start — the leaf-hash commitment cannot be reconstructed by WAL replay
// alone.
//
// Expected error returns during normal operation:
//   - error if config.Triedir is empty
//   - error if no V7 (payloadless) checkpoint exists in config.Triedir
func newLocalPayloadlessLedger(config Config, triggerCheckpoint *atomic.Bool) (ledger.PayloadlessLedger, error) {
	logger := config.Logger.With().Str("subcomponent", "ledger").Logger()

	if config.Triedir == "" {
		return nil, fmt.Errorf("payloadless ledger requires a non-empty config.Triedir")
	}

	// A V7 (payloadless) checkpoint must exist in `Triedir` before a payloadless
	// node can boot. There is no payloadless bootstrap path that doesn't go
	// through a V7 checkpoint: the WAL alone records full payload updates, but
	// the leaf-hash commitment can only be reconstructed by replaying every
	// update from genesis, which is not feasible at runtime. A numbered V7
	// checkpoint (written by the compactor) or a V7 root checkpoint (converted
	// from the V6 root.checkpoint during bootstrap) both satisfy this. Hence:
	// neither present → refuse to start.
	v7Numbers, latestV7, err := wal.ListV7Checkpoints(config.Triedir)
	if err != nil {
		return nil, fmt.Errorf("could not list V7 checkpoints in %s: %w", config.Triedir, err)
	}
	if latestV7 < 0 {
		// No numbered V7 checkpoint. A V7 root checkpoint is also acceptable: a
		// freshly-sporked payloadless node has its V6 root.checkpoint converted
		// to a V7 root checkpoint during bootstrap, which the bundle seeds from.
		hasV7Root, rootErr := wal.HasRootCheckpointV7(config.Triedir)
		if rootErr != nil {
			return nil, fmt.Errorf("could not check for V7 root checkpoint in %s: %w", config.Triedir, rootErr)
		}
		if !hasV7Root {
			// Look for V6 checkpoints so the error message can point the operator
			// at the convert utility. List failures here are non-fatal: we still
			// want the operator to see the primary "no V7" error.
			v6Numbers, latestV6, v6ListErr := wal.ListV6Checkpoints(config.Triedir)
			if v6ListErr != nil {
				logger.Warn().Err(v6ListErr).
					Str("triedir", config.Triedir).
					Msg("payloadless ledger: could not also list V6 checkpoints while reporting missing V7")
			}
			if latestV6 >= 0 {
				logger.Warn().
					Str("triedir", config.Triedir).
					Int("latest_v6", latestV6).
					Int("v6_count", len(v6Numbers)).
					Msg("payloadless ledger: no V7 checkpoint found, but V6 checkpoints exist — " +
						"run the `checkpoint-convert-v7` util to produce a V7 checkpoint")
				return nil, fmt.Errorf(
					"no V7 (payloadless) checkpoint found in %s but %d V6 checkpoint(s) exist (latest: %d); "+
						"run the `checkpoint-convert-v7` util to produce a V7 checkpoint before restart",
					config.Triedir, len(v6Numbers), latestV6,
				)
			}
			return nil, fmt.Errorf(
				"no V7 (payloadless) checkpoint found in %s; a V7 checkpoint is required to start a payloadless node",
				config.Triedir,
			)
		}
		logger.Info().
			Str("triedir", config.Triedir).
			Msg("payloadless ledger: V7 root checkpoint discovered; the bundle will seed from it")
	} else {
		logger.Info().
			Str("triedir", config.Triedir).
			Int("latest_v7", latestV7).
			Int("v7_count", len(v7Numbers)).
			Msg("payloadless ledger: V7 checkpoint discovered; the bundle will seed from it")
	}

	diskWAL, err := wal.NewDiskWAL(
		logger.With().Str("subcomponent", "wal").Logger(),
		config.MetricsRegisterer,
		config.WALMetrics,
		config.Triedir,
		int(config.MTrieCacheSize),
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize payloadless wal: %w", err)
	}

	compactorConfig := &ledger.CompactorConfig{
		CheckpointCapacity: uint(config.MTrieCacheSize),
		CheckpointDistance: config.CheckpointDistance,
		CheckpointsToKeep:  config.CheckpointsToKeep,
		Metrics:            config.WALMetrics,
	}

	bundle, err := complete.NewPayloadlessLedgerWithCompactor(
		diskWAL,
		int(config.MTrieCacheSize),
		compactorConfig,
		triggerCheckpoint,
		config.LedgerMetrics,
		logger,
		complete.DefaultPathFinderVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create payloadless ledger with compactor: %w", err)
	}

	return bundle, nil
}
