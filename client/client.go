package client

import (
	"fmt"

	"github.com/dapperlabs/bamboo-emulator/server"
)

type Client struct {
	Server server.Server
}

// Connect connects to a locally running Bamboo Emulator server.
func Connect() {
	// TODO: connect to Bamboo Local Server
	// also creates an HD wallet for client
	fmt.Println("connected to access node!")
}

// LogCommands displays all the usable commands to a client. 
func LogCommands() {
	// TODO: log all help commands available to Client
	fmt.Println("here are all the commands you can use!")
}

// CreateAccount creates an account for the client.
func CreateAccount() {
	// TODO: create a new account in the client's wallet
}
