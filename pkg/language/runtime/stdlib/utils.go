package stdlib

import (
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/interpreter"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
)

// ToValueDeclarations

func ToValueDeclarations(functions []StandardLibraryFunction) []sema.ValueDeclaration {
	valueDeclarations := make([]sema.ValueDeclaration, len(functions))
	for i, function := range functions {
		valueDeclarations[i] = function
	}
	return valueDeclarations
}

// ToValues

func ToValues(functions []StandardLibraryFunction) map[string]interpreter.Value {
	values := map[string]interpreter.Value{}
	for _, function := range functions {
		values[function.Name] = function.Function
	}
	return values
}
