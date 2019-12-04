package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dapperlabs/flow-go/sdk/templates"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/keys"

	"github.com/dapperlabs/flow-go/language/tools/language-server/protocol"
)

const (
	CommandSubmitTransaction   = "cadence.server.submitTransaction"
	CommandExecuteScript       = "cadence.server.executeScript"
	CommandUpdateAccountCode   = "cadence.server.updateAccountCode"
	CommandCreateAccount       = "cadence.server.createAccount"
	CommandSwitchActiveAccount = "cadence.server.switchActiveAccount"
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
							CommandUpdateAccountCode,
							CommandCreateAccount,
							CommandSwitchActiveAccount,
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
	s.commands[CommandUpdateAccountCode] = s.updateAccountCode
	s.commands[CommandSwitchActiveAccount] = s.switchActiveAccount
	s.commands[CommandCreateAccount] = s.createAccount
}

// submitTransaction handles submitting a transaction defined in the
// source document in VS Code.
//
// There should be exactly 1 argument:
//   * the DocumentURI of the file to submit
func (s *Server) submitTransaction(conn protocol.Conn, args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must have 1 args, got: %d", len(args))
	}
	uri, ok := args[0].(string)
	if !ok {
		return nil, errors.New("invalid uri argument")
	}
	doc, ok := s.documents[protocol.DocumentUri(uri)]
	if !ok {
		return nil, fmt.Errorf("could not find document for URI %s", uri)
	}

	tx := flow.Transaction{
		Script:         []byte(doc.text),
		Nonce:          s.getNextNonce(),
		ComputeLimit:   10,
		PayerAccount:   s.activeAccount,
		ScriptAccounts: []flow.Address{s.activeAccount},
	}

	err := s.sendTransaction(conn, tx)
	return nil, err
}

// executeScript handles executing a script defined in the source document in
// VS Code.
//
// There should be exactly 1 argument:
//   * the DocumentURI of the file to submit
func (s *Server) executeScript(conn protocol.Conn, args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.New("missing argument")
	}
	uri, ok := args[0].(string)
	if !ok {
		return nil, errors.New("invalid uri argument")
	}
	doc, ok := s.documents[protocol.DocumentUri(uri)]
	if !ok {
		return nil, fmt.Errorf("could not find document for URI %s", uri)
	}

	script := []byte(doc.text)
	res, err := s.flowClient.ExecuteScript(context.Background(), script)
	if err == nil {
		conn.LogMessage(&protocol.LogMessageParams{
			Type:    protocol.Info,
			Message: fmt.Sprintf("Executed script with result: %v", res),
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
			conn.LogMessage(&protocol.LogMessageParams{
				Type:    protocol.Warning,
				Message: fmt.Sprintf("Failed to execute script: %s", grpcErr.Message()),
			})
			return nil, nil
		}
	}

	return nil, err
}

// switchActiveAccount sets the account that is currently active and should be
// used when submitting transactions.
func (s *Server) switchActiveAccount(conn protocol.Conn, args ...interface{}) (interface{}, error) {
	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Log,
		Message: fmt.Sprintf("set active acct %v", args),
	})

	if len(args) != 1 {
		return nil, fmt.Errorf("requires 1 argument, got %d", len(args))
	}
	addrHex, ok := args[0].(string)
	if !ok {
		return nil, errors.New("invalid argument")
	}
	addr := flow.HexToAddress(addrHex)

	_, ok = s.accounts[addr]
	if !ok {
		return nil, errors.New("cannot set active account that does not exist")
	}

	s.activeAccount = addr
	return nil, nil
}

// createAccount creates a new account and returns its address.
func (s *Server) createAccount(conn protocol.Conn, args ...interface{}) (interface{}, error) {
	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Log,
		Message: fmt.Sprintf("create acct args: %v", args),
	})

	if len(args) != 0 {
		return nil, fmt.Errorf("expecting 0 args got: %d", len(args))
	}

	accountKey := flow.AccountPublicKey{
		PublicKey: s.config.RootAccountKey.PrivateKey.PublicKey(),
		SignAlgo:  s.config.RootAccountKey.SignAlgo,
		HashAlgo:  s.config.RootAccountKey.HashAlgo,
		Weight:    keys.PublicKeyWeightThreshold,
	}
	script, err := templates.CreateAccount([]flow.AccountPublicKey{accountKey}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate account creation script: %w", err)
	}

	tx := flow.Transaction{
		Script:         script,
		Nonce:          s.getNextNonce(),
		ComputeLimit:   10,
		PayerAccount:   s.activeAccount,
		ScriptAccounts: []flow.Address{},
	}

	err = s.sendTransaction(conn, tx)
	if err != nil {
		return nil, err
	}

	minedTx, err := s.flowClient.GetTransaction(context.Background(), tx.Hash())
	if err != nil {
		return nil, err
	}
	if !(minedTx.Status == flow.TransactionFinalized || minedTx.Status == flow.TransactionSealed) {
		return nil, fmt.Errorf("failed to get account address for transaction with status %s", tx.Status)
	}
	if len(minedTx.Events) != 1 {
		return nil, fmt.Errorf("failed to get new account address for tx %s", tx.Hash().Hex())
	}
	accountCreatedEvent, err := flow.DecodeAccountCreatedEvent(minedTx.Events[0].Payload)
	if err != nil {
		return nil, err
	}
	addr := accountCreatedEvent.Address()

	s.accounts[addr] = s.config.RootAccountKey
	return addr, nil
}

// updateAccountCode updates the configured account with the code of the given
// file.
//
// There should be exactly 2 arguments:
//   * the DocumentURI of the file to submit
//   * the address of the account to sign with
func (s *Server) updateAccountCode(conn protocol.Conn, args ...interface{}) (interface{}, error) {
	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Log,
		Message: fmt.Sprintf("update acct code args: %v", args),
	})
	if len(args) != 1 {
		return nil, fmt.Errorf("must have 1 args, got: %d", len(args))
	}
	uri, ok := args[0].(string)
	if !ok {
		return nil, errors.New("invalid uri argument")
	}
	doc, ok := s.documents[protocol.DocumentUri(uri)]
	if !ok {
		return nil, fmt.Errorf("could not find document for URI %s", uri)
	}

	accountCode := []byte(doc.text)
	script := templates.UpdateAccountCode(accountCode)

	tx := flow.Transaction{
		Script:         script,
		Nonce:          s.getNextNonce(),
		ComputeLimit:   10,
		PayerAccount:   s.activeAccount,
		ScriptAccounts: []flow.Address{s.activeAccount},
	}

	err := s.sendTransaction(conn, tx)
	return nil, err
}

// sendTransaction sends a transaction with the given script, from the
// currently active account. Returns the hash of the transaction if it is
// successfully submitted.
//
// If an error occurs, attempts to show an appropriate message (either via logs
// or UI popups in the client).
func (s *Server) sendTransaction(conn protocol.Conn, tx flow.Transaction) error {
	key, ok := s.accounts[s.activeAccount]
	if !ok {
		return fmt.Errorf("cannot sign transaction for account with unknown address %s", s.activeAccount)
	}

	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Info,
		Message: fmt.Sprintf("submitting transaction %d", tx.Nonce),
	})

	sig, err := keys.SignTransaction(tx, key)
	if err != nil {
		return err
	}
	tx.AddSignature(s.activeAccount, sig)

	err = s.flowClient.SendTransaction(context.Background(), tx)
	if err == nil {
		conn.LogMessage(&protocol.LogMessageParams{
			Type:    protocol.Info,
			Message: fmt.Sprintf("Submitted transaction nonce=%d\thash=%s", tx.Nonce, tx.Hash().Hex()),
		})
		return nil
	}

	grpcErr, ok := status.FromError(err)
	if ok {
		if grpcErr.Code() == codes.Unavailable {
			// The emulator server isn't running
			conn.ShowMessage(&protocol.ShowMessageParams{
				Type:    protocol.Warning,
				Message: "The emulator server is unavailable. Please start the emulator (`cadence.runEmulator`) first.",
			})
			return nil
		} else if grpcErr.Code() == codes.InvalidArgument {
			// The request was invalid
			conn.ShowMessage(&protocol.ShowMessageParams{
				Type:    protocol.Warning,
				Message: "The transaction could not be submitted.",
			})
			conn.LogMessage(&protocol.LogMessageParams{
				Type:    protocol.Warning,
				Message: fmt.Sprintf("Failed to submit transaction: %s", grpcErr.Message()),
			})
			return nil
		}
	}

	return err
}
