package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckBoolean(t *testing.T) {
	RegisterTestingT(t)

	checker, err := ParseAndCheck(`
        let x = true
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["x"].Type).
		To(Equal(&sema.BoolType{}))
}
