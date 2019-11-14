package runtime

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
	"github.com/dapperlabs/flow-go/language/runtime/interpreter"
	"github.com/dapperlabs/flow-go/language/runtime/parser"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	"github.com/dapperlabs/flow-go/language/runtime/trampoline"
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
	)
	if err != nil {
		return nil, err
	}

	values := standardLibraryFunctions.ToValues()

	inter, err := interpreter.NewInterpreter(
		checker,
		interpreter.WithPredefinedValues(values),
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

func (r *REPL) Suggestions() (result []struct{ Name, Description string }) {
	names := map[string]string{}

	for name, variable := range r.checker.GlobalValues {
		if names[name] != "" {
			continue
		}
		names[name] = variable.Type.String()
	}

	for name, description := range names {
		result = append(result, struct{ Name, Description string }{
			Name:        name,
			Description: description,
		})
	}

	return
}
