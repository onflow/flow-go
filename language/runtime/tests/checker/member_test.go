package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckOptionalChainingFieldRead(t *testing.T) {

	testType := &sema.CompositeType{
		Kind:       common.CompositeKindStructure,
		Identifier: "Test",
		Members:    map[string]*sema.Member{},
	}

	fieldType := &sema.IntType{}

	testType.Members["x"] = &sema.Member{
		Type:            fieldType,
		DeclarationKind: common.DeclarationKindField,
		VariableKind:    ast.VariableKindConstant,
	}

	values := stdlib.StandardLibraryValues{
		{
			Name: "test",
			Type: &sema.OptionalType{
				Type: testType,
			},
			Kind:       common.DeclarationKindConstant,
			IsConstant: true,
		},
	}.ToValueDeclarations()

	checker, err := ParseAndCheckWithOptions(t,
		`
          let x = test?.x
        `,
		ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(values),
			},
		},
	)

	require.Nil(t, err)

	assert.True(t, checker.GlobalValues["x"].Type.Equal(
		&sema.OptionalType{Type: fieldType},
	))
}

func TestCheckOptionalChainingFunctionRead(t *testing.T) {

	testType := &sema.CompositeType{
		Kind:       common.CompositeKindStructure,
		Identifier: "Test",
		Members:    map[string]*sema.Member{},
	}

	functionType := &sema.FunctionType{
		ReturnTypeAnnotation: &sema.TypeAnnotation{
			Type: &sema.IntType{},
		},
	}
	testType.Members["x"] = &sema.Member{
		Type:            functionType,
		DeclarationKind: common.DeclarationKindFunction,
		VariableKind:    ast.VariableKindConstant,
	}

	values := stdlib.StandardLibraryValues{
		{
			Name: "test",
			Type: &sema.OptionalType{
				Type: testType,
			},
			Kind:       common.DeclarationKindConstant,
			IsConstant: true,
		},
	}.ToValueDeclarations()

	checker, err := ParseAndCheckWithOptions(t,
		`
          let x = test?.x
        `,
		ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(values),
			},
		},
	)

	require.Nil(t, err)

	assert.True(t, checker.GlobalValues["x"].Type.Equal(
		&sema.OptionalType{Type: functionType},
	))
}

func TestCheckOptionalChainingFunctionCall(t *testing.T) {

	testType := &sema.CompositeType{
		Kind:       common.CompositeKindStructure,
		Identifier: "Test",
		Members:    map[string]*sema.Member{},
	}

	returnType := &sema.IntType{}

	functionType := &sema.FunctionType{
		ReturnTypeAnnotation: &sema.TypeAnnotation{
			Type: returnType,
		},
	}
	testType.Members["x"] = &sema.Member{
		Type:            functionType,
		DeclarationKind: common.DeclarationKindFunction,
		VariableKind:    ast.VariableKindConstant,
	}

	values := stdlib.StandardLibraryValues{
		{
			Name: "test",
			Type: &sema.OptionalType{
				Type: testType,
			},
			Kind:       common.DeclarationKindConstant,
			IsConstant: true,
		},
	}.ToValueDeclarations()

	checker, err := ParseAndCheckWithOptions(t,
		`
          let x = test?.x()
        `,
		ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(values),
			},
		},
	)

	require.Nil(t, err)

	assert.True(t, checker.GlobalValues["x"].Type.Equal(
		&sema.OptionalType{Type: returnType},
	))
}

func TestCheckInvalidOptionalChainingNonOptional(t *testing.T) {

	testType := &sema.CompositeType{
		Kind:       common.CompositeKindStructure,
		Identifier: "Test",
		Members:    map[string]*sema.Member{},
	}

	functionType := &sema.FunctionType{
		ReturnTypeAnnotation: &sema.TypeAnnotation{
			Type: &sema.IntType{},
		},
	}
	testType.Members["x"] = &sema.Member{
		Type:            functionType,
		DeclarationKind: common.DeclarationKindFunction,
		VariableKind:    ast.VariableKindConstant,
	}

	values := stdlib.StandardLibraryValues{
		{
			Name:       "test",
			Type:       testType,
			Kind:       common.DeclarationKindConstant,
			IsConstant: true,
		},
	}.ToValueDeclarations()

	_, err := ParseAndCheckWithOptions(t,
		`
          let x = test?.x
        `,
		ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(values),
			},
		},
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidOptionalChainingError{}, errs[0])
}

func TestCheckInvalidOptionalChainingFieldAssignment(t *testing.T) {

	testType := &sema.CompositeType{
		Kind:       common.CompositeKindStructure,
		Identifier: "Test",
		Members:    map[string]*sema.Member{},
	}

	fieldType := &sema.IntType{}

	testType.Members["x"] = &sema.Member{
		Type:            fieldType,
		DeclarationKind: common.DeclarationKindField,
		VariableKind:    ast.VariableKindVariable,
	}

	values := stdlib.StandardLibraryValues{
		{
			Name: "test",
			Type: &sema.OptionalType{
				Type: testType,
			},
			Kind:       common.DeclarationKindConstant,
			IsConstant: true,
		},
	}.ToValueDeclarations()

	_, err := ParseAndCheckWithOptions(t,
		`
          fun foo() {
              test?.x = 1
          }
        `,
		ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(values),
			},
		},
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedOptionalChainingAssignmentError{}, errs[0])
}
