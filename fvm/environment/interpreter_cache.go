package environment

import (
	"github.com/onflow/cadence/runtime/interpreter"
)

type InterpreterCache struct {
	interpreter *interpreter.Interpreter
}

func (c *InterpreterCache) SetInterpreter(inter *interpreter.Interpreter) {
	c.interpreter = inter
}

func (c *InterpreterCache) GetInterpreter() *interpreter.Interpreter {
	return c.interpreter
}

func (c *InterpreterCache) Reset() {
	c.interpreter = nil
}
