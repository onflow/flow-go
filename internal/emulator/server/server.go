package server

import (
	"context"
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/dapperlabs/bamboo-node/pkg/grpc/services/observe"

	"github.com/dapperlabs/bamboo-node/internal/emulator/core"
)

// EmulatorServer is a local server that runs a Bamboo Emulator instance.
//
// The server wraps the Emulator Core Library with the Observation gRPC interface.
type EmulatorServer struct {
	blockchain *core.EmulatedBlockchain
	config     *Config
	logger     *log.Logger
}

// Config is EmulatorServer configuration settings.
type Config struct {
	Port          int           `default:"5000"`
	BlockInterval time.Duration `default:"5s"`
}

// NewEmulatorServer creates a new instance of a Bamboo Emulator server.
func NewEmulatorServer(logger *log.Logger, config *Config) *EmulatorServer {
	return &EmulatorServer{
		blockchain: core.NewEmulatedBlockchain(core.DefaultOptions),
		config:     config,
		logger:     logger,
	}
}

// Start spins up the Bamboo Emulator server instance.
//
// Starts a gRPC server to listen for incoming requests and process incoming transactions.
// By defualt, the Bamboo Emulator server automatically mines a block every BlockInteval.
func (s *EmulatorServer) Start(ctx context.Context) {
	s.logger.WithFields(log.Fields{
		"port": s.config.Port,
	}).Infof("🌱  Starting Emulator Server on port %d...", s.config.Port)

	// Start gRPC server in a separate goroutine to continually listen for requests
	go s.startGrpcServer()

	tick := time.Tick(s.config.BlockInterval)
	for {
		select {
		case <-tick:
			block := s.blockchain.CommitBlock()

			s.logger.WithFields(log.Fields{
				"blockNum":  block.Number,
				"blockHash": block.Hash(),
				"blockSize": len(block.TransactionHashes),
			}).Tracef("️⛏  Block #%d mined", block.Number)

		case <-ctx.Done():
			return
		}
	}
}

func (s *EmulatorServer) startGrpcServer() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.Port))
	if err != nil {
		s.logger.WithError(err).Fatal("Failed to listen")
	}

	grpcServer := grpc.NewServer()
	observe.RegisterObserveServiceServer(grpcServer, s)
	grpcServer.Serve(lis)
}

// StartServer sets up an instance of the Bamboo Emulator server and starts it.
func StartServer(logger *log.Logger, config *Config) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emulatorServer := NewEmulatorServer(logger, config)
	emulatorServer.Start(ctx)
}
