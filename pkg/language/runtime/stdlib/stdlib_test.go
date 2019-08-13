package stdlib

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/interpreter"
)

func TestAssert(t *testing.T) {
	RegisterTestingT(t)

	inter := interpreter.NewInterpreter(&ast.Program{})
	for _, function := range BuiltIns {
		Expect(inter.ImportFunction(function.Name, function.Function)).
			To(Not(HaveOccurred()))
	}

	_, err := inter.Invoke("assert", false, "oops")
	Expect(err).
		To(Equal(AssertionError{
			Message:  "oops",
			Position: ast.Position{},
		}))

	_, err = inter.Invoke("assert", true, "oops")
	Expect(err).
		To(Not(HaveOccurred()))

func TestPanic(t *testing.T) {
	RegisterTestingT(t)

	inter := interpreter.NewInterpreter(&ast.Program{})
	for _, function := range BuiltIns {
		Expect(inter.ImportFunction(function.Name, function.Function)).
			To(Not(HaveOccurred()))
	}

	_, err := inter.Invoke("panic", "oops")
	Expect(err).
		To(Equal(PanicError{
			Message:  "oops",
			Position: ast.Position{},
		}))
}
