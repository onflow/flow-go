package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckAnyStruct(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let a: AnyStruct = 1
      let b: AnyStruct = true
    `)

	assert.NoError(t, err)
}

func TestCheckInvalidAnyStructResourceType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      let a: AnyStruct = <-create R()
      let b: AnyStruct = [<-create R()]
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
}
