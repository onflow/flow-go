package runtime

import "github.com/dapperlabs/flow-go/language/runtime/interpreter"

type Value = interpreter.Value

type Event struct {
	Type   Type
	Fields []Value
}

type Address = interpreter.AddressValue
