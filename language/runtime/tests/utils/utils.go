package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
	"github.com/dapperlabs/flow-go/language/runtime/parser"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
)

// TestLocation used as the default location for scripts executed in tests.
const TestLocation = ast.StringLocation("test")

func ParseAndCheck(t *testing.T, code string) (*sema.Checker, error) {
	return ParseAndCheckWithOptions(t, code, ParseAndCheckOptions{})
}

type ParseAndCheckOptions struct {
	ImportResolver ast.ImportResolver
	Location       ast.Location
	Options        []sema.Option
}

func ParseAndCheckWithOptions(
	t *testing.T,
	code string,
	options ParseAndCheckOptions,
) (*sema.Checker, error) {
	program, _, err := parser.ParseProgram(code)

	if !assert.Nil(t, err) {
		assert.FailNow(t, errors.UnrollChildErrors(err))
		return nil, err
	}

	if options.ImportResolver != nil {
		err := program.ResolveImports(options.ImportResolver)
		if err != nil {
			return nil, err
		}
	}

	if options.Location == nil {
		options.Location = TestLocation
	}
	checker, err := sema.NewChecker(
		program,
		options.Location,
		options.Options...,
	)
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
