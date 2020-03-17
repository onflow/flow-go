package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckPath(t *testing.T) {

	for _, domain := range common.AllPathDomainsByIdentifier {

		t.Run(fmt.Sprintf("valid: %s", domain.Name()), func(t *testing.T) {

			checker, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      let x = /%s/random
                    `,
					domain.Identifier(),
				),
			)

			require.NoError(t, err)

			assert.IsType(t,
				&sema.PathType{},
				checker.GlobalValues["x"].Type,
			)
		})
	}

	t.Run("invalid: unsupported domain", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          let x = /wrong/random
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidPathDomainError{}, errs[0])
	})
}
