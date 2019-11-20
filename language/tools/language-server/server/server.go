package server

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
	"github.com/dapperlabs/flow-go/language/runtime/parser"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	"github.com/dapperlabs/flow-go/language/tools/language-server/protocol"
)

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

var valueDeclarations = append(stdlib.BuiltinFunctions, stdlib.HelperFunctions...).ToValueDeclarations()
var typeDeclarations = stdlib.BuiltinTypes.ToTypeDeclarations()

type Server struct {
	checkers map[protocol.DocumentUri]*sema.Checker
}

func NewServer() Server {
	return Server{
		map[protocol.DocumentUri]*sema.Checker{},
	}
}

func (s Server) Start() {
	<-protocol.NewServer(s).Start()
}

func (s Server) Initialize(
	connection protocol.Connection,
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
				ResolveProvider: true,
			},
		},
	}

	// after initialization, indicate to the client which commands we support
	go s.registerCommands(connection)

	return result, nil
}

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

func (s Server) parse(connection protocol.Connection, code, location string) (*ast.Program, error) {
	start := time.Now()
	program, _, err := parser.ParseProgram(code)
	elapsed := time.Since(start)

	connection.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Info,
		Message: fmt.Sprintf("parsing %s took %s", location, elapsed),
	})

	return program, err
}

func (s Server) DidChangeTextDocument(
	connection protocol.Connection,
	params *protocol.DidChangeTextDocumentParams,
) error {
	connection.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Log,
		Message: "DidChangeText",
	})
	uri := params.TextDocument.URI
	code := params.ContentChanges[0].Text

	program, err := s.parse(connection, code, string(uri))

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
			return resolveImport(connection, mainPath, location)
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

		connection.LogMessage(&protocol.LogMessageParams{
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

	connection.PublishDiagnostics(&protocol.PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: diagnostics,
	})

	return nil
}

func (s Server) Hover(
	connection protocol.Connection,
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

func (s Server) Definition(
	connection protocol.Connection,
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

func (s Server) SignatureHelp(
	connection protocol.Connection,
	params *protocol.TextDocumentPositionParams,
) (*protocol.SignatureHelp, error) {
	return nil, nil
}

func (s Server) CodeLens(connection protocol.Connection, params *protocol.CodeLensParams) ([]*protocol.CodeLens, error) {
	connection.LogMessage(&protocol.LogMessageParams{
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
	for declaration, _ := range checker.Elaboration.FunctionDeclarationFunctionTypes {
		if declaration.Identifier.String() == "main" {
			actions = append(actions, &protocol.CodeLens{
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      float64(declaration.StartPosition().Line - 1),
						Character: 0,
					},
					End: protocol.Position{
						Line:      float64(declaration.StartPosition().Line - 1),
						Character: 0,
					},
				},
				Command: &protocol.Command{
					Title:   "submit transaction",
					Command: "cadence.submitTransaction",
					//Arguments: []interface{}{"a", "b", "c"},
				},
				//Data: "codelens data",
			})
		}
	}

	return actions, nil
}

// TODO is this necessary?
func (s Server) CodeLensResolve(connection protocol.Connection, params *protocol.CodeLens) (*protocol.CodeLens, error) {
	return params, nil
}

func (s Server) ExecuteCommand(connection protocol.Connection, params *protocol.ExecuteCommandParams) (interface{}, error) {
	connection.LogMessage(&protocol.LogMessageParams{
		Type:    protocol.Log,
		Message: "called execute command: " + params.Command,
	})

	switch params.Command {
	case "cadence.submitTransaction":
		connection.ShowMessage(&protocol.ShowMessageParams{
			Type:    protocol.Info,
			Message: "called submit transaction",
		})
	}
	return nil, nil
}

// Registers the commands that the server is able to handle.
// The best reference I've found for how this works is:
// https://stackoverflow.com/questions/43328582/how-to-implement-quickfix-via-a-language-server
func (s Server) registerCommands(connection protocol.Connection) {
	err := connection.RegisterCapability(&protocol.RegistrationParams{
		Registrations: []protocol.Registration{
			{
				ID:     "test",
				Method: "workspace/executeCommand",
				RegisterOptions: protocol.ExecuteCommandRegistrationOptions{
					ExecuteCommandOptions: protocol.ExecuteCommandOptions{
						Commands: []string{"cadence.submitTransaction"},
					},
				},
			},
		},
	})
	if err != nil {
		connection.LogMessage(&protocol.LogMessageParams{
			Type:    protocol.Warning,
			Message: fmt.Sprintf("Failed to register command: %w", err),
		})
	}
}

func (Server) Shutdown(connection protocol.Connection) error {
	connection.ShowMessage(&protocol.ShowMessageParams{
		Type:    protocol.Warning,
		Message: "Cadence language server is shutting down",
	})
	return nil
}

func (Server) Exit(connection protocol.Connection) error {
	os.Exit(0)
	return nil
}

func resolveImport(
	connection protocol.Connection,
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
