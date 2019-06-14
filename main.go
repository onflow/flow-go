package main

import (
	"context"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dapperlabs/bamboo-emulator/data"
	"github.com/dapperlabs/bamboo-emulator/nodes/access"
	"github.com/dapperlabs/bamboo-emulator/nodes/security"
	"github.com/dapperlabs/bamboo-emulator/server"
)

func main() {
	// TODO: create CLI app to run the emulator
	// TODO: add configuration options: env vars and CLI flags
	port := 5000

	log := logrus.New()
	log.Formatter = new(logrus.TextFormatter)
	log.Out = os.Stdout

	log.WithField("port", port).Info("Starting emulator server...")

	collections := make(chan *data.Collection, 16)

	state := data.NewWorldState()

	accessNode := access.NewNode(
		&access.Config{
			CollectionInterval: time.Second,
		},
		state,
		collections,
		log,
	)
	securityNode := security.NewNode(
		&security.Config{
			BlockInterval: time.Second,
		},
		state,
		collections,
		log,
	)

	emulatorServer := server.NewServer(accessNode)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go accessNode.Start(ctx)
	go securityNode.Start(ctx)

	emulatorServer.Start(port)
}
