package ast

// Members

type Members struct {
	Fields                []*FieldDeclaration
	Initializers          []*InitializerDeclaration
	Functions             []*FunctionDeclaration
	functionsByIdentifier map[string]*FunctionDeclaration
	CompositeDeclarations []*CompositeDeclaration
}

func (m *Members) FunctionsByIdentifier() map[string]*FunctionDeclaration {
	if m.functionsByIdentifier == nil {
		functionsByIdentifier := make(map[string]*FunctionDeclaration, len(m.Functions))
		for _, function := range m.Functions {
			functionsByIdentifier[function.Identifier.Identifier] = function
		}
		m.functionsByIdentifier = functionsByIdentifier
	}
	return m.functionsByIdentifier
}
