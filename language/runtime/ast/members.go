package ast

import "github.com/dapperlabs/flow-go/language/runtime/common"

// Members

type Members struct {
	Fields []*FieldDeclaration
	// Use `FieldsByIdentifier()` instead
	_fieldsByIdentifier map[string]*FieldDeclaration
	// All special functions, such as initializers and destructors.
	// Use `Initializers()` and `Destructors()` to get subsets
	SpecialFunctions []*SpecialFunctionDeclaration
	// Use `Initializers()` instead
	_initializers []*SpecialFunctionDeclaration
	// Semantically only one destructor is allowed,
	// but the program might illegally declare multiple.
	// Use `Destructors()` instead
	_destructors []*SpecialFunctionDeclaration
	Functions    []*FunctionDeclaration
	// Use `FunctionsByIdentifier()` instead
	_functionsByIdentifier map[string]*FunctionDeclaration
	CompositeDeclarations  []*CompositeDeclaration
}

func (m *Members) FieldsByIdentifier() map[string]*FieldDeclaration {
	if m._fieldsByIdentifier == nil {
		fieldsByIdentifier := make(map[string]*FieldDeclaration, len(m.Fields))
		for _, field := range m.Fields {
			fieldsByIdentifier[field.Identifier.Identifier] = field
		}
		m._fieldsByIdentifier = fieldsByIdentifier
	}
	return m._fieldsByIdentifier
}

func (m *Members) FunctionsByIdentifier() map[string]*FunctionDeclaration {
	if m._functionsByIdentifier == nil {
		functionsByIdentifier := make(map[string]*FunctionDeclaration, len(m.Functions))
		for _, function := range m.Functions {
			functionsByIdentifier[function.Identifier.Identifier] = function
		}
		m._functionsByIdentifier = functionsByIdentifier
	}
	return m._functionsByIdentifier
}

func (m *Members) Initializers() []*SpecialFunctionDeclaration {
	if m._initializers == nil {
		initializers := []*SpecialFunctionDeclaration{}
		for _, function := range m.SpecialFunctions {
			if function.DeclarationKind != common.DeclarationKindInitializer {
				continue
			}
			initializers = append(initializers, function)
		}
		m._initializers = initializers
	}
	return m._initializers
}

func (m *Members) Destructors() []*SpecialFunctionDeclaration {
	if m._destructors == nil {
		destructors := []*SpecialFunctionDeclaration{}
		for _, function := range m.SpecialFunctions {
			if function.DeclarationKind != common.DeclarationKindDestructor {
				continue
			}
			destructors = append(destructors, function)
		}
		m._destructors = destructors
	}
	return m._destructors
}

// Destructor returns the first destructor, if any
func (m *Members) Destructor() *SpecialFunctionDeclaration {
	destructors := m.Destructors()
	if len(destructors) == 0 {
		return nil
	}
	return destructors[0]
}
