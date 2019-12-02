package server

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/proto/services/observation"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/emulator/storage/badger"
	"github.com/dapperlabs/flow-go/utils/liveness"
	grpcprometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// EmulatorServer is a local server that runs a Flow Emulator instance.
//
// The server wraps an EmulatedBlockchain instance with the Observation gRPC interface.
type EmulatorServer struct {
	backend       *Backend
	grpcServer    *grpc.Server
	config        *Config
	logger        *log.Logger
	store         badger.Store
	livenessCheck *liveness.CheckCollector

	// Wraps the cleanup function to ensure we only run cleanup once
	cleanupOnce sync.Once
}

const (
	defaultBlockInterval          = 5 * time.Second
	defaultLivenessCheckTolerance = time.Second
	defaultPort                   = 3569
	defaultHTTPPort               = 8080
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
	// LivenessCheckTolerance is the tolerence level of the liveness check
	// e.g. how long we can go without answering before being considered not alive
	LivenessCheckTolerance time.Duration
}

// NewEmulatorServer creates a new instance of a Flow Emulator server.
func NewEmulatorServer(logger *log.Logger, store badger.Store, conf *Config) *EmulatorServer {

	messageLogger := func(msg string) {
		logger.Debug(msg)
	}

	options := []emulator.Option{
		emulator.WithRuntimeLogger(messageLogger),
		emulator.WithStore(store),
	}
	if conf.RootAccountKey != nil {
		options = append(options, emulator.WithRootAccountKey(*conf.RootAccountKey))
	}

	if conf.BlockInterval == 0 {
		conf.BlockInterval = defaultBlockInterval
	}

	if conf.LivenessCheckTolerance == 0 {
		conf.LivenessCheckTolerance = defaultLivenessCheckTolerance
	}

	if conf.Port == 0 {
		conf.Port = defaultPort
	}

	if conf.HTTPPort == 0 {
		conf.HTTPPort = defaultHTTPPort
	}

	blockchain, err := emulator.NewEmulatedBlockchain(options...)
	if err != nil {
		logger.WithError(err).Fatal("Failed to initialize blockchain")
	}

	grpcServer := grpc.NewServer(
		grpc.StreamInterceptor(grpcprometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpcprometheus.UnaryServerInterceptor),
	)

	server := &EmulatorServer{
		backend: &Backend{
			blockchain: blockchain,
			logger:     logger,
		},
		grpcServer:    grpcServer,
		config:        conf,
		logger:        logger,
		store:         store,
		livenessCheck: liveness.NewCheckCollector(conf.LivenessCheckTolerance),
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
	grpcprometheus.Register(server.grpcServer)
	return server
}

// Start spins up the Flow Emulator server instance.
//
// This function starts a gRPC server to listen for requests and process incoming transactions.
// By default, the Flow Emulator server automatically mines a block every BlockInterval.
func (e *EmulatorServer) Start(ctx context.Context) {
	// Start gRPC server in a separate goroutine to continually listen for requests
	go e.startGrpcServer()

	ticker := time.NewTicker(e.config.BlockInterval)
	livenessTicker := time.NewTicker(e.config.LivenessCheckTolerance / 2)
	checker := e.livenessCheck.NewCheck()
	defer ticker.Stop()
	defer livenessTicker.Stop()
	for {
		select {
		case <-ticker.C:
			block, err := e.backend.blockchain.CommitBlock()
			if err != nil {
				e.logger.WithError(err).Error("Failed to commit block")
			} else {
				e.logger.WithFields(log.Fields{
					"blockNum":  block.Number,
					"blockHash": block.Hash().Hex(),
					"blockSize": len(block.TransactionHashes),
				}).Debugf("⛏  Block #%d mined", block.Number)
			}
		case <-livenessTicker.C:
			checker.CheckIn()
		case <-ctx.Done():
			return
		}
	}
}

func (e *EmulatorServer) startGrpcServer() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", e.config.Port))
	if err != nil {
		e.logger.WithError(err).Fatal("☠️  Failed to start Emulator Server")
	}

	err = e.grpcServer.Serve(lis)
	if err != nil {
		e.logger.WithError(err).Fatal("☠️  Failed to serve GRPC Service")
	}
}

// StartServer sets up a wrapped instance of the Flow Emulator server and starts it.
func StartServer(logger *log.Logger, config *Config) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := badger.New(
		badger.WithPath(config.DBPath),
		badger.WithLogger(logger),
		badger.WithTruncate(true))
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

	// Add /metrics endponit
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/live", emulatorServer.livenessCheck)
	mux.Handle("/", http.HandlerFunc(wrappedServer.ServeHTTP))

	httpServer := http.Server{
		Addr:    fmt.Sprintf(":%d", config.HTTPPort),
		Handler: mux,
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
	e.cleanupOnce.Do(func() {
		if err := e.store.Close(); err != nil {
			e.logger.WithError(err).Error("Cleanup failed: could not close store")
		}
	})
}

// handleSIGTERM waits for a SIGTERM, then cleans up the server's resources.
// This should be run as a goroutine.
func (e *EmulatorServer) handleSIGTERM() {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGTERM)
	<-c
	e.cleanup()
}
