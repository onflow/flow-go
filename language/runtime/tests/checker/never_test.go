package checker

import (
	"testing"

	require "github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckNever(t *testing.T) {

	_, err := ParseAndCheckWithOptions(t,
		`
            pub fun test(): Int {
                return panic("XXX")
            }
        `,
		ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(
					stdlib.StandardLibraryFunctions{
						stdlib.PanicFunction,
					}.ToValueDeclarations(),
				),
			},
		},
	)

	require.NoError(t, err)
}
