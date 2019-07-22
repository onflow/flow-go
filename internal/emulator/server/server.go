package server

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dapperlabs/bamboo-node/internal/emulator/core"
	"github.com/dapperlabs/bamboo-node/pkg/types"
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
	// TODO: connect gRPC interface to server

	tick := time.Tick(s.config.BlockInterval)
	for {
		select {
		case tx := <-s.transactionsIn:
			s.blockchain.SubmitTransaction(tx)
			s.blockchain.CommitBlock()
		case <-tick:
			s.blockchain.CommitBlock()
		case <-ctx.Done():
			return
		}
	}
}

func StartServer(log *logrus.Logger, config *Config) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emulatorServer := NewEmulatorServer(log, config)
	emulatorServer.Start(ctx)
}
