package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckBoolean(t *testing.T) {

	checker, err := ParseAndCheck(t, `
        let x = true
	`)

	assert.Nil(t, err)

	assert.Equal(t, checker.GlobalValues["x"].Type, &sema.BoolType{})
}
