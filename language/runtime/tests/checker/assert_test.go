package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckAssertWithoutMessage(t *testing.T) {

	_, err := ParseAndCheckWithOptions(t,
		`
            fun test() {
                assert(1 == 2)
            }
        `,
		ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(
					stdlib.StandardLibraryFunctions{
						stdlib.AssertFunction,
					}.ToValueDeclarations(),
				),
			},
		},
	)

	assert.Nil(t, err)
}

func TestCheckAssertWithMessage(t *testing.T) {

	_, err := ParseAndCheckWithOptions(t,
		`
            fun test() {
                assert(1 == 2, message: "test message")
            }
        `,
		ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(
					stdlib.StandardLibraryFunctions{
						stdlib.AssertFunction,
					}.ToValueDeclarations(),
				),
			},
		},
	)

	assert.Nil(t, err)
}
