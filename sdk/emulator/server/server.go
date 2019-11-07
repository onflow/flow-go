package server

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/proto/services/observation"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/emulator/events"
	"google.golang.org/grpc/reflection"
)

// EmulatorServer is a local server that runs a Flow Emulator instance.
//
// The server wraps an EmulatedBlockchain instance with the Observation gRPC interface.
type EmulatorServer struct {
	backend    *Backend
	grpcServer *grpc.Server
	config     *Config
	logger     *log.Logger
}

const (
	defaultBlockInterval = 5 * time.Second
	defaultPort          = 3569
	defaultHTTPPort      = 8080
)

// Config is the configuration for an emulator server.
type Config struct {
	Port           int
	HTTPPort       int
	BlockInterval  time.Duration
	RootAccountKey *flow.AccountPrivateKey
	GRPCDebug      bool
}

// NewEmulatorServer creates a new instance of a Flow Emulator server.
func NewEmulatorServer(logger *log.Logger, conf *Config) *EmulatorServer {
	options := emulator.DefaultOptions

	if conf.RootAccountKey != nil {
		options.RootAccountKey = conf.RootAccountKey
	}

	options.OnLogMessage = func(msg string) {
		logger.Debug(msg)
	}

	eventStore := events.NewMemStore()

	options.OnEventEmitted = func(event flow.Event, blockNumber uint64, txHash crypto.Hash) {
		logger.
			WithField("eventType", event.Type).
			Infof("🔔  Event emitted: %s", event)

		ctx := context.Background()
		err := eventStore.Add(ctx, blockNumber, event)
		if err != nil {
			logger.WithError(err).Errorf("Failed to save event %s", event.Type)
		}
	}

	if conf.BlockInterval == 0 {
		conf.BlockInterval = defaultBlockInterval
	}

	if conf.Port == 0 {
		conf.Port = defaultPort
	}

	if conf.HTTPPort == 0 {
		conf.HTTPPort = defaultHTTPPort
	}

	grpcServer := grpc.NewServer()
	server := &EmulatorServer{
		backend: &Backend{
			blockchain: emulator.NewEmulatedBlockchain(options),
			logger:     logger,
			eventStore: eventStore,
		},
		grpcServer: grpcServer,
		config:     conf,
		logger:     logger,
	}

	if conf.GRPCDebug {
		reflection.Register(grpcServer)
	}

	address := server.backend.blockchain.RootAccountAddress()
	prKey := server.backend.blockchain.RootKey()
	prKeyBytes, _ := prKey.PrivateKey.Encode()

	logger.WithFields(log.Fields{
		"address": address.Hex(),
		"prKey":   hex.EncodeToString(prKeyBytes),
	}).Infof("⚙️   Using root account 0x%s", address.Hex())

	observation.RegisterObserveServiceServer(server.grpcServer, server.backend)
	return server
}

// Start spins up the Flow Emulator server instance.
//
// This function starts a gRPC server to listen for requests and process incoming transactions.
// By default, the Flow Emulator server automatically mines a block every BlockInterval.
func (b *EmulatorServer) Start(ctx context.Context) {
	// Start gRPC server in a separate goroutine to continually listen for requests
	go b.startGrpcServer()

	ticker := time.NewTicker(b.config.BlockInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			block := b.backend.blockchain.CommitBlock()

			b.logger.WithFields(log.Fields{
				"blockNum":  block.Number,
				"blockHash": block.Hash().Hex(),
				"blockSize": len(block.TransactionHashes),
			}).Debugf("⛏  Block #%d mined", block.Number)

		case <-ctx.Done():
			return
		}
	}
}

func (b *EmulatorServer) startGrpcServer() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", b.config.Port))
	if err != nil {
		b.logger.WithError(err).Fatal("☠️  Failed to start Emulator Server")
	}

	err = b.grpcServer.Serve(lis)
	if err != nil {
		b.logger.WithError(err).Fatal("☠️  Failed to serve GRPC Service")
	}
}

// StartServer sets up a wrapped instance of the Flow Emulator server and starts it.
func StartServer(logger *log.Logger, config *Config) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emulatorServer := NewEmulatorServer(logger, config)

	logger.
		WithField("port", config.Port).
		Infof("🌱  Starting Emulator server on port %d...", config.Port)

	go emulatorServer.Start(ctx)

	wrappedServer := grpcweb.WrapServer(
		emulatorServer.grpcServer,
		grpcweb.WithOriginFunc(func(origin string) bool { return true }),
	)

	httpServer := http.Server{
		Addr:    fmt.Sprintf(":%d", config.HTTPPort),
		Handler: http.HandlerFunc(wrappedServer.ServeHTTP),
	}

	logger.
		WithField("port", config.HTTPPort).
		Infof("🌱  Starting wrapped HTTP server on port %d...", config.HTTPPort)

	if err := httpServer.ListenAndServe(); err != nil {
		logger.WithError(err).Fatal("☠️  Failed to start HTTP Server")
	}
}
