package main

import (
	"fmt"

	"github.com/dapperlabs/bamboo-emulator/server"
)

func main() {
	port := 5000

	fmt.Printf("Starting server on port %d...\n", port)
	fmt.Println("Listening for transactions ...")

	server.Start(port)
}
