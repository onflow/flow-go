package main

import "github.com/dapperlabs/flow-go/internal/roles/observe"

func main() {
	server, err := observe.InitializeServer()
	if err != nil {
		panic(err)
	}
	server.Start()
}
