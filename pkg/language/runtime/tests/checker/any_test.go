package checker

import (
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckAny(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let a: Any = 1
      let b: Any = true
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}
