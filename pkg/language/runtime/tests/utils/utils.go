package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/parser"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
)

func ParseAndCheck(t *testing.T, code string) (*sema.Checker, error) {
	return ParseAndCheckWithOptions(t, code, ParseAndCheckOptions{})
}

type ParseAndCheckOptions struct {
	Values         map[string]sema.ValueDeclaration
	Types          map[string]sema.TypeDeclaration
	ImportLocation ast.ImportLocation
	ImportResolver ast.ImportResolver
}

func ParseAndCheckWithOptions(
	t *testing.T,
	code string,
	options ParseAndCheckOptions,
) (*sema.Checker, error) {
	program, _, err := parser.ParseProgram(code)

	require.Nil(t, err)

	if options.ImportResolver != nil {
		err := program.ResolveImports(options.ImportResolver)
		if err != nil {
			return nil, err
		}
	}

	checker, err := sema.NewChecker(program, options.Values, options.Types)
	if err != nil {
		return checker, err
	}
	checker.ImportLocation = options.ImportLocation

	err = checker.Check()
	return checker, err
}

func ExpectCheckerErrors(t *testing.T, err error, len int) []error {
	if len <= 0 && err == nil {
		return nil
	}

	require.Error(t, err)

	assert.IsType(t, &sema.CheckerError{}, err)

	errs := err.(*sema.CheckerError).Errors

	require.Len(t, errs, len)

	return errs
}
