package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/keys"

	"github.com/dapperlabs/flow-go/language/tools/language-server/protocol"
)

const (
	CommandSubmitTransaction = "cadence.submitTransaction"
	CommandExecuteScript     = "cadence.executeScript"
)

// CommandHandler represents the form of functions that handle commands
// submitted from the client using workspace/executeCommand.
type CommandHandler func(conn protocol.Conn, args ...interface{}) (interface{}, error)

// Registers the commands that the server is able to handle.
//
// The best reference I've found for how this works is:
// https://stackoverflow.com/questions/43328582/how-to-implement-quickfix-via-a-language-server
func (s Server) registerCommands(conn protocol.Conn) {
	// Send a message to the client indicating which commands we support
	registration := protocol.RegistrationParams{
		Registrations: []protocol.Registration{
			{
				ID:     "registerCommand",
				Method: "workspace/executeCommand",
				RegisterOptions: protocol.ExecuteCommandRegistrationOptions{
					ExecuteCommandOptions: protocol.ExecuteCommandOptions{
						Commands: []string{
							CommandSubmitTransaction,
							CommandExecuteScript,
						},
					},
				},
			},
		},
	}

	// We have occasionally observed the client failing to recognize this
	// method if the request is sent too soon after the extension loads.
	// Retrying with a backoff avoids this problem.
	retryAfter := time.Millisecond * 100
	nRetries := 10
	for i := 0; i < nRetries; i++ {
		err := conn.RegisterCapability(&registration)
		if err == nil {
			break
		}
		conn.LogMessage(&protocol.LogMessageParams{
			Type: protocol.Warning,
			Message: fmt.Sprintf(
				"Failed to register command. Will retry %d more times... err: %s",
				nRetries-1-i, err.Error()),
		})
		time.Sleep(retryAfter)
		retryAfter *= 2
	}

	// Register each command handler function in the server
	s.commands[CommandSubmitTransaction] = s.submitTransaction
	s.commands[CommandExecuteScript] = s.executeScript
}

// submitTransaction handles submitting a transaction defined in the
// source document in VS Code.
//
// There should be exactly 1 argument, the DocumentURI of the file to submit.
func (s *Server) submitTransaction(conn protocol.Conn, args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.New("missing argument")
	}
	uri, ok := args[0].(string)
	if !ok {
		return nil, errors.New("invalid uri argument")
	}
	documentText, ok := s.documents[protocol.DocumentUri(uri)]
	if !ok {
		return nil, fmt.Errorf("could not find document for URI %s", uri)
	}

	tx := flow.Transaction{
		Script:         []byte(documentText),
		Nonce:          s.getNextNonce(),
		ComputeLimit:   10,
		PayerAccount:   s.config.AccountAddr,
		ScriptAccounts: []flow.Address{s.config.AccountAddr},
	}

	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Info,
		Message: fmt.Sprintf("submitting transaction %d", tx.Nonce),
	})

	sig, err := keys.SignTransaction(tx, s.config.AccountKey)
	if err != nil {
		return nil, err
	}
	tx.AddSignature(s.config.AccountAddr, sig)

	err = s.flowClient.SendTransaction(context.Background(), tx)
	if err == nil {
		conn.LogMessage(&protocol.LogMessageParams{
			Type:    protocol.Info,
			Message: fmt.Sprintf("Submitted transaction nonce=%d\thash=%s", tx.Nonce, tx.Hash().Hex()),
		})
		return nil, nil
	}

	grpcErr, ok := status.FromError(err)
	if ok {
		if grpcErr.Code() == codes.Unavailable {
			// The emulator server isn't running
			conn.ShowMessage(&protocol.ShowMessageParams{
				Type:    protocol.Warning,
				Message: "The emulator server is unavailable. Please start the emulator (`cadence.runEmulator`) first.",
			})
			return nil, nil
		} else if grpcErr.Code() == codes.InvalidArgument {
			// The request was invalid
			conn.ShowMessage(&protocol.ShowMessageParams{
				Type:    protocol.Warning,
				Message: "The transaction could not be submitted.",
			})
			return nil, nil
		}
	}

	return nil, err
}

// executeScript handles executing a script defined in the source document in
// VS Code.
//
// There should be exactly 1 argument, the DocumentURI of the file to submit.
func (s *Server) executeScript(conn protocol.Conn, args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.New("missing argument")
	}
	uri, ok := args[0].(string)
	if !ok {
		return nil, errors.New("invalid uri argument")
	}
	documentText, ok := s.documents[protocol.DocumentUri(uri)]
	if !ok {
		return nil, fmt.Errorf("could not find document for URI %s", uri)
	}

	script := []byte(documentText)
	res, err := s.flowClient.ExecuteScript(context.Background(), script)
	if err == nil {
		conn.LogMessage(&protocol.LogMessageParams{
			Type:    protocol.Info,
			Message: fmt.Sprintf("Executed Script with result: %v", res),
		})
		return res, nil
	}

	grpcErr, ok := status.FromError(err)
	if ok {
		if grpcErr.Code() == codes.Unavailable {
			// The emulator server isn't running
			conn.ShowMessage(&protocol.ShowMessageParams{
				Type:    protocol.Warning,
				Message: "The emulator server is unavailable. Please start the emulator (`cadence.runEmulator`) first.",
			})
			return nil, nil
		} else if grpcErr.Code() == codes.InvalidArgument {
			// The request was invalid
			conn.ShowMessage(&protocol.ShowMessageParams{
				Type:    protocol.Warning,
				Message: "The script could not be executed.",
			})
			return nil, nil
		}
	}

	return nil, err
}
