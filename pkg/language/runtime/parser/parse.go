package parser

import (
	"fmt"
	"github.com/antlr/antlr4/runtime/Go/antlr"
	"github.com/dapperlabs/bamboo-node/language/runtime/ast"
)

type errorListener struct {
	*antlr.DefaultErrorListener
	syntaxErrors []*SyntaxError
}

func (l *errorListener) SyntaxError(
	recognizer antlr.Recognizer,
	offendingSymbol interface{},
	line, column int,
	message string,
	e antlr.RecognitionException,
) {
	position := ast.PositionFromToken(offendingSymbol.(antlr.Token))

	l.syntaxErrors = append(l.syntaxErrors, &SyntaxError{
		Pos:     position,
		Message: message,
	})
}

func Parse(code string) (program *ast.Program, errors []error) {
	input := antlr.NewInputStream(code)
	lexer := NewStrictusLexer(input)
	stream := antlr.NewCommonTokenStream(lexer, 0)
	parser := NewStrictusParser(stream)
	// diagnostics, for debugging only:
	// parser.AddErrorListener(antlr.NewDiagnosticErrorListener(true))
	listener := new(errorListener)
	// remove the default console error listener
	parser.RemoveErrorListeners()
	parser.AddErrorListener(listener)

	appendSyntaxErrors := func() {
		for _, syntaxError := range listener.syntaxErrors {
			errors = append(errors, syntaxError)
		}
	}

	// recover internal panics and return them as an error
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}
			appendSyntaxErrors()
			errors = append(errors, err)
			program = nil
		}
	}()

	result := parser.Program().Accept(&ProgramVisitor{})

	appendSyntaxErrors()

	if len(errors) > 0 {
		return nil, errors
	}

	program, ok := result.(*ast.Program)
	if !ok {
		return nil, errors
	}

	return program, errors
}
