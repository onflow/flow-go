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
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
	"google.golang.org/grpc"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/parser"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/cadence/languageserver/config"
	"github.com/onflow/cadence/languageserver/protocol"
)

var valueDeclarations = append(
	stdlib.FlowBuiltInFunctions(stdlib.FlowBuiltinImpls{}),
	stdlib.BuiltinFunctions...,
).ToValueDeclarations()
var typeDeclarations = append(
	stdlib.FlowBuiltInTypes,
	stdlib.BuiltinTypes...,
).ToTypeDeclarations()

// document represents an open document on the client. It contains all cached
// information about each document that is used to support CodeLens,
// transaction submission, and script execution.
type document struct {
	text          string
	latestVersion float64
	hasErrors     bool
}

type Server struct {
	config    config.Config
	checkers  map[protocol.DocumentUri]*sema.Checker
	documents map[protocol.DocumentUri]document
	// registry of custom commands we support
	commands   map[string]CommandHandler
	flowClient *client.Client
	// set of created accounts we can submit transactions for
	accounts      map[flow.Address]config.AccountPrivateKey
	activeAccount flow.Address
}

func NewServer() *Server {
	return &Server{
		checkers:  make(map[protocol.DocumentUri]*sema.Checker),
		documents: make(map[protocol.DocumentUri]document),
		commands:  make(map[string]CommandHandler),
		accounts:  make(map[flow.Address]config.AccountPrivateKey),
	}
}

func (s *Server) Start() {
	<-protocol.NewServer(s).Start()
}

func (s *Server) Initialize(
	conn protocol.Conn,
	params *protocol.InitializeParams,
) (
	*protocol.InitializeResult,
	error,
) {
	result := &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync:   protocol.Full,
			HoverProvider:      true,
			DefinitionProvider: true,
			// TODO:
			//SignatureHelpProvider: &protocol.SignatureHelpOptions{
			//	TriggerCharacters: []string{"("},
			//},
			CodeLensProvider: &protocol.CodeLensOptions{
				ResolveProvider: false,
			},
		},
	}

	// load the config options sent from the client
	conf, err := config.FromInitializationOptions(params.InitializationOptions)
	if err != nil {
		return nil, err
	}
	s.config = conf

	serviceAddress := flow.ServiceAddress(flow.Emulator)

	// add the service account as a usable account
	s.accounts[serviceAddress] = conf.ServiceAccountKey
	s.activeAccount = serviceAddress

	s.flowClient, err = client.New(
		s.config.EmulatorAddr,
		grpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	// TODO remove
	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Info,
		Message: fmt.Sprintf("Successfully loaded config emu_addr: %s", conf.EmulatorAddr),
	})

	// after initialization, indicate to the client which commands we support
	go s.registerCommands(conn)

	return result, nil
}

// DidOpenTextDocument is called whenever a new file is opened.
// We parse and check the text and publish diagnostics about the document.
func (s *Server) DidOpenTextDocument(conn protocol.Conn, params *protocol.DidOpenTextDocumentParams) error {
	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Log,
		Message: "DidOpenTextDoc",
	})

	uri := params.TextDocument.URI
	text := params.TextDocument.Text

	diagnostics, err := s.getDiagnostics(conn, uri, text)
	if err != nil {
		return err
	}
	conn.PublishDiagnostics(&protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})

	s.documents[uri] = document{
		text:          text,
		latestVersion: params.TextDocument.Version,
		hasErrors:     len(diagnostics) > 0,
	}

	return nil
}

// DidChangeTextDocument is called whenever the current document changes.
// We parse and check the text and publish diagnostics about the document.
func (s *Server) DidChangeTextDocument(
	conn protocol.Conn,
	params *protocol.DidChangeTextDocumentParams,
) error {
	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Log,
		Message: fmt.Sprintf("DidChangeText changes: %d", len(params.ContentChanges)),
	})
	uri := params.TextDocument.URI
	text := params.ContentChanges[0].Text

	diagnostics, err := s.getDiagnostics(conn, uri, text)
	if err != nil {
		return err
	}
	conn.PublishDiagnostics(&protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})

	s.documents[uri] = document{
		text:          text,
		latestVersion: params.TextDocument.Version,
		hasErrors:     len(diagnostics) > 0,
	}

	return nil
}

// Hover returns contextual type information about the variable at the given
// location.
func (s *Server) Hover(
	_ protocol.Conn,
	params *protocol.TextDocumentPositionParams,
) (*protocol.Hover, error) {

	uri := params.TextDocument.URI
	checker, ok := s.checkers[uri]
	if !ok {
		return nil, nil
	}

	occurrence := checker.Occurrences.Find(protocolToSemaPosition(params.Position))

	if occurrence == nil || occurrence.Origin == nil {
		return nil, nil
	}

	contents := protocol.MarkupContent{
		Kind:  protocol.Markdown,
		Value: fmt.Sprintf("* Type: `%s`", occurrence.Origin.Type.String()),
	}
	return &protocol.Hover{Contents: contents}, nil
}

// Definition finds the definition of the type at the given location.
func (s *Server) Definition(
	_ protocol.Conn,
	params *protocol.TextDocumentPositionParams,
) (*protocol.Location, error) {

	uri := params.TextDocument.URI
	checker, ok := s.checkers[uri]
	if !ok {
		return nil, nil
	}

	occurrence := checker.Occurrences.Find(protocolToSemaPosition(params.Position))

	if occurrence == nil {
		return nil, nil
	}

	origin := occurrence.Origin
	if origin == nil || origin.StartPos == nil || origin.EndPos == nil {
		return nil, nil
	}

	return &protocol.Location{
		URI:   uri,
		Range: astToProtocolRange(*origin.StartPos, *origin.EndPos),
	}, nil
}

// TODO
func (s *Server) SignatureHelp(
	_ protocol.Conn,
	_ *protocol.TextDocumentPositionParams,
) (*protocol.SignatureHelp, error) {
	return nil, nil
}

// CodeLens is called every time the document contents change and returns a
// list of actions to be injected into the source as inline buttons.
func (s *Server) CodeLens(conn protocol.Conn, params *protocol.CodeLensParams) ([]*protocol.CodeLens, error) {
	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Info,
		Message: "code lens called uri:" + string(params.TextDocument.URI) + " acct:" + s.activeAccount.String(),
	})

	uri := params.TextDocument.URI
	checker, ok := s.checkers[uri]
	if !ok {
		// Can we ensure this doesn't happen?
		return []*protocol.CodeLens{}, nil
	}

	elaboration := checker.Elaboration
	var (
		scriptFuncDeclarations        = getScriptDeclarations(elaboration.FunctionDeclarationFunctionTypes)
		txDeclarations                = getTransactionDeclarations(elaboration.TransactionDeclarationTypes)
		contractDeclarations          = getContractDeclarations(elaboration.CompositeDeclarationTypes)
		contractInterfaceDeclarations = getContractInterfaceDeclarations(elaboration.InterfaceDeclarationTypes)

		actions []*protocol.CodeLens
	)

	// Show submit button when there is exactly one transaction declaration and no
	// other actionable declarations.
	if len(txDeclarations) == 1 &&
		len(contractDeclarations) == 0 &&
		len(contractInterfaceDeclarations) == 0 &&
		len(scriptFuncDeclarations) == 0 {
		actions = append(actions, &protocol.CodeLens{
			Range: astToProtocolRange(txDeclarations[0].StartPosition(), txDeclarations[0].StartPosition()),
			Command: &protocol.Command{
				Title:     fmt.Sprintf("submit transaction with account 0x%s", s.activeAccount.Hex()),
				Command:   CommandSubmitTransaction,
				Arguments: []interface{}{uri},
			},
		})
	}

	// Show deploy button when there is exactly one contract declaration,
	// any number of contract interface declarations, and no other actionable
	// declarations.
	if len(contractDeclarations) == 1 &&
		len(txDeclarations) == 0 &&
		len(scriptFuncDeclarations) == 0 {
		actions = append(actions, &protocol.CodeLens{
			Range: astToProtocolRange(contractDeclarations[0].StartPosition(), contractDeclarations[0].StartPosition()),
			Command: &protocol.Command{
				Title:     fmt.Sprintf("deploy contract to account 0x%s", s.activeAccount.Hex()),
				Command:   CommandUpdateAccountCode,
				Arguments: []interface{}{uri},
			},
		})
	}

	// Show deploy interface button when there are 1 or more contract interface
	// declarations, but no other actionable declarations.
	if len(contractInterfaceDeclarations) > 0 &&
		len(txDeclarations) == 0 &&
		len(scriptFuncDeclarations) == 0 &&
		len(contractDeclarations) == 0 {
		// decide whether to pluralize
		pluralInterface := "interface"
		if len(contractInterfaceDeclarations) > 1 {
			pluralInterface = "interfaces"
		}

		actions = append(actions, &protocol.CodeLens{
			Range: firstLineRange(),
			Command: &protocol.Command{
				Title:     fmt.Sprintf("deploy contract %s to account 0x%s", pluralInterface, s.activeAccount.Hex()),
				Command:   CommandUpdateAccountCode,
				Arguments: []interface{}{uri},
			},
		})
	}

	// Show execute script button when there is exactly one valid script
	// function and no other actionable declarations.
	if len(scriptFuncDeclarations) == 1 &&
		len(contractDeclarations) == 0 &&
		len(contractInterfaceDeclarations) == 0 &&
		len(txDeclarations) == 0 {
		actions = append(actions, &protocol.CodeLens{
			Range: astToProtocolRange(scriptFuncDeclarations[0].StartPosition(), scriptFuncDeclarations[0].StartPosition()),
			Command: &protocol.Command{
				Title:     "execute script",
				Command:   CommandExecuteScript,
				Arguments: []interface{}{uri},
			},
		})
	}

	return actions, nil
}

// ExecuteCommand is called to execute a custom, server-defined command.
//
// We register all the commands we support in registerCommands and populate
// their corresponding handler at server initialization.
func (s *Server) ExecuteCommand(conn protocol.Conn, params *protocol.ExecuteCommandParams) (interface{}, error) {
	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Log,
		Message: "called execute command: " + params.Command,
	})

	f, ok := s.commands[params.Command]
	if !ok {
		return nil, fmt.Errorf("invalid command: %s", params.Command)
	}
	return f(conn, params.Arguments...)
}

// Shutdown tells the server to stop accepting any new requests. This can only
// be followed by a call to Exit, which exits the process.
func (*Server) Shutdown(conn protocol.Conn) error {
	conn.ShowMessage(&protocol.ShowMessageParams{
		Type:    protocol.Warning,
		Message: "Cadence language server is shutting down",
	})
	return nil
}

// Exit exits the process.
func (*Server) Exit(_ protocol.Conn) error {
	os.Exit(0)
	return nil
}

// getAccountKey returns the first account key and signer for the given address.
func (s *Server) getAccountKey(address flow.Address) (*flow.AccountKey, crypto.Signer, error) {
	privateKey, ok := s.accounts[address]
	if !ok {
		return nil, nil, fmt.Errorf(
			"cannot sign transaction: unknown account %s",
			address,
		)
	}

	account, err := s.flowClient.GetAccount(context.Background(), address)
	if err != nil {
		return nil, nil, err
	}

	if len(account.Keys) == 0 {
		return nil, nil, fmt.Errorf(
			"cannot sign transaction: account %s has no keys",
			address.Hex(),
		)
	}

	accountKey := account.Keys[0]
	signer := crypto.NewNaiveSigner(privateKey.PrivateKey, privateKey.HashAlgo)

	return accountKey, signer, nil
}

// getDiagnostics parses and checks the given file and generates diagnostics
// indicating each syntax or semantic error. Returns a list of diagnostics
// that the caller is responsible for publishing to the client.
//
// Returns an error if an unexpected error occurred.
func (s *Server) getDiagnostics(conn protocol.Conn, uri protocol.DocumentUri, text string) ([]protocol.Diagnostic, error) {
	diagnostics := make([]protocol.Diagnostic, 0)
	program, err := parse(conn, text, string(uri))

	// If there were parsing errors, convert each one to a diagnostic and exit
	// without checking.
	if err != nil {
		if parentErr, ok := err.(errors.ParentError); ok {
			parserDiagnostics := getDiagnosticsForParentError(conn, parentErr)
			diagnostics = append(diagnostics, parserDiagnostics...)
			return diagnostics, nil
		}
		return nil, err
	}

	// There were no parser errors. Proceed to resolving imports and
	// checking the parsed program.
	mainPath := strings.TrimPrefix(string(uri), "file://")

	_ = program.ResolveImports(func(location ast.Location) (program *ast.Program, err error) {
		return s.resolveImport(conn, mainPath, location)
	})

	checker, err := sema.NewChecker(
		program,
		runtime.FileLocation(string(uri)),
		sema.WithPredeclaredValues(valueDeclarations),
		sema.WithPredeclaredTypes(typeDeclarations),
	)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	err = checker.Check()
	elapsed := time.Since(start)

	// Log how long it took to check the file
	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Info,
		Message: fmt.Sprintf("checking %s took %s", string(uri), elapsed),
	})

	s.checkers[uri] = checker

	if err != nil {
		if parentErr, ok := err.(errors.ParentError); ok {
			checkerDiagnostics := getDiagnosticsForParentError(conn, parentErr)
			diagnostics = append(diagnostics, checkerDiagnostics...)
		} else {
			return nil, err
		}
	}

	extraDiagnostics := getExtraDiagnostics(conn, checker)
	diagnostics = append(diagnostics, extraDiagnostics...)

	return diagnostics, nil
}

// getExtraDiagnostics gets extra non-error diagnostics based on a checker.
//
// For example, this function will return diagnostics for declarations that are
// syntactically and semantically valid, but unsupported by the extension.
func getExtraDiagnostics(_ protocol.Conn, checker *sema.Checker) (diagnostics []protocol.Diagnostic) {
	elaboration := checker.Elaboration

	// Warn if there are more than 1 transaction declarations as deployment will fail
	if len(elaboration.TransactionDeclarationTypes) > 1 {
		isFirst := true
		for decl := range elaboration.TransactionDeclarationTypes {
			// Skip the first declaration
			if isFirst {
				isFirst = false
				continue
			}

			diagnostics = append(diagnostics, protocol.Diagnostic{
				Range:    astToProtocolRange(decl.StartPosition(), decl.StartPosition().Shifted(len("transaction"))),
				Severity: protocol.SeverityWarning,
				Message:  "Cannot declare more than one transaction per file",
			})
		}
	}

	// Warn if there are more than 1 contract declarations as deployment will fail
	contractDeclarations := getContractDeclarations(checker.Elaboration.CompositeDeclarationTypes)
	if len(contractDeclarations) > 1 {
		isFirst := true
		for _, decl := range contractDeclarations {
			// Skip the first declaration
			if isFirst {
				isFirst = false
				continue
			}

			diagnostics = append(diagnostics, protocol.Diagnostic{
				Range:    astToProtocolRange(decl.Identifier.StartPosition(), decl.Identifier.EndPosition()),
				Severity: protocol.SeverityWarning,
				Message:  "Cannot declare more than one contract per file",
			})
		}
	}

	return
}

// getDiagnosticsForParentError unpacks all child errors and converts each to
// a diagnostic. Both parser and checker errors can be unpacked.
//
// Logs any conversion failures to the client.
func getDiagnosticsForParentError(conn protocol.Conn, err errors.ParentError) (diagnostics []protocol.Diagnostic) {
	for _, childErr := range err.ChildErrors() {
		convertibleErr, ok := childErr.(convertibleError)
		if !ok {
			conn.LogMessage(&protocol.LogMessageParams{
				Type:    protocol.Warning,
				Message: fmt.Sprintf("Unable to convert non-convertable error: %s", err.Error()),
			})
			continue
		}
		diagnostic := convertError(convertibleErr)
		diagnostics = append(diagnostics, diagnostic)
	}

	return
}

// parse parses the given code and returns the resultant program.
func parse(conn protocol.Conn, code, location string) (*ast.Program, error) {
	start := time.Now()
	program, _, err := parser.ParseProgram(code)
	elapsed := time.Since(start)

	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Info,
		Message: fmt.Sprintf("parsing %s took %s", location, elapsed),
	})

	return program, err
}

func (s *Server) resolveImport(
	_ protocol.Conn,
	mainPath string,
	location ast.Location,
) (*ast.Program, error) {
	switch loc := location.(type) {
	case ast.StringLocation:
		return s.resolveFileImport(mainPath, loc)
	case ast.AddressLocation:
		return s.resolveAccountImport(loc)
	default:
		return nil, fmt.Errorf("unresolvable import location %s", loc.ID())
	}
}

func (s *Server) resolveFileImport(mainPath string, location ast.StringLocation) (*ast.Program, error) {
	filename := path.Join(path.Dir(mainPath), string(location))

	if filename == mainPath {
		return nil, fmt.Errorf("cannot import current file: %s", filename)
	}

	program, _, _, err := parser.ParseProgramFromFile(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot find imported file: %s", filename)
	}

	return program, nil
}

func (s *Server) resolveAccountImport(location ast.AddressLocation) (*ast.Program, error) {
	accountAddr := location.ToAddress()

	acct, err := s.flowClient.GetAccount(context.Background(), flow.BytesToAddress(accountAddr[:]))
	if err != nil {
		return nil, fmt.Errorf("RAI cannot get account with address 0x%s. err: %w", accountAddr, err)
	}

	program, _, err := parser.ParseProgram(string(acct.Code))
	if err != nil {
		return nil, fmt.Errorf("cannot parse code at adddress 0x%s. err: %w", accountAddr, err)
	}

	return program, nil
}

// convertibleError is an error that can be converted to LSP diagnostic.
type convertibleError interface {
	error
	ast.HasPosition
}

// convertError converts a checker error to a diagnostic.
func convertError(err convertibleError) protocol.Diagnostic {
	startPosition := err.StartPosition()
	endPosition := err.EndPosition()

	var message strings.Builder
	message.WriteString(err.Error())

	if secondaryError, ok := err.(errors.SecondaryError); ok {
		message.WriteString(". ")
		message.WriteString(secondaryError.SecondaryError())
	}

	return protocol.Diagnostic{
		Message: message.String(),
		Code:    protocol.SeverityError,
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      float64(startPosition.Line - 1),
				Character: float64(startPosition.Column),
			},
			End: protocol.Position{
				Line:      float64(endPosition.Line - 1),
				Character: float64(endPosition.Column + 1),
			},
		},
	}
}

// getScriptDeclarations finds function declarations that are interpreted as scripts.
func getScriptDeclarations(funcDeclarationMap map[*ast.FunctionDeclaration]*sema.FunctionType) (scriptDeclarations []*ast.FunctionDeclaration) {
	for decl := range funcDeclarationMap {
		if decl.Identifier.String() == "main" && len(decl.ParameterList.Parameters) == 0 {
			scriptDeclarations = append(scriptDeclarations, decl)
		}
	}
	return
}

// getTranscactionDeclarations finds all transaction declarations.
func getTransactionDeclarations(txDeclarationMap map[*ast.TransactionDeclaration]*sema.TransactionType) (txDeclarations []*ast.TransactionDeclaration) {
	for decl := range txDeclarationMap {
		txDeclarations = append(txDeclarations, decl)
	}
	return
}

// getContractInterfaceDeclarations finds all interface declarations for contracts.
func getContractInterfaceDeclarations(interfaceDeclarationMap map[*ast.InterfaceDeclaration]*sema.InterfaceType) (contractInterfaceDeclarations []*ast.InterfaceDeclaration) {
	for decl := range interfaceDeclarationMap {
		if decl.CompositeKind == common.CompositeKindContract {
			contractInterfaceDeclarations = append(contractInterfaceDeclarations, decl)
		}
	}
	return
}

// getContractDeclarations returns a list of contract declarations based on
// the keys of the input map.
// Usage: `getContractDeclarations(checker.Elaboration.CompositeDeclarations)`
func getContractDeclarations(compositeDeclarations map[*ast.CompositeDeclaration]*sema.CompositeType) []*ast.CompositeDeclaration {
	contractDeclarations := make([]*ast.CompositeDeclaration, 0)
	for decl := range compositeDeclarations {
		if decl.CompositeKind == common.CompositeKindContract {
			contractDeclarations = append(contractDeclarations, decl)
		}
	}
	return contractDeclarations
}

func protocolToSemaPosition(pos protocol.Position) sema.Position {
	return sema.Position{
		Line:   int(pos.Line + 1),
		Column: int(pos.Character),
	}
}

func astToProtocolPosition(pos ast.Position) protocol.Position {
	return protocol.Position{
		Line:      float64(pos.Line - 1),
		Character: float64(pos.Column),
	}
}

func astToProtocolRange(startPos, endPos ast.Position) protocol.Range {
	return protocol.Range{
		Start: astToProtocolPosition(startPos),
		End:   astToProtocolPosition(endPos.Shifted(1)),
	}
}

// firstLine returns a range mapping to the first character of the first
// line of the document.
func firstLineRange() protocol.Range {
	return protocol.Range{
		Start: protocol.Position{
			Line:      0,
			Character: 0,
		},
		End: protocol.Position{
			Line:      0,
			Character: 0,
		},
	}
}
