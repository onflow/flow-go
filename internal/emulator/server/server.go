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

	"github.com/dapperlabs/bamboo-node/pkg/grpc/services/observe"

	"github.com/dapperlabs/bamboo-node/internal/emulator/core"
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
	Port          int           `default:"5000"`
	WrappedPort   int           `default:"9090"`
	BlockInterval time.Duration `default:"5s"`
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
		s.logger.WithError(err).Fatal("Failed to listen")
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
	handler := func(resp http.ResponseWriter, req *http.Request) {
		wrappedServer.ServeHTTP(resp, req)
	}

	httpServer := http.Server{
		Addr:    fmt.Sprintf(":%d", config.WrappedPort),
		Handler: http.HandlerFunc(handler),
	}

	logger.Debugf("Starting server. http port: %d", config.WrappedPort)

	if err := httpServer.ListenAndServe(); err != nil {
		logger.Fatalf("failed starting http server: %v", err)
	}
}
