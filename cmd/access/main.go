package main

import (
	"github.com/dapperlabs/bamboo-node/internal/nodes/access"
)

func main() {
	server, err := access.InitializeServer()
	if err != nil {
		panic(err)
	}
	server.Start()
}
