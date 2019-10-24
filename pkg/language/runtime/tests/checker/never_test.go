package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
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
