package main

import (
	"github.com/dapperlabs/bamboo-node/internal/execute"
)

func main() {
	server, err := execute.InitializeServer()
	if err != nil {
		panic(err)
	}
	server.Start()
}
