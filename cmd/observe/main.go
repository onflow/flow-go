package main

import "github.com/dapperlabs/bamboo-node/internal/roles/observe"

func main() {
	server, err := observe.InitializeServer()
	if err != nil {
		panic(err)
	}
	server.Start()
}
