package main

import "github.com/dapperlabs/bamboo-node/internal/roles/verify"

func main() {
	server, err := verify.InitializeServer()
	if err != nil {
		panic(err)
	}
	server.Start()
}
