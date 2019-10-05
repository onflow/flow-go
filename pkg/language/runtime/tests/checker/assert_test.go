package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckAssertWithoutMessage(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t,
		`
            fun test() {
                assert(1 == 2)
            }
        `,
		stdlib.StandardLibraryFunctions{
			stdlib.AssertFunction,
		}.ToValueDeclarations(),
		nil,
		nil,
	)

	assert.Nil(t, err)
}

func TestCheckAssertWithMessage(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t,
		`
            fun test() {
                assert(1 == 2, message: "test message")
            }
        `,
		stdlib.StandardLibraryFunctions{
			stdlib.AssertFunction,
		}.ToValueDeclarations(),
		nil,
		nil,
	)

	assert.Nil(t, err)
}
