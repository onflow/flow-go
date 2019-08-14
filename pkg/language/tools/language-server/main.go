package main

import (
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/errors"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/parser"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
	"github.com/dapperlabs/bamboo-node/pkg/language/tools/language-server/protocol"
	"os"
	"strings"
)

type server struct{}

func (server) Initialize(
	connection protocol.Connection,
	params *protocol.InitializeParams,
) (
	*protocol.InitializeResult,
	error,
) {
	result := &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: protocol.Full,
		},
	}
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

func (server) DidChangeTextDocument(
	connection protocol.Connection,
	params *protocol.DidChangeTextDocumentParams,
) error {
	code := params.ContentChanges[0].Text

	program, parseErrors := parser.ParseProgram(code)
	errorCount := len(parseErrors)

	diagnostics := []protocol.Diagnostic{}

	if errorCount > 0 {

		for _, err := range parseErrors {
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

	} else {
		// no parsing parseErrors, check program

		checker := sema.NewChecker(program)
		err := checker.Check()

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

func (server) Shutdown(connection protocol.Connection) error {
	connection.ShowMessage(&protocol.ShowMessageParams{
		Type:    protocol.Warning,
		Message: "Bamboo language server is shutting down",
	})
	return nil
}

func (server) Exit(connection protocol.Connection) error {
	os.Exit(0)
	return nil
}

func main() {
	server := &server{}
	<-protocol.NewServer(server).Start()
}
