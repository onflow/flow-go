package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/parser"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
)

// TestLocation used as a location for scripts executed in tests.
const TestLocation = ast.StringLocation("test")

func ParseAndCheck(t *testing.T, code string) (*sema.Checker, error) {
	return ParseAndCheckWithExtra(t, code, nil, nil, nil, nil)
}

func ParseAndCheckWithExtra(
	t *testing.T,
	code string,
	values map[string]sema.ValueDeclaration,
	types map[string]sema.TypeDeclaration,
	location ast.Location,
	resolver ast.ImportResolver,
) (*sema.Checker, error) {
	program, _, err := parser.ParseProgram(code)

	require.Nil(t, err)

	if resolver != nil {
		err := program.ResolveImports(resolver)
		if err != nil {
			return nil, err
		}
	}

	if location == nil {
		location = TestLocation
	}

	checker, err := sema.NewChecker(program, values, types, location)
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

	require.Error(t, err)

	assert.IsType(t, &sema.CheckerError{}, err)

	errs := err.(*sema.CheckerError).Errors

	require.Len(t, errs, len)

	return errs
}
