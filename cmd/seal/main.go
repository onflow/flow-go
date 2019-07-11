package main

import "github.com/dapperlabs/bamboo-node/internal/roles/seal"

func main() {
	server, err := seal.InitializeServer()
	if err != nil {
		panic(err)
	}
	server.Start()
}
