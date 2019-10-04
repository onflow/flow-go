package checker

import (
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckAny(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let a: Any = 1
      let b: Any = true
	`)

	assert.Nil(t, err)
}
