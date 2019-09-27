package main

import "github.com/dapperlabs/flow-go/internal/roles/execute"

func main() {
	server, err := execute.InitializeServer()
	if err != nil {
		panic(err)
	}
	server.Start()
}
