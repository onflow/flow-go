package stdlib

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/interpreter"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
)

func TestAssert(t *testing.T) {
	RegisterTestingT(t)

	program := &ast.Program{}

	checker, err := sema.NewChecker(program, ToValueDeclarations(BuiltinFunctions), nil)
	Expect(err).
		To(Not(HaveOccurred()))

	inter, err := interpreter.NewInterpreter(checker, ToValues(BuiltinFunctions))

	Expect(err).
		To(Not(HaveOccurred()))

	_, err = inter.Invoke("assert", false, "oops")
	Expect(err).
		To(Equal(AssertionError{
			Message:  "oops",
			Location: interpreter.Location{},
		}))

	_, err = inter.Invoke("assert", false)
	Expect(err).
		To(Equal(AssertionError{
			Message:  "",
			Location: interpreter.Location{},
		}))

	_, err = inter.Invoke("assert", true, "oops")
	Expect(err).
		To(Not(HaveOccurred()))

	_, err = inter.Invoke("assert", true)
	Expect(err).
		To(Not(HaveOccurred()))
}

func TestPanic(t *testing.T) {
	RegisterTestingT(t)

	checker, err := sema.NewChecker(&ast.Program{}, ToValueDeclarations(BuiltinFunctions), nil)
	Expect(err).
		To(Not(HaveOccurred()))

	inter, err := interpreter.NewInterpreter(checker, ToValues(BuiltinFunctions))

	Expect(err).
		To(Not(HaveOccurred()))

	_, err = inter.Invoke("panic", "oops")
	Expect(err).
		To(Equal(PanicError{
			Message:  "oops",
			Location: interpreter.Location{},
		}))
}
