package server

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/dapperlabs/bamboo-node/grpc/services/observe"
	"github.com/dapperlabs/bamboo-node/pkg/types"

	"github.com/dapperlabs/bamboo-node/internal/emulator/core"
)

type EmulatorServer struct {
	blockchain     *core.EmulatedBlockchain
	transactionsIn chan *types.SignedTransaction
	config         *Config
	log            *logrus.Logger
}

type Config struct {
	Port          int           `default:"5000"`
	BlockInterval time.Duration `default:"5s"`
}

func NewEmulatorServer(log *logrus.Logger, config *Config) *EmulatorServer {
	return &EmulatorServer{
		blockchain:     core.NewEmulatedBlockchain(),
		transactionsIn: make(chan *types.SignedTransaction, 16),
		config:         config,
		log:            log,
	}
}

func (s *EmulatorServer) Start(ctx context.Context) {
	s.log.WithFields(logrus.Fields{
		"port": s.config.Port,
	}).Infof("🌱  Starting Emulator Server on port %d...", s.config.Port)

	go s.startGrpcServer()

	tick := time.Tick(s.config.BlockInterval)
	for {
		select {
		case tx := <-s.transactionsIn:
			s.blockchain.SubmitTransaction(tx)

			s.log.WithFields(logrus.Fields{
				"txHash": tx.Hash(),
			}).Infof("💸  Transaction %s submitted to network", tx.Hash())

			hash := s.blockchain.CommitBlock()

			s.log.WithFields(logrus.Fields{
				"stateHash": hash,
			}).Infof("️⛏  Block %s mined", hash)
		case <-tick:
			hash := s.blockchain.CommitBlock()

			s.log.WithFields(logrus.Fields{
				"stateHash": hash,
			}).Tracef("️⛏  Block %s mined", hash)
		case <-ctx.Done():
			return
		}
	}
}

func (s *EmulatorServer) startGrpcServer() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.Port))
	if err != nil {
		s.log.WithError(err).Fatal("Failed to listen")
	}

	grpcServer := grpc.NewServer()
	observe.RegisterObserveServiceServer(grpcServer, s)
	grpcServer.Serve(lis)
}

func StartServer(log *logrus.Logger, config *Config) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emulatorServer := NewEmulatorServer(log, config)
	emulatorServer.Start(ctx)
}
