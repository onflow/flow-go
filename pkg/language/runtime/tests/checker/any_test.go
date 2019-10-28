package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
)

func TestCheckAny(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let a: Any = 1
      let b: Any = true
    `)

	assert.Nil(t, err)
}
