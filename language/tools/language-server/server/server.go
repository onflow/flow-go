package server

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/dapperlabs/flow-go/sdk/client"

	"github.com/dapperlabs/flow-go/language/tools/language-server/config"

	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
	"github.com/dapperlabs/flow-go/language/runtime/parser"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	"github.com/dapperlabs/flow-go/language/tools/language-server/protocol"
)

var valueDeclarations = append(stdlib.BuiltinFunctions, stdlib.HelperFunctions...).ToValueDeclarations()
var typeDeclarations = stdlib.BuiltinTypes.ToTypeDeclarations()

type Server struct {
	config   config.Config
	checkers map[protocol.DocumentUri]*sema.Checker
	// map of document URI to the document text
	documents map[protocol.DocumentUri]string
	// registry of custom commands we support
	commands   map[string]CommandHandler
	flowClient *client.Client
	// the nonce to use when submitting transactions
	nonce uint64
}

func NewServer() Server {
	return Server{
		checkers:  make(map[protocol.DocumentUri]*sema.Checker),
		documents: make(map[protocol.DocumentUri]string),
		commands:  make(map[string]CommandHandler),
	}
}

func (s Server) Start() {
	<-protocol.NewServer(s).Start()
}

func (s Server) Initialize(
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

	s.flowClient, err = client.New(s.config.EmulatorAddr)
	if err != nil {
		return nil, err
	}

	// TODO remove
	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Info,
		Message: fmt.Sprintf("Successfully loaded config emu_addr: %s acct_addr: %s", conf.EmulatorAddr, conf.AccountAddr.String()),
	})

	// after initialization, indicate to the client which commands we support
	go s.registerCommands(conn)

	return result, nil
}

// DidChangeTextDocument is called whenever the current document changes.
// We parse and check the new text and indicate any syntax or semantic errors
// by publishing "diagnostics".
func (s Server) DidChangeTextDocument(
	conn protocol.Conn,
	params *protocol.DidChangeTextDocumentParams,
) error {
	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Log,
		Message: "DidChangeText",
	})
	uri := params.TextDocument.URI
	code := params.ContentChanges[0].Text
	s.documents[uri] = code

	program, err := parse(conn, code, string(uri))

	diagnostics := []protocol.Diagnostic{}

	if err != nil {

		if parserError, ok := err.(parser.Error); ok {
			for _, err := range parserError.Errors {
				parseError, ok := err.(parser.ParseError)
				if !ok {
					continue
				}

				diagnostic := convertError(parseError)
				if diagnostic == nil {
					continue
				}

				diagnostics = append(diagnostics, *diagnostic)
			}
		}
	} else {
		// no parsing errors

		// resolve imports

		mainPath := strings.TrimPrefix(string(uri), "file://")

		_ = program.ResolveImports(func(location ast.Location) (program *ast.Program, err error) {
			return resolveImport(mainPath, location)
		})

		// check program

		checker, err := sema.NewChecker(
			program,
			runtime.FileLocation(string(uri)),
			sema.WithPredeclaredValues(valueDeclarations),
			sema.WithPredeclaredTypes(typeDeclarations),
		)
		if err != nil {
			panic(err)
		}

		start := time.Now()
		err = checker.Check()
		elapsed := time.Since(start)

		conn.LogMessage(&protocol.LogMessageParams{
			Type:    protocol.Info,
			Message: fmt.Sprintf("checking took %s", elapsed),
		})

		s.checkers[uri] = checker

		if checkerError, ok := err.(*sema.CheckerError); ok && checkerError != nil {
			for _, err := range checkerError.Errors {
				if semanticError, ok := err.(sema.SemanticError); ok {
					diagnostic := convertError(semanticError)
					if diagnostic != nil {
						diagnostics = append(diagnostics, *diagnostic)
					}
				}
			}
		}
	}

	conn.PublishDiagnostics(&protocol.PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: diagnostics,
	})

	return nil
}

// Hover returns contextual type information about the variable at the given
// location.
func (s Server) Hover(
	conn protocol.Conn,
	params *protocol.TextDocumentPositionParams,
) (*protocol.Hover, error) {

	uri := params.TextDocument.URI
	checker, ok := s.checkers[uri]
	if !ok {
		return nil, nil
	}

	occurrence := checker.Occurrences.Find(protocolToSemaPosition(params.Position))

	if occurrence == nil {
		return nil, nil
	}

	contents := protocol.MarkupContent{
		Kind:  protocol.Markdown,
		Value: fmt.Sprintf("* Type: `%s`", occurrence.Origin.Type.String()),
	}
	return &protocol.Hover{Contents: contents}, nil
}

// Definition finds the definition of the type at the given location.
func (s Server) Definition(
	conn protocol.Conn,
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
func (s Server) SignatureHelp(
	conn protocol.Conn,
	params *protocol.TextDocumentPositionParams,
) (*protocol.SignatureHelp, error) {
	return nil, nil
}

// CodeLens is called every time the document contents change and returns a
// list of actions to be injected into the source as inline buttons.
func (s Server) CodeLens(conn protocol.Conn, params *protocol.CodeLensParams) ([]*protocol.CodeLens, error) {
	conn.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Info,
		Message: "code lens called" + string(params.TextDocument.URI),
	})

	checker, ok := s.checkers[params.TextDocument.URI]
	if !ok {
		// Can we ensure this doesn't happen?
		return []*protocol.CodeLens{}, nil
	}

	var actions []*protocol.CodeLens

	// Search for relevant function declarations
	for declaration := range checker.Elaboration.FunctionDeclarationFunctionTypes {
		if declaration.Identifier.String() == "main" {
			_ = params.TextDocument.URI
			actions = append(actions, &protocol.CodeLens{
				Range: astToProtocolRange(declaration.StartPosition(), declaration.StartPosition()),
				Command: &protocol.Command{
					Title:     "submit transaction",
					Command:   "cadence.submitTransaction",
					Arguments: []interface{}{params.TextDocument.URI},
				},
			})
		}
	}

	return actions, nil
}

// ExecuteCommand is called to execute a custom, server-defined command.
//
// We register all the commands we support in registerCommands and populate
// their corresponding handler at server initialization.
func (s Server) ExecuteCommand(conn protocol.Conn, params *protocol.ExecuteCommandParams) (interface{}, error) {
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
func (Server) Shutdown(conn protocol.Conn) error {
	conn.ShowMessage(&protocol.ShowMessageParams{
		Type:    protocol.Warning,
		Message: "Cadence language server is shutting down",
	})
	return nil
}

// Exit exits the process.
func (Server) Exit(_ protocol.Conn) error {
	os.Exit(0)
	return nil
}

// getNextNonce increments and returns the nonce. This ensures that subsequent
// transaction submissions aren't duplicates.
func (s *Server) getNextNonce() uint64 {
	s.nonce++
	return s.nonce
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

func resolveImport(
	mainPath string,
	location ast.Location,
) (*ast.Program, error) {
	stringLocation, ok := location.(ast.StringLocation)
	// TODO: publish diagnostic type is not supported?
	if !ok {
		return nil, nil
	}

	filename := path.Join(path.Dir(mainPath), string(stringLocation))

	// TODO: publish diagnostic import is self?
	if filename == mainPath {
		return nil, nil
	}

	program, _, _, err := parser.ParseProgramFromFile(filename)
	// TODO: publish diagnostic file does not exist?
	if err != nil {
		return nil, nil
	}

	return program, nil
}

// convertError converts a checker error to a diagnostic.
func convertError(err error) *protocol.Diagnostic {
	positionedError, ok := err.(ast.HasPosition)
	if !ok {
		return nil
	}

	startPosition := positionedError.StartPosition()
	endPosition := positionedError.EndPosition()

	var message strings.Builder
	message.WriteString(err.Error())

	if secondaryError, ok := err.(errors.SecondaryError); ok {
		message.WriteString(". ")
		message.WriteString(secondaryError.SecondaryError())
	}

	return &protocol.Diagnostic{
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

func protocolToSemaPosition(pos protocol.Position) sema.Position {
	return sema.Position{
		Line:   int(pos.Line + 1),
		Column: int(pos.Character),
	}
}

func semaToProtocolPosition(pos sema.Position) protocol.Position {
	return protocol.Position{
		Line:      float64(pos.Line - 1),
		Character: float64(pos.Column),
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
