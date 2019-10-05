package stdlib

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/interpreter"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
)

func TestAssert(t *testing.T) {

	program := &ast.Program{}

	checker, err := sema.NewChecker(program, BuiltinFunctions.ToValueDeclarations(), nil)
	assert.Nil(t, err)

	inter, err := interpreter.NewInterpreter(checker, BuiltinFunctions.ToValues())

	assert.Nil(t, err)

	_, err = inter.Invoke("assert", false, "oops")
	assert.Equal(t, err, AssertionError{
		Message:  "oops",
		Location: interpreter.Location{},
	})

	_, err = inter.Invoke("assert", false)
	assert.Equal(t, err, AssertionError{
		Message:  "",
		Location: interpreter.Location{},
	})

	_, err = inter.Invoke("assert", true, "oops")
	assert.Nil(t, err)

	_, err = inter.Invoke("assert", true)
	assert.Nil(t, err)
}

func TestPanic(t *testing.T) {

	checker, err := sema.NewChecker(&ast.Program{}, BuiltinFunctions.ToValueDeclarations(), nil)
	assert.Nil(t, err)

	inter, err := interpreter.NewInterpreter(checker, BuiltinFunctions.ToValues())

	assert.Nil(t, err)

	_, err = inter.Invoke("panic", "oops")
	assert.Equal(t, err, PanicError{
		Message:  "oops",
		Location: interpreter.Location{},
	})
}
