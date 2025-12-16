package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip" // required for gRPC compression

	_ "github.com/onflow/flow-go/engine/common/grpc/compressor/deflate" // required for gRPC compression
	_ "github.com/onflow/flow-go/engine/common/grpc/compressor/snappy"  // required for gRPC compression

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/ledger/remote"
	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
	"github.com/onflow/flow-go/module/metrics"
)

var (
	walDir            = flag.String("wal-dir", "", "Directory for WAL files (required)")
	grpcListenAddr    = flag.String("grpc-addr", "0.0.0.0:9000", "gRPC server listen address")
	capacity          = flag.Int("capacity", 100, "Ledger capacity (number of tries)")
	checkpointDist    = flag.Uint("checkpoint-distance", 100, "Checkpoint distance")
	checkpointsToKeep = flag.Uint("checkpoints-to-keep", 3, "Number of checkpoints to keep")
)

func main() {
	flag.Parse()

	if *walDir == "" {
		fmt.Fprintf(os.Stderr, "error: -wal-dir is required\n")
		os.Exit(1)
	}

	logger := zerolog.New(os.Stderr).With().
		Timestamp().
		Str("service", "ledger").
		Logger()

	logger.Info().
		Str("wal_dir", *walDir).
		Str("grpc_addr", *grpcListenAddr).
		Int("capacity", *capacity).
		Msg("starting ledger service")

	// Create WAL
	metricsCollector := &metrics.NoopCollector{}
	diskWal, err := wal.NewDiskWAL(
		logger,
		nil,
		metricsCollector,
		*walDir,
		*capacity,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create WAL")
	}

	// Create compactor config
	compactorConfig := &ledger.CompactorConfig{
		CheckpointCapacity:                   uint(*capacity),
		CheckpointDistance:                   *checkpointDist,
		CheckpointsToKeep:                    *checkpointsToKeep,
		TriggerCheckpointOnNextSegmentFinish: atomic.NewBool(false),
		Metrics:                              metricsCollector,
	}

	// Create ledger factory
	factory := complete.NewLocalLedgerFactory(
		diskWal,
		*capacity,
		compactorConfig,
		metricsCollector,
		logger.With().Str("component", "ledger").Logger(),
		complete.DefaultPathFinderVersion,
	)

	// Create ledger instance
	ledgerStorage, err := factory.NewLedger()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create ledger")
	}

	// Wait for ledger to be ready (WAL replay)
	logger.Info().Msg("waiting for ledger initialization...")
	<-ledgerStorage.Ready()
	logger.Info().Msg("ledger ready")

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Create and register ledger service
	ledgerService := remote.NewService(ledgerStorage, logger)
	ledgerpb.RegisterLedgerServiceServer(grpcServer, ledgerService)

	// Start gRPC server
	lis, err := net.Listen("tcp", *grpcListenAddr)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to listen")
	}

	logger.Info().Str("address", *grpcListenAddr).Msg("gRPC server listening")

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

	logger.Info().Msg("waiting for ledger to stop...")
	<-ledgerStorage.Done()

	logger.Info().Msg("ledger service stopped")
}
