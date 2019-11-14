package server

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dapperlabs/flow-go/sdk/emulator/storage/badger"

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
	store      badger.Store
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
	// DBPath is the path to the Badger database on disk
	DBPath string
}

// NewEmulatorServer creates a new instance of a Flow Emulator server.
func NewEmulatorServer(logger *log.Logger, store badger.Store, conf *Config) *EmulatorServer {

	messageLogger := func(msg string) {
		logger.Debug(msg)
	}

	eventStore := events.NewMemStore()

	eventEmitter := func(event flow.Event, blockNumber uint64, txHash crypto.Hash) {
		logger.
			WithField("eventType", event.Type).
			Infof("🔔  Event emitted: %s", event)

		ctx := context.Background()
		err := eventStore.Add(ctx, blockNumber, event)
		if err != nil {
			logger.WithError(err).Errorf("Failed to save event %s", event.Type)
		}
	}

	options := []emulator.Option{
		emulator.WithRuntimeLogger(messageLogger),
		emulator.WithEventEmitter(eventEmitter),
		emulator.WithStore(store),
	}
	if conf.RootAccountKey != nil {
		options = append(options, emulator.WithRootAccountKey(*conf.RootAccountKey))
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
			blockchain: emulator.NewEmulatedBlockchain(options...),
			logger:     logger,
			eventStore: eventStore,
		},
		grpcServer: grpcServer,
		config:     conf,
		logger:     logger,
		store:      store,
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
			block, err := b.backend.blockchain.CommitBlock()
			if err != nil {
				b.logger.WithError(err).Error("Failed to commit block")
			} else {
				b.logger.WithFields(log.Fields{
					"blockNum":  block.Number,
					"blockHash": block.Hash().Hex(),
					"blockSize": len(block.TransactionHashes),
				}).Debugf("⛏  Block #%d mined", block.Number)
			}

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

	store, err := badger.New(config.DBPath)
	if err != nil {
		logger.WithError(err).Fatal("☠️  Failed to set up Emulator server")
	}

	emulatorServer := NewEmulatorServer(logger, store, config)
	defer emulatorServer.cleanup()
	go emulatorServer.handleSIGTERM()

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

// cleanup cleans up the server.
// This MUST be called before the server process terminates.
func (e *EmulatorServer) cleanup() {
	if err := e.store.Close(); err != nil {
		e.logger.WithError(err).Error("Cleanup failed: could not close store")
	}
}

// handleSIGTERM waits for a SIGTERM, then cleans up the server's resources.
// This should be run as a goroutine.
func (e *EmulatorServer) handleSIGTERM() {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGTERM)
	<-c
	e.cleanup()
}
