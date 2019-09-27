package parser

import (
	"fmt"
	"io/ioutil"
	goRuntime "runtime"

	"github.com/antlr/antlr4/runtime/Go/antlr"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
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

func ParseProgram(code string) (*ast.Program, error) {
	result, errors := parse(
		code,
		func(parser *StrictusParser) antlr.ParserRuleContext {
			return parser.Program()
		},
	)

	var err error
	if len(errors) > 0 {
		err = Error{errors}
	}

	program, ok := result.(*ast.Program)
	if !ok {
		return nil, err
	}

	return program, err
}

func ParseExpression(code string) (ast.Expression, error) {
	result, errors := parse(
		code,
		func(parser *StrictusParser) antlr.ParserRuleContext {
			return parser.Expression()
		},
	)

	var err error
	if len(errors) > 0 {
		err = Error{errors}
	}

	program, ok := result.(ast.Expression)
	if !ok {
		return nil, err
	}

	return program, err
}

func parse(
	code string,
	parse func(*StrictusParser) antlr.ParserRuleContext,
) (
	result ast.Repr,
	errors []error,
) {
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
			var err error
			// don't recover Go errors
			err, ok = r.(goRuntime.Error)
			if ok {
				panic(err)
			}
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}
			appendSyntaxErrors()
			errors = append(errors, err)
			result = nil
		}
	}()

	parsed := parse(parser)

	appendSyntaxErrors()

	if len(errors) > 0 {
		return nil, errors
	}

	return parsed.Accept(&ProgramVisitor{}), errors
}

func ParseProgramFromFile(filename string) (program *ast.Program, code string, err error) {
	var data []byte
	data, err = ioutil.ReadFile(filename)
	if err != nil {
		return nil, "", err
	}

	code = string(data)

	program, err = ParseProgram(code)
	if err != nil {
		return nil, code, err
	}
	return program, code, nil
}
