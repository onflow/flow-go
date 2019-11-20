package server

import (
	"fmt"

	"github.com/dapperlabs/flow-go/language/tools/language-server/protocol"
)

const (
	CommandSubmitTransaction = "cadence.submitTransaction"
)

// CommandHandler represents the form of functions that handle commands
// submitted from the client using workspace/executeCommand.
type CommandHandler func(conn protocol.Connection, args ...interface{}) (interface{}, error)

// Registers the commands that the server is able to handle.
//
// The best reference I've found for how this works is:
// https://stackoverflow.com/questions/43328582/how-to-implement-quickfix-via-a-language-server
func (s Server) registerCommands(connection protocol.Connection) {
	// Send a message to the client indicating which commands we support
	err := connection.RegisterCapability(&protocol.RegistrationParams{
		Registrations: []protocol.Registration{
			{
				ID:     "test",
				Method: "workspace/executeCommand",
				RegisterOptions: protocol.ExecuteCommandRegistrationOptions{
					ExecuteCommandOptions: protocol.ExecuteCommandOptions{
						Commands: []string{CommandSubmitTransaction},
					},
				},
			},
		},
	})
	if err != nil {
		connection.LogMessage(&protocol.LogMessageParams{
			Type:    protocol.Warning,
			Message: fmt.Sprintf("Failed to register command: %w", err.Error()),
		})
	}

	// Register each command handler function in the server
	s.commands[CommandSubmitTransaction] = s.submitTransaction
}

// submitTransaction handles submitting a transaction defined in the
// source document in VS Code.
func (s Server) submitTransaction(connection protocol.Connection, args ...interface{}) (interface{}, error) {
	connection.ShowMessage(&protocol.ShowMessageParams{
		Type:    protocol.Info,
		Message: "called submit transaction",
	})
	return nil, nil
}
