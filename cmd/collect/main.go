package main

import "github.com/dapperlabs/flow-go/internal/roles/collect"

func main() {
	server, err := collect.InitializeServer()
	if err != nil {
		panic(err)
	}
	server.Start()
}
