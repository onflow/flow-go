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

package parser

import (
	"fmt"
	"io/ioutil"
	goRuntime "runtime"

	"github.com/antlr/antlr4/runtime/Go/antlr"

	"example.com/cadence-initial/runtime/ast"
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

	if l.isIncompleteInputException(e, offendingSymbol) {
		l.inputIsIncomplete = true
	}

	var offset int
	if token, ok := offendingSymbol.(*antlr.CommonToken); ok {
		offset = token.GetStart()
	} else if e != nil {
		offset = e.GetInputStream().Index()
	}

	position := ast.Position{
		Offset: offset,
		Line:   line,
		Column: column,
	}

	l.syntaxErrors = append(l.syntaxErrors,
		&SyntaxError{
			Pos:     position,
			Message: message,
		},
	)
}

func (l *errorListener) isIncompleteInputException(e antlr.RecognitionException, offendingSymbol interface{}) bool {
	switch e.(type) {
	case *antlr.InputMisMatchException, *antlr.NoViableAltException:
		break
	default:
		return false
	}

	if offendingToken, ok := offendingSymbol.(antlr.Token); ok {
		if offendingToken.GetTokenType() != antlr.TokenEOF {
			return false
		}
	}

	return true
}

func ParseProgram(code string) (program *ast.Program, inputIsComplete bool, err error) {
	result, inputIsComplete, errors := parse(
		code,
		func(parser *CadenceParser) antlr.ParserRuleContext {
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
		func(parser *CadenceParser) antlr.ParserRuleContext {
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

func ParseReplInput(code string) (replInput []interface{}, inputIsComplete bool, err error) {
	result, inputIsComplete, errors := parse(
		code,
		func(parser *CadenceParser) antlr.ParserRuleContext {
			return parser.ReplInput()
		},
	)

	if len(errors) > 0 {
		err = Error{errors}
	}

	elements, ok := result.([]interface{})
	if !ok {
		return nil, inputIsComplete, err
	}

	return elements, inputIsComplete, err
}

func parse(
	code string,
	parse func(*CadenceParser) antlr.ParserRuleContext,
) (
	result ast.Repr,
	inputIsComplete bool,
	errors []error,
) {
	input := antlr.NewInputStream(code)

	listener := new(errorListener)

	lexer := NewCadenceLexer(input)
	// remove the lexer's default console error listener
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(listener)

	stream := antlr.NewCommonTokenStream(lexer, 0)

	parser := NewCadenceParser(stream)
	// remove the default console error listener
	parser.RemoveErrorListeners()
	parser.AddErrorListener(listener)

	// for debugging only (to get diagnostics):
	// parser.AddErrorListener(antlr.NewDiagnosticErrorListener(true))

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
