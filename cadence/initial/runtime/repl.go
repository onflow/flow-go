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

package runtime

import (
	"sort"

	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/errors"
	"example.com/cadence-initial/runtime/interpreter"
	"example.com/cadence-initial/runtime/parser"
	"example.com/cadence-initial/runtime/sema"
	"example.com/cadence-initial/runtime/stdlib"
	"example.com/cadence-initial/runtime/trampoline"
)

type REPL struct {
	checker  *sema.Checker
	inter    *interpreter.Interpreter
	onError  func(error)
	onResult func(interpreter.Value)
}

func NewREPL(onError func(error), onResult func(interpreter.Value)) (*REPL, error) {

	standardLibraryFunctions := append(stdlib.BuiltinFunctions, stdlib.HelperFunctions...)
	valueDeclarations := standardLibraryFunctions.ToValueDeclarations()
	typeDeclarations := stdlib.BuiltinTypes.ToTypeDeclarations()

	checker, err := sema.NewChecker(
		nil,
		REPLLocation{},
		sema.WithPredeclaredValues(valueDeclarations),
		sema.WithPredeclaredTypes(typeDeclarations),
		sema.WithAccessCheckMode(sema.AccessCheckModeNotSpecifiedUnrestricted),
	)
	if err != nil {
		return nil, err
	}

	values := standardLibraryFunctions.ToValues()

	var uuid uint64

	inter, err := interpreter.NewInterpreter(
		checker,
		interpreter.WithPredefinedValues(values),
		interpreter.WithUUIDHandler(func() uint64 {
			defer func() { uuid++ }()
			return uuid
		}),
	)
	if err != nil {
		return nil, err
	}

	repl := &REPL{
		checker:  checker,
		inter:    inter,
		onError:  onError,
		onResult: onResult,
	}
	return repl, nil
}

func (r *REPL) handleCheckerError(code string) bool {
	err := r.checker.CheckerError()
	if err == nil {
		return true
	}
	if r.onError != nil {
		r.onError(err)
	}
	return false
}

func (r *REPL) execute(element ast.Element) {
	result := trampoline.Run(element.Accept(r.inter).(trampoline.Trampoline))
	expStatementRes, ok := result.(interpreter.ExpressionStatementResult)
	if !ok {
		return
	}
	if r.onResult == nil {
		return
	}
	r.onResult(expStatementRes.Value)
}

func (r *REPL) check(element ast.Element, code string) bool {
	element.Accept(r.checker)
	return r.handleCheckerError(code)
}

func (r *REPL) Accept(code string) (inputIsComplete bool) {
	var result []interface{}
	var err error
	result, inputIsComplete, err = parser.ParseReplInput(code)

	if !inputIsComplete {
		return
	}

	if err != nil {
		r.onError(err)
		return
	}

	r.checker.ResetErrors()

	for _, element := range result {

		switch typedElement := element.(type) {
		case ast.Declaration:
			program := &ast.Program{
				Declarations: []ast.Declaration{typedElement},
			}

			if !r.check(program, code) {
				return
			}

			r.execute(typedElement)

		case ast.Statement:
			r.checker.Program = nil

			if !r.check(typedElement, code) {
				return
			}

			r.execute(typedElement)

		default:
			panic(errors.NewUnreachableError())
		}
	}

	return
}

type REPLSuggestion struct {
	Name, Description string
}

func (r *REPL) Suggestions() (result []REPLSuggestion) {
	names := map[string]string{}

	for name, variable := range r.checker.GlobalValues {
		if names[name] != "" {
			continue
		}
		names[name] = variable.Type.String()
	}

	for name, description := range names {
		result = append(result, REPLSuggestion{
			Name:        name,
			Description: description,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		a := result[i]
		b := result[j]
		return a.Name < b.Name
	})

	return
}
