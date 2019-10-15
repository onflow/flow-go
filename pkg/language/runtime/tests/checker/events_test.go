package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
)

func TestCheckEventDeclaration(t *testing.T) {
	t.Run("ValidEvent", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
			event Transfer(to: Int, from: Int)
		`)

		assert.Nil(t, err)
	})

	t.Run("InvalidEventNonPrimitiveType", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            struct Token {
              let ID: String

              init(ID: String) {
                self.ID = ID
              }
            }

			event Transfer(token: Token)
		`)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidEventParameterTypeError{}, errs[0])
	})

	t.Run("RedeclaredEvent", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
			event Transfer(to: Int, from: Int)
			event Transfer(to: Int)
		`)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.RedeclarationError{}, errs[0])
	})
}
