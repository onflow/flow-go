package ast

// Members

type Members struct {
	Fields                []*FieldDeclaration
	fieldsByIdentifier    map[string]*FieldDeclaration
	Initializers          []*InitializerDeclaration
	Functions             []*FunctionDeclaration
	functionsByIdentifier map[string]*FunctionDeclaration
	CompositeDeclarations []*CompositeDeclaration
}

func (m *Members) FieldsByIdentifier() map[string]*FieldDeclaration {
	if m.fieldsByIdentifier == nil {
		fieldsByIdentifier := make(map[string]*FieldDeclaration, len(m.Fields))
		for _, field := range m.Fields {
			fieldsByIdentifier[field.Identifier.Identifier] = field
		}
		m.fieldsByIdentifier = fieldsByIdentifier
	}
	return m.fieldsByIdentifier
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
