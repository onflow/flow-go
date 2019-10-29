package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckNever(t *testing.T) {

	_, err := ParseAndCheckWithOptions(t,
		`
            fun test(): Int {
                return panic("XXX")
            }
        `,
		ParseAndCheckOptions{
			Values: stdlib.StandardLibraryFunctions{
				stdlib.PanicFunction,
			}.ToValueDeclarations(),
		},
	)

	assert.Nil(t, err)
}
