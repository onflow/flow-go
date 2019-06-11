package main

import (
	"context"
	"fmt"

	"github.com/dapperlabs/bamboo-emulator/nodes/access"
	"github.com/dapperlabs/bamboo-emulator/server"
)

func main() {
	port := 5000

	fmt.Printf("Starting server on port %d...\n", port)
	fmt.Println("Listening for transactions ...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	accessNode := access.NewNode()

	emulatorServer := server.NewServer(accessNode)

	go accessNode.Start(ctx)

	emulatorServer.Start(port)
}
