package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"
	"google.golang.org/grpc"

	ledgerfactory "github.com/onflow/flow-go/ledger/factory"
	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
	"github.com/onflow/flow-go/ledger/remote"
	"github.com/onflow/flow-go/module/metrics"
)

var (
	triedir             = flag.String("triedir", "", "Directory for trie files (required)")
	ledgerServiceTCP    = flag.String("ledger-service-tcp", "", "Ledger service TCP listen address (e.g., 0.0.0.0:9000). If provided, server accepts TCP connections.")
	ledgerServiceSocket = flag.String("ledger-service-socket", "", "Ledger service Unix socket path (e.g., /sockets/ledger.sock). If provided, server accepts Unix socket connections. Can specify multiple sockets separated by comma.")
	mtrieCacheSize      = flag.Int("mtrie-cache-size", 500, "MTrie cache size (number of tries)")
	checkpointDist      = flag.Uint("checkpoint-distance", 100, "Checkpoint distance")
	checkpointsToKeep   = flag.Uint("checkpoints-to-keep", 3, "Number of checkpoints to keep")
	logLevel            = flag.String("loglevel", "info", "Log level (panic, fatal, error, warn, info, debug)")
	maxRequestSize      = flag.Uint("max-request-size", 1<<30, "Maximum request message size in bytes (default: 1 GiB)")
	maxResponseSize     = flag.Uint("max-response-size", 1<<30, "Maximum response message size in bytes (default: 1 GiB)")
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

	// Validate that at least one address is provided
	if *ledgerServiceTCP == "" && *ledgerServiceSocket == "" {
		fmt.Fprintf(os.Stderr, "error: at least one of --ledger-service-tcp or --ledger-service-socket must be provided\n")
		os.Exit(1)
	}

	logger.Info().
		Str("triedir", *triedir).
		Str("ledger_service_tcp", *ledgerServiceTCP).
		Str("ledger_service_socket", *ledgerServiceSocket).
		Int("mtrie_cache_size", *mtrieCacheSize).
		Msg("starting ledger service")

	// Create ledger using factory
	// TODO(leo): to use real metrics collector
	metricsCollector := &metrics.NoopCollector{}
	result, err := ledgerfactory.NewLedger(ledgerfactory.Config{
		Triedir:                              *triedir,
		MTrieCacheSize:                       uint32(*mtrieCacheSize),
		CheckpointDistance:                   *checkpointDist,
		CheckpointsToKeep:                    *checkpointsToKeep,
		TriggerCheckpointOnNextSegmentFinish: atomic.NewBool(false),
		MetricsRegisterer:                    nil,
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

	// Create gRPC server with max message size configuration.
	// Default to 1 GiB for responses (instead of standard 4 MiB) to handle large proofs that can exceed 4MB.
	// This was increased to fix "grpc: received message larger than max" errors when generating
	// proofs for blocks with many state changes.
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(int(*maxRequestSize)),
		grpc.MaxSendMsgSize(int(*maxResponseSize)),
	)

	// Create and register ledger service
	ledgerService := remote.NewService(ledgerStorage, logger)
	ledgerpb.RegisterLedgerServiceServer(grpcServer, ledgerService)

	// Create listeners based on provided flags
	type listenerInfo struct {
		listener     net.Listener
		address      string
		socketPath   string
		isUnixSocket bool
	}
	var listeners []listenerInfo
	var socketPaths []string

	// Create TCP listener if TCP address is provided
	if *ledgerServiceTCP != "" {
		lis, err := net.Listen("tcp", *ledgerServiceTCP)
		if err != nil {
			logger.Fatal().Err(err).Str("address", *ledgerServiceTCP).Msg("failed to listen on TCP")
		}

		logger.Info().Str("address", *ledgerServiceTCP).Msg("gRPC server listening on TCP")
		listeners = append(listeners, listenerInfo{
			listener:     lis,
			address:      *ledgerServiceTCP,
			socketPath:   "",
			isUnixSocket: false,
		})
	}

	// Create Unix socket listeners if socket path(s) are provided
	if *ledgerServiceSocket != "" {
		// Support multiple socket paths separated by comma
		socketPathsList := strings.Split(*ledgerServiceSocket, ",")
		for _, socketPath := range socketPathsList {
			socketPath = strings.TrimSpace(socketPath)
			if socketPath == "" {
				continue
			}

			// Ensure the socket directory exists
			socketDir := filepath.Dir(socketPath)
			if socketDir != "" && socketDir != "." {
				if err := os.MkdirAll(socketDir, 0755); err != nil {
					logger.Fatal().Err(err).Str("socket_dir", socketDir).Msg("failed to create socket directory")
				}
			}

			// Clean up any existing socket file
			if _, err := os.Stat(socketPath); err == nil {
				logger.Info().Str("socket_path", socketPath).Msg("removing existing socket file")
				if err := os.Remove(socketPath); err != nil {
					logger.Warn().Err(err).Str("socket_path", socketPath).Msg("failed to remove existing socket file")
				}
			}

			lis, err := net.Listen("unix", socketPath)
			if err != nil {
				logger.Fatal().Err(err).Str("socket_path", socketPath).Msg("failed to listen on Unix socket")
			}

			// Set socket file permissions (readable/writable by owner and group)
			if err := os.Chmod(socketPath, 0660); err != nil {
				logger.Warn().Err(err).Str("socket_path", socketPath).Msg("failed to set socket file permissions")
			}

			logger.Info().Str("socket_path", socketPath).Msg("gRPC server listening on Unix domain socket")
			socketPaths = append(socketPaths, socketPath)
			listeners = append(listeners, listenerInfo{
				listener:     lis,
				address:      socketPath,
				socketPath:   socketPath,
				isUnixSocket: true,
			})
		}
	}

	// Start server on all listeners in separate goroutines
	errCh := make(chan error, len(listeners))
	for _, info := range listeners {
		info := info // capture loop variable
		go func() {
			if err := grpcServer.Serve(info.listener); err != nil {
				errCh <- fmt.Errorf("gRPC server error on %s: %w", info.address, err)
			}
		}()
	}

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

	// Clean up Unix socket files
	for _, socketPath := range socketPaths {
		if socketPath != "" {
			if err := os.Remove(socketPath); err != nil {
				logger.Warn().Err(err).Str("socket_path", socketPath).Msg("failed to remove socket file")
			} else {
				logger.Info().Str("socket_path", socketPath).Msg("removed socket file")
			}
		}
	}

	logger.Info().Msg("waiting for ledger to stop...")
	<-ledgerStorage.Done()

	logger.Info().Msg("ledger service stopped")
}
