package main

import (
	"github.com/dapperlabs/bamboo-node/internal/nodes/security"
)

func main() {
	server, err := security.InitializeServer()
	if err != nil {
		panic(err)
	}
	server.Start()
}
