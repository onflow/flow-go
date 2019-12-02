package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

var publicAccountValueDeclaration = stdlib.StandardLibraryValue{
	Name:       "publicAccount",
	Type:       &sema.PublicAccountType{},
	Kind:       common.DeclarationKindConstant,
	IsConstant: true,
}

func ParseAndCheckPublicAccount(t *testing.T, code string) (*sema.Checker, error) {
	return ParseAndCheckWithOptions(t,
		code,
		ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(map[string]sema.ValueDeclaration{
					"publicAccount": publicAccountValueDeclaration,
				}),
				sema.WithAccessCheckMode(sema.AccessCheckModeNotSpecifiedUnrestricted),
			},
		},
	)
}

func TestCheckPublicAccount(t *testing.T) {

	t.Run("storage is not accessible", func(t *testing.T) {
		_, err := ParseAndCheckPublicAccount(t,
			`
              resource R {}

              fun test(): Bool {
                  return publicAccount.storage[R] != nil
              }
            `,
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
	})

	t.Run("published is not assignable", func(t *testing.T) {
		_, err := ParseAndCheckPublicAccount(t,
			`
              resource R {}

              fun test() {
                  publicAccount.published[&R] = &publicAccount.storage[R] as R
              }
            `,
		)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
		assert.IsType(t, &sema.ReadOnlyTargetAssignmentError{}, errs[1])
	})
}
