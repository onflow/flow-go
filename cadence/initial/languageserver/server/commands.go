/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package server

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"

	"github.com/onflow/cadence/languageserver/protocol"
)

const (
	CommandSubmitTransaction     = "cadence.server.submitTransaction"
	CommandExecuteScript         = "cadence.server.executeScript"
	CommandUpdateAccountCode     = "cadence.server.updateAccountCode"
	CommandCreateAccount         = "cadence.server.createAccount"
	CommandCreateDefaultAccounts = "cadence.server.createDefaultAccounts"
	CommandSwitchActiveAccount   = "cadence.server.switchActiveAccount"
)

// CommandHandler represents the form of functions that handle commands
// submitted from the client using workspace/executeCommand.
type CommandHandler func(conn protocol.Conn, args ...interface{}) (interface{}, error)

// Registers the commands that the server is able to handle.
//
// The best reference I've found for how this works is:
// https://stackoverflow.com/questions/43328582/how-to-implement-quickfix-via-a-language-server
func (s *Server) registerCommands(conn protocol.Conn) {
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
							CommandCreateDefaultAccounts,
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
	s.commands[CommandCreateDefaultAccounts] = s.createDefaultAccounts
}

// submitTransaction handles submitting a transaction defined in the
// source document in VS Code.
//
// There should be exactly 1 argument:
//   * the DocumentURI of the file to submit
func (s *Server) submitTransaction(conn protocol.Conn, args ...interface{}) (interface{}, error) {
	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Log,
		Message: fmt.Sprintf("submit transaction args: %v", args),
	})

	expectedArgCount := 1
	if len(args) != expectedArgCount {
		return nil, fmt.Errorf("expecting %d arguments, got %d", expectedArgCount, len(args))
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

	_, err := s.sendTransactionHelper(conn, script, true)
	return nil, err
}

// executeScript handles executing a script defined in the source document in
// VS Code.
//
// There should be exactly 1 argument:
//   * the DocumentURI of the file to submit
func (s *Server) executeScript(conn protocol.Conn, args ...interface{}) (interface{}, error) {
	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Log,
		Message: fmt.Sprintf("execute script args: %v", args),
	})

	expectedArgCount := 1
	if len(args) != expectedArgCount {
		return nil, fmt.Errorf("expecting %d arguments, got %d", expectedArgCount, len(args))
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
	res, err := s.flowClient.ExecuteScriptAtLatestBlock(context.Background(), script)
	if err != nil {

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
		} else {
			conn.LogMessage(&protocol.LogMessageParams{
				Type:    protocol.Warning,
				Message: fmt.Sprintf("Failed to submit transaction: %s", err.Error()),
			})
		}

		return nil, err
	}

	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Info,
		Message: fmt.Sprintf("Executed script with result: %v", res),
	})

	return res, nil
}

// switchActiveAccount sets the account that is currently active and should be
// used when submitting transactions.
//
// There should be exactly 1 argument:
//   * the address of the new active account
func (s *Server) switchActiveAccount(conn protocol.Conn, args ...interface{}) (interface{}, error) {
	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Log,
		Message: fmt.Sprintf("set active acct %v", args),
	})

	expectedArgCount := 1
	if len(args) != expectedArgCount {
		return nil, fmt.Errorf("expecting %d arguments, got %d", expectedArgCount, len(args))
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

	expectedArgCount := 0
	if len(args) != expectedArgCount {
		return nil, fmt.Errorf("expecting %d args got: %d", expectedArgCount, len(args))
	}

	addr, err := s.createAccountHelper(conn)
	if err != nil {
		return nil, err
	}

	return addr, nil
}

// createDefaultAccounts creates a set of default accounts and returns their addresses.
//
// This command will wait until the emulator server is started before submitting any transactions.
func (s *Server) createDefaultAccounts(conn protocol.Conn, args ...interface{}) (interface{}, error) {
	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Log,
		Message: fmt.Sprintf("create default acct %v", args),
	})

	expectedArgCount := 1
	if len(args) != expectedArgCount {
		return nil, fmt.Errorf("must have %d args, got: %d", expectedArgCount, len(args))
	}

	n, ok := args[0].(float64)
	if !ok {
		return nil, errors.New("invalid count argument")
	}

	count := int(n)

	conn.ShowMessage(&protocol.ShowMessageParams{
		Type:    protocol.Info,
		Message: fmt.Sprintf("Creating %d default accounts", count),
	})

	// Ping the emulator server for 30 seconds until it is available
	timer := time.NewTimer(30 * time.Second)
RetryLoop:
	for {
		select {
		case <-timer.C:
			return nil, errors.New("emulator server timed out")
		default:
			err := s.flowClient.Ping(context.Background())
			if err == nil {
				break RetryLoop
			}
		}
	}

	accounts := make([]flow.Address, count)

	for i := 0; i < count; i++ {
		addr, err := s.createAccountHelper(conn)
		if err != nil {
			return nil, err
		}
		accounts[i] = addr
	}

	return accounts, nil
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

	expectedArgCount := 1
	if len(args) != expectedArgCount {
		return nil, fmt.Errorf("must have %d args, got: %d", expectedArgCount, len(args))
	}
	uri, ok := args[0].(string)
	if !ok {
		return nil, errors.New("invalid uri argument")
	}
	doc, ok := s.documents[protocol.DocumentUri(uri)]
	if !ok {
		return nil, fmt.Errorf("could not find document for URI %s", uri)
	}

	file := parseFileFromURI(uri)

	conn.ShowMessage(&protocol.ShowMessageParams{
		Type:    protocol.Info,
		Message: fmt.Sprintf("Deploying %s to account 0x%s", file, s.activeAccount.Hex()),
	})

	accountCode := []byte(doc.text)
	script := templates.UpdateAccountCode(accountCode)

	_, err := s.sendTransactionHelper(conn, script, true)
	return nil, err
}

// sendTransactionHelper sends a transaction with the given script, from the
// currently active account. Returns the hash of the transaction if it is
// successfully submitted.
//
// If an error occurs, attempts to show an appropriate message (either via logs
// or UI popups in the client).
func (s *Server) sendTransactionHelper(conn protocol.Conn, script []byte, authorize bool) (flow.Identifier, error) {
	accountKey, signer, err := s.getAccountKey(s.activeAccount)
	if err != nil {
		return flow.EmptyID, err
	}

	tx := flow.NewTransaction().
		SetScript(script).
		SetProposalKey(s.activeAccount, accountKey.ID, accountKey.SequenceNumber).
		SetPayer(s.activeAccount)

	if authorize {
		tx.AddAuthorizer(s.activeAccount)
	}

	err = tx.SignEnvelope(s.activeAccount, accountKey.ID, signer)
	if err != nil {
		return flow.EmptyID, err
	}

	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Info,
		Message: fmt.Sprintf("submitting transaction %d", tx.ID()),
	})

	err = s.flowClient.SendTransaction(context.Background(), *tx)
	if err != nil {
		grpcErr, ok := status.FromError(err)
		if ok {
			if grpcErr.Code() == codes.Unavailable {
				// The emulator server isn't running
				conn.ShowMessage(&protocol.ShowMessageParams{
					Type:    protocol.Warning,
					Message: "The emulator server is unavailable. Please start the emulator (`cadence.runEmulator`) first.",
				})
				return flow.EmptyID, err
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
				return flow.EmptyID, err
			}
		} else {
			conn.LogMessage(&protocol.LogMessageParams{
				Type:    protocol.Warning,
				Message: fmt.Sprintf("Failed to submit transaction: %s", err.Error()),
			})
		}

		return flow.EmptyID, err
	}

	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Info,
		Message: fmt.Sprintf("Submitted transaction id=%s", tx.ID().Hex()),
	})

	return tx.ID(), nil
}

// createAccountHelper creates a new account and returns its address.
func (s *Server) createAccountHelper(conn protocol.Conn) (addr flow.Address, err error) {
	accountKey := &flow.AccountKey{
		PublicKey: s.config.ServiceAccountKey.PrivateKey.PublicKey(),
		SigAlgo:   s.config.ServiceAccountKey.SigAlgo,
		HashAlgo:  s.config.ServiceAccountKey.HashAlgo,
		Weight:    flow.AccountKeyWeightThreshold,
	}

	script, err := templates.CreateAccount([]*flow.AccountKey{accountKey}, nil)
	if err != nil {
		return addr, fmt.Errorf("failed to generate account creation script: %w", err)
	}

	txID, err := s.sendTransactionHelper(conn, script, true)
	if err != nil {
		return addr, err
	}

	// TODO: replace this for loop with a synchronous GetTransaction in SDK
	// that handles waiting for it to be mined
	var txResult *flow.TransactionResult
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	for {
		txResult, err = s.flowClient.GetTransactionResult(ctx, txID)
		if err != nil {
			return addr, err
		}
		if txResult.Status == flow.TransactionStatusFinalized ||
			txResult.Status == flow.TransactionStatusSealed {

			break
		}
	}

	for _, event := range txResult.Events {
		if event.Type == flow.EventAccountCreated {
			accountCreatedEvent := flow.AccountCreatedEvent(event)
			addr = accountCreatedEvent.Address()
			break
		}
	}

	if addr == flow.EmptyAddress {
		return addr, fmt.Errorf("failed to get new account address for tx %s", txID.Hex())
	}

	s.accounts[addr] = s.config.ServiceAccountKey

	return addr, nil
}

func parseFileFromURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}

	return filepath.Base(u.Path)
}
