package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckCharacterLiteral(t *testing.T) {

	checker, err := ParseAndCheck(t, `
        let a: Character = "a"
    `)

	assert.Nil(t, err)

	assert.Equal(t,
		&sema.CharacterType{},
		checker.GlobalValues["a"].Type,
	)
}

func TestCheckInvalidCharacterLiteral(t *testing.T) {

	_, err := ParseAndCheck(t, `
        let a: Character = "abc"
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidCharacterLiteralError{}, errs[0])
}
