package main

import "github.com/dapperlabs/bamboo-node/internal/roles/consensus"

func main() {
	server, err := consensus.InitializeServer()
	if err != nil {
		panic(err)
	}
	server.Start()
}
