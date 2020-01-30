package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func newReferencesTestValueDeclarations(referencesIsAssignable bool) map[string]sema.ValueDeclaration {
	result := map[string]sema.ValueDeclaration{
		"references": stdlib.StandardLibraryValue{
			Name: "references",
			Type: &sema.ReferencesType{
				Assignable: referencesIsAssignable,
			},
			Kind:       common.DeclarationKindConstant,
			IsConstant: true,
		},
	}
	result["storage"] = storageValueDeclaration
	return result
}

func ParseAndCheckReferences(t *testing.T, referencesIsAssignable bool, code string) (*sema.Checker, error) {
	return ParseAndCheckWithOptions(t,
		code,
		ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(newReferencesTestValueDeclarations(referencesIsAssignable)),
			},
		},
	)
}

func TestCheckReferencesIndexing(t *testing.T) {

	checker, err := ParseAndCheckReferences(t, false,
		`
          resource R {}
          let a = references[&R]
        `,
	)
	require.NoError(t, err)

	assert.Equal(t,
		&sema.OptionalType{
			Type: &sema.ReferenceType{
				Type: checker.GlobalTypes["R"].Type,
			},
		},
		checker.GlobalValues["a"].Type,
	)
}

func TestCheckInvalidReferencesIndexingAssignment(t *testing.T) {

	const referencesIsAssignable = false

	_, err := ParseAndCheckReferences(t,
		referencesIsAssignable,
		`
          resource R {}

          fun test() {
              references[&R] = &storage[R] as R
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ReadOnlyTargetAssignmentError{}, errs[0])
}

func TestCheckReferencesIndexingAssignment(t *testing.T) {

	const referencesIsAssignable = true

	_, err := ParseAndCheckReferences(t,
		referencesIsAssignable,
		`
          resource R {}

          fun test() {
              references[&R] = &storage[R] as R
          }
        `,
	)

	require.NoError(t, err)
}

func TestCheckInvalidReferencesIndexingAssignmentNonReference(t *testing.T) {

	const referencesIsAssignable = true

	_, err := ParseAndCheckReferences(t,
		referencesIsAssignable,
		`
          fun test() {
              references[Int] = 0
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchWithDescriptionError{}, errs[0])
}
