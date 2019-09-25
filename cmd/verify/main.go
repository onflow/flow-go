package main

import "github.com/dapperlabs/flow-go/internal/roles/verify"

func main() {
	server, err := verify.InitializeServer()
	if err != nil {
		panic(err)
	}
	server.Start()
}
