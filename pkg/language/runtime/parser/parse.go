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
	syntaxErrors      []*SyntaxError
	inputIsIncomplete bool
}

func (l *errorListener) SyntaxError(
	recognizer antlr.Recognizer,
	offendingSymbol interface{},
	line, column int,
	message string,
	e antlr.RecognitionException,
) {
	offendingToken := offendingSymbol.(antlr.Token)

	if l.isIncompleteInputException(e, offendingToken) {
		l.inputIsIncomplete = true
	}

	position := ast.PositionFromToken(offendingToken)

	l.syntaxErrors = append(l.syntaxErrors,
		&SyntaxError{
			Pos:     position,
			Message: message,
		},
	)
}

func (l *errorListener) isIncompleteInputException(e antlr.RecognitionException, offendingToken antlr.Token) bool {
	switch e.(type) {
	case *antlr.InputMisMatchException, *antlr.NoViableAltException:
		break
	default:
		return false
	}

	if offendingToken.GetTokenType() != antlr.TokenEOF {
		return false
	}

	return true
}

func ParseProgram(code string) (program *ast.Program, inputIsComplete bool, err error) {
	result, inputIsComplete, errors := parse(
		code,
		func(parser *StrictusParser) antlr.ParserRuleContext {
			return parser.Program()
		},
	)

	if len(errors) > 0 {
		err = Error{errors}
	}

	program, ok := result.(*ast.Program)
	if !ok {
		return nil, inputIsComplete, err
	}

	return program, inputIsComplete, err
}

func ParseExpression(code string) (expression ast.Expression, inputIsComplete bool, err error) {
	result, inputIsComplete, errors := parse(
		code,
		func(parser *StrictusParser) antlr.ParserRuleContext {
			return parser.Expression()
		},
	)

	if len(errors) > 0 {
		err = Error{errors}
	}

	program, ok := result.(ast.Expression)
	if !ok {
		return nil, inputIsComplete, err
	}

	return program, inputIsComplete, err
}

func ParseReplInput(code string) (replInput interface{}, inputIsComplete bool, err error) {
	result, inputIsComplete, errors := parse(
		code,
		func(parser *StrictusParser) antlr.ParserRuleContext {
			return parser.ReplInput()
		},
	)

	if len(errors) > 0 {
		err = Error{errors}
	}

	return result, inputIsComplete, err
}

func parse(
	code string,
	parse func(*StrictusParser) antlr.ParserRuleContext,
) (
	result ast.Repr,
	inputIsComplete bool,
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

	appendParseErrors := func() {
		inputIsComplete = !listener.inputIsIncomplete

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
			appendParseErrors()
			errors = append(errors, err)
			result = nil
		}
	}()

	parsed := parse(parser)

	appendParseErrors()

	if len(errors) > 0 {
		return nil, inputIsComplete, errors
	}

	visitor := &ProgramVisitor{}
	result = parsed.Accept(visitor)
	errors = append(errors, visitor.parseErrors...)
	return result, inputIsComplete, errors
}

func ParseProgramFromFile(
	filename string,
) (
	program *ast.Program,
	inputIsComplete bool,
	code string,
	err error,
) {
	var data []byte
	data, err = ioutil.ReadFile(filename)
	if err != nil {
		return nil, true, "", err
	}

	code = string(data)

	program, inputIsComplete, err = ParseProgram(code)
	if err != nil {
		return nil, inputIsComplete, code, err
	}
	return program, inputIsComplete, code, nil
}
