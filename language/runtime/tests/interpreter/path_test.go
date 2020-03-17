package interpreter_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/interpreter"
)

func TestInterpretPath(t *testing.T) {

	for _, domain := range common.AllPathDomainsByIdentifier {

		t.Run(fmt.Sprintf("valid: %s", domain.Name()), func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      let x = /%s/random
                    `,
					domain.Identifier(),
				),
			)

			assert.Equal(t,
				interpreter.PathValue{
					Domain:     domain.Identifier(),
					Identifier: "random",
				},
				inter.Globals["x"].Value,
			)
		})
	}
}
