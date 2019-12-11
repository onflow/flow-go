package checker

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

var accountValueDeclaration = stdlib.StandardLibraryValue{
	Name:       "account",
	Type:       &sema.AccountType{},
	Kind:       common.DeclarationKindConstant,
	IsConstant: true,
}

func ParseAndCheckAccount(t *testing.T, code string) (*sema.Checker, error) {
	return ParseAndCheckWithOptions(t,
		code,
		ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(map[string]sema.ValueDeclaration{
					"account": accountValueDeclaration,
				}),
				sema.WithAccessCheckMode(sema.AccessCheckModeNotSpecifiedUnrestricted),
			},
		},
	)
}

func TestCheckAccount(t *testing.T) {

	t.Run("storage is assignable", func(t *testing.T) {
		_, err := ParseAndCheckAccount(t,
			`
              resource R {}

              fun test(): @R? {
                  let r <- account.storage[R] <- create R()
                  return <-r
              }
            `,
		)

		require.NoError(t, err)
	})

	t.Run("published is assignable", func(t *testing.T) {
		_, err := ParseAndCheckAccount(t,
			`
              resource R {}

              fun test() {
                  account.published[&R] = &account.storage[R] as R
              }
            `,
		)

		require.NoError(t, err)
	})
}
