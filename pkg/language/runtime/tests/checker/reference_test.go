package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
)

func TestCheckReferenceTypeOuter(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test(r: &[R]) {}
    `)

	assert.Nil(t, err)
}

func TestCheckReferenceTypeInner(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test(r: [&R]) {}
    `)

	assert.Nil(t, err)
}

func TestCheckNestedReferenceType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test(r: &[&R]) {}
    `)

	assert.Nil(t, err)
}
