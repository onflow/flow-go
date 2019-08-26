package stdlib

import (
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/interpreter"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
)

// ToValueDeclarations

func ToValueDeclarations(functions []StandardLibraryFunction) map[string]sema.ValueDeclaration {
	valueDeclarations := make(map[string]sema.ValueDeclaration, len(functions))
	for _, function := range functions {
		valueDeclarations[function.Name] = function
	}
	return valueDeclarations
}

// ToValues

func ToValues(functions []StandardLibraryFunction) map[string]interpreter.Value {
	values := make(map[string]interpreter.Value, len(functions))
	for _, function := range functions {
		values[function.Name] = function.Function
	}
	return values
}
