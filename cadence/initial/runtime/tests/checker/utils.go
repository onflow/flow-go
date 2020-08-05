package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/parser"
	"github.com/onflow/cadence/runtime/parser2"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/tests/utils"
)

func ParseAndCheck(t *testing.T, code string) (*sema.Checker, error) {
	return ParseAndCheckWithOptions(t, code, ParseAndCheckOptions{})
}

type ParseAndCheckOptions struct {
	ImportResolver ast.ImportResolver
	Location       ast.Location
	Options        []sema.Option
	SkipNewParser  bool
}

func ParseAndCheckWithOptions(
	t *testing.T,
	code string,
	options ParseAndCheckOptions,
) (*sema.Checker, error) {

	program, _, err := parser.ParseProgram(code)
	if !assert.NoError(t, err) {
		assert.FailNow(t, errors.UnrollChildErrors(err))
		return nil, err
	}

	if !options.SkipNewParser {
		program2, err := parser2.ParseProgram(code)
		if !assert.NoError(t, err) {
			assert.FailNow(t, errors.UnrollChildErrors(err))
			return nil, err
		}

		utils.AssertEqualWithDiff(t, program, program2)
	}

	if options.ImportResolver != nil {
		err := program.ResolveImports(options.ImportResolver)
		if err != nil {
			return nil, err
		}
	}

	if options.Location == nil {
		options.Location = utils.TestLocation
	}

	checkerOptions := append(
		[]sema.Option{
			sema.WithAccessCheckMode(sema.AccessCheckModeNotSpecifiedUnrestricted),
		},
		options.Options...,
	)

	checker, err := sema.NewChecker(
		program,
		options.Location,
		checkerOptions...,
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

	// Get the error message, to check that it can be successfully generated

	for _, checkerErr := range errs {
		_ = checkerErr.Error()
	}

	return errs
}
