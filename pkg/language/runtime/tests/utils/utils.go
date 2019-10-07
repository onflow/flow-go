package utils

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/parser"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	"github.com/stretchr/testify/assert"
	"testing"
)

func ParseAndCheck(t *testing.T, code string) (*sema.Checker, error) {
	return ParseAndCheckWithExtra(t, code, nil, nil, nil)
}

func ParseAndCheckWithExtra(
	t *testing.T,
	code string,
	values map[string]sema.ValueDeclaration,
	types map[string]sema.TypeDeclaration,
	resolver ast.ImportResolver,
) (*sema.Checker, error) {
	program, _, err := parser.ParseProgram(code)

	assert.Nil(t, err)

	if resolver != nil {
		err := program.ResolveImports(resolver)
		if err != nil {
			return nil, err
		}
	}

	checker, err := sema.NewChecker(program, values, types)
	if err != nil {
		return checker, err
	}

	err = checker.Check()
	return checker, err
}

func ExpectCheckerErrors(t *testing.T, err error, len int) []error {
	if len <= 0 && err == nil {
		return nil
	}

	assert.Error(t, err)

	assert.IsType(t, &sema.CheckerError{}, err)

	errs := err.(*sema.CheckerError).Errors

	assert.Len(t, errs, len)

	return errs
}
