package main

import (
	"context"
	"fmt"

	"github.com/dapperlabs/bamboo-emulator/data"
	"github.com/dapperlabs/bamboo-emulator/nodes/access"
	"github.com/dapperlabs/bamboo-emulator/nodes/security"
	"github.com/dapperlabs/bamboo-emulator/server"
)

func main() {
	port := 5000

	fmt.Printf("Starting server on port %d...\n", port)

	collections := make(chan *data.Collection, 16)

	state := data.NewWorldState()

	accessNode := access.NewNode(state, collections)
	securityNode := security.NewNode(state, collections)

	emulatorServer := server.NewServer(accessNode)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go accessNode.Start(ctx)
	go securityNode.Start(ctx)

	emulatorServer.Start(port)
}
