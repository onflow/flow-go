package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/dapperlabs/bamboo-node/internal/emulator/core"
	"github.com/dapperlabs/bamboo-node/pkg/grpc/services/observe"
)

// EmulatorServer is a local server that runs a Bamboo Emulator instance.
//
// The server wraps the Emulator Core Library with the Observation gRPC interface.
type EmulatorServer struct {
	blockchain *core.EmulatedBlockchain
	grpcServer *grpc.Server
	config     *Config
	logger     *log.Logger
}

// Config for the EmulatorServer configuration settings.
type Config struct {
	Port          int
	HTTPPort      int
	BlockInterval time.Duration
}

// NewEmulatorServer creates a new instance of a Bamboo Emulator server.
func NewEmulatorServer(logger *log.Logger, config *Config) *EmulatorServer {
	server := &EmulatorServer{
		blockchain: core.NewEmulatedBlockchain(core.DefaultOptions),
		grpcServer: grpc.NewServer(),
		config:     config,
		logger:     logger,
	}

	observe.RegisterObserveServiceServer(server.grpcServer, server)
	return server
}

// Start spins up the Bamboo Emulator server instance.
//
// This function starts a gRPC server to listen for requests and process incoming transactions.
// By default, the Bamboo Emulator server automatically mines a block every BlockInterval.
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
		s.logger.WithError(err).Fatal("☠️  Failed to start Emulator Server")
	}

	s.grpcServer.Serve(lis)
}

// StartServer sets up a wrapped instance of the Bamboo Emulator server and starts it.
func StartServer(logger *log.Logger, config *Config) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emulatorServer := NewEmulatorServer(logger, config)
	go emulatorServer.Start(ctx)

	wrappedServer := grpcweb.WrapServer(emulatorServer.grpcServer)

	httpServer := http.Server{
		Addr:    fmt.Sprintf(":%d", config.HTTPPort),
		Handler: http.HandlerFunc(wrappedServer.ServeHTTP),
	}

	logger.
		WithField("port", config.HTTPPort).
		Debugf("🏁  Starting HTTP Server on port %d...", config.HTTPPort)

	if err := httpServer.ListenAndServe(); err != nil {
		logger.WithError(err).Fatal("☠️  Failed to start HTTP Server")
	}
}
