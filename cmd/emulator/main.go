package main

import (
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/dapperlabs/bamboo-node/internal/emulator/server"
)

func main() {
	logger := log.New()
	logger.SetLevel(log.DebugLevel)

	config := &server.Config{
		Port:          5000,
		HTTPPort:      9090,
		BlockInterval: time.Second * 5,
	}

	server.StartServer(logger, config)
}
