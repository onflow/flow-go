package runtime

import (
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/parser"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckConstantAndVariableDeclarations(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
        let x = 1
        var y = 1
    `)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(checker.Globals["x"].Type).
		To(Equal(&sema.IntType{}))

	Expect(checker.Globals["y"].Type).
		To(Equal(&sema.IntType{}))
}

func TestCheckBoolean(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
        let x = true
    `)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(checker.Globals["x"].Type).
		To(Equal(&sema.BoolType{}))
}
