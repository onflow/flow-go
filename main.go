package main

import (
	"fmt"

	"github.com/dapperlabs/bamboo-emulator/server"
)

func main() {
	fmt.Println("Starting server ...")
	fmt.Println("Listening for transactions ...")

	port := 5000

	server.Start(port)
}
