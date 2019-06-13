package main

import (
	"context"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dapperlabs/bamboo-emulator/data"
	"github.com/dapperlabs/bamboo-emulator/nodes/access"
	"github.com/dapperlabs/bamboo-emulator/server"
)

func main() {
	port := 5000

	log := logrus.New()
	log.Formatter = new(logrus.TextFormatter)
	log.Out = os.Stdout

	log.WithFields(logrus.Fields{
		"port": port,
	}).Info("Starting emulator server...")

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

	emulatorServer := server.NewServer(accessNode)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go accessNode.Start(ctx)

	emulatorServer.Start(port)
}
