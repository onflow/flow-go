package strictus

import (
	"bamboo-runtime/execution/strictus/ast"
	"fmt"
	"github.com/antlr/antlr4/runtime/Go/antlr"
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
	l.syntaxErrors = append(l.syntaxErrors, &SyntaxError{
		Line:    line,
		Column:  column,
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

	result := parser.Program().Accept(&ProgramVisitor{}).(ast.Program)

	appendSyntaxErrors()

	if len(errors) > 0 {
		return nil, errors
	}

	return &result, errors
}
