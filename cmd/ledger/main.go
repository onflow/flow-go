package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"
	"google.golang.org/grpc"

	ledgerfactory "github.com/onflow/flow-go/ledger/factory"
	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
	"github.com/onflow/flow-go/ledger/remote"
	"github.com/onflow/flow-go/module/metrics"
)

var (
	triedir           = flag.String("triedir", "", "Directory for trie files (required)")
	ledgerServiceAddr = flag.String("ledger-service-addr", "0.0.0.0:9000", "Ledger service listen address (TCP: ip:port or Unix: unix:///path/to/socket)")
	mtrieCacheSize    = flag.Int("mtrie-cache-size", 500, "MTrie cache size (number of tries)")
	checkpointDist    = flag.Uint("checkpoint-distance", 100, "Checkpoint distance")
	checkpointsToKeep = flag.Uint("checkpoints-to-keep", 3, "Number of checkpoints to keep")
	logLevel          = flag.String("loglevel", "info", "Log level (panic, fatal, error, warn, info, debug)")
	maxRequestSize    = flag.Uint("max-request-size", 1<<30, "Maximum request message size in bytes (default: 1 GiB)")
	maxResponseSize   = flag.Uint("max-response-size", 10<<30, "Maximum response message size in bytes (default: 10 GiB)")
)

func main() {
	flag.Parse()

	if *triedir == "" {
		fmt.Fprintf(os.Stderr, "error: --triedir is required\n")
		os.Exit(1)
	}

	// Parse and set log level
	lvl, err := zerolog.ParseLevel(strings.ToLower(*logLevel))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid log level %q: %v\n", *logLevel, err)
		os.Exit(1)
	}
	zerolog.SetGlobalLevel(lvl)

	logger := zerolog.New(os.Stderr).With().
		Timestamp().
		Str("service", "ledger").
		Logger()

	logger.Info().
		Str("triedir", *triedir).
		Str("ledger_service_addr", *ledgerServiceAddr).
		Int("mtrie_cache_size", *mtrieCacheSize).
		Msg("starting ledger service")

	// Create ledger using factory
	metricsCollector := metrics.NewLedgerCollector("ledger", "wal")
	result, err := ledgerfactory.NewLedger(ledgerfactory.Config{
		Triedir:                              *triedir,
		MTrieCacheSize:                       uint32(*mtrieCacheSize),
		CheckpointDistance:                   *checkpointDist,
		CheckpointsToKeep:                    *checkpointsToKeep,
		TriggerCheckpointOnNextSegmentFinish: atomic.NewBool(false),
		MetricsRegisterer:                    prometheus.DefaultRegisterer,
		WALMetrics:                           metricsCollector,
		LedgerMetrics:                        metricsCollector,
		Logger:                               logger,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create ledger")
	}

	ledgerStorage := result.Ledger

	// Wait for ledger to be ready (WAL replay)
	logger.Info().Msg("waiting for ledger initialization...")
	<-ledgerStorage.Ready()
	logger.Info().Msg("ledger ready")

	// Check if any trie is loaded after startup
	stateCount := ledgerStorage.StateCount()
	if stateCount == 0 {
		logger.Fatal().Msg("no trie loaded after startup - no states available")
	}

	// Get the last trie state for logging
	lastState, err := ledgerStorage.StateByIndex(-1)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to get last state for logging")
	}
	logger.Info().
		Int("state_count", stateCount).
		Str("last_state", lastState.String()).
		Msg("ledger health check passed")

	// Create gRPC server with max message size configuration.
	// Default to 10 GiB for responses (instead of standard 4 MiB) to handle large proofs that can exceed 4MB.
	// This was increased to fix "grpc: received message larger than max" errors when generating
	// proofs for blocks with many state changes.
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(int(*maxRequestSize)),
		grpc.MaxSendMsgSize(int(*maxResponseSize)),
	)

	// Create and register ledger service
	ledgerService := remote.NewService(ledgerStorage, logger)
	ledgerpb.RegisterLedgerServiceServer(grpcServer, ledgerService)

	// Determine if we're using Unix domain socket or TCP
	var lis net.Listener
	var socketPath string
	var isUnixSocket bool

	if strings.HasPrefix(*ledgerServiceAddr, "unix://") {
		// Unix domain socket
		isUnixSocket = true
		socketPath = strings.TrimPrefix(*ledgerServiceAddr, "unix://")
		// Handle both unix:///absolute/path and unix://relative/path formats
		// net.Listen("unix", ...) expects the path without the unix:// prefix

		// Clean up any existing socket file
		if _, err := os.Stat(socketPath); err == nil {
			logger.Info().Str("socket_path", socketPath).Msg("removing existing socket file")
			if err := os.Remove(socketPath); err != nil {
				logger.Warn().Err(err).Str("socket_path", socketPath).Msg("failed to remove existing socket file")
			}
		}

		lis, err = net.Listen("unix", socketPath)
		if err != nil {
			logger.Fatal().Err(err).Str("socket_path", socketPath).Msg("failed to listen on Unix socket")
		}

		// Set socket file permissions (readable/writable by owner and group)
		if err := os.Chmod(socketPath, 0660); err != nil {
			logger.Warn().Err(err).Str("socket_path", socketPath).Msg("failed to set socket file permissions")
		}

		logger.Info().Str("socket_path", socketPath).Msg("gRPC server listening on Unix domain socket")
	} else {
		// TCP socket
		lis, err = net.Listen("tcp", *ledgerServiceAddr)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to listen")
		}

		logger.Info().Str("address", *ledgerServiceAddr).Msg("gRPC server listening on TCP")
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()

	// Wait for interrupt signal or error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info().Str("signal", sig.String()).Msg("received signal, shutting down")
	case err := <-errCh:
		logger.Error().Err(err).Msg("server error")
	}

	// Graceful shutdown
	logger.Info().Msg("shutting down gRPC server...")
	grpcServer.GracefulStop()

	// Clean up Unix socket file if we used one
	if isUnixSocket && socketPath != "" {
		if err := os.Remove(socketPath); err != nil {
			logger.Warn().Err(err).Str("socket_path", socketPath).Msg("failed to remove socket file")
		} else {
			logger.Info().Str("socket_path", socketPath).Msg("removed socket file")
		}
	}

	logger.Info().Msg("waiting for ledger to stop...")
	<-ledgerStorage.Done()

	logger.Info().Msg("ledger service stopped")
}
