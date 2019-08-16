package server

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
)

// TODO: improve (currently a utility to run EmulatorServer)
func TestWrappedServer(t *testing.T) {
	RegisterTestingT(t)

	logger := log.New()
	logger.SetLevel(log.DebugLevel)

	config := &Config{
		Port:          5000,
		WrappedPort:   9090,
		BlockInterval: time.Second * 5,
	}

	StartServer(logger, config)
}
