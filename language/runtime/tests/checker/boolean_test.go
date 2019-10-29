package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckBoolean(t *testing.T) {

	checker, err := ParseAndCheck(t, `
        let x = true
    `)

	assert.Nil(t, err)

	assert.Equal(t,
		&sema.BoolType{},
		checker.GlobalValues["x"].Type,
	)
}
