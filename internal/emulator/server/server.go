package server

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dapperlabs/bamboo-node/internal/emulator/core"
)

type EmulatorServer struct {
	blockchain *core.EmulatedBlockchain
	config     *Config
	log        *logrus.Logger
}

type Config struct {
	Port          int
	BlockInterval time.Duration
}

func NewEmulatorServer(log *logrus.Logger, config Config) *EmulatorServer {
	return &EmulatorServer{
		blockchain: core.NewEmulatedBlockchain(),
		config:     config,
		log:        log,
	}
}

func (s *EmulatorServer) Start() {
	// TODO: connect gRPC interface to server
}

func StartServer(log *logrus.Logger, config Config) {
	emulatorServer := NewEmulatorServer(log, config)
	emulatorServer.Start()
}
