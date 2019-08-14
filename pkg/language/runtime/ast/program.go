package ast

type Program struct {
	// all declarations, in the order they are defined
	Declarations          []Declaration
	structureDeclarations []*StructureDeclaration
	functionDeclarations  []*FunctionDeclaration
}

func (p *Program) Accept(visitor Visitor) Repr {
	return visitor.VisitProgram(p)
}

func (p *Program) StructureDeclarations() []*StructureDeclaration {
	if p.structureDeclarations == nil {
		p.structureDeclarations = make([]*StructureDeclaration, 0)
		for _, declaration := range p.Declarations {
			if structureDeclaration, ok := declaration.(*StructureDeclaration); ok {
				p.structureDeclarations = append(p.structureDeclarations, structureDeclaration)
			}
		}
	}
	return p.structureDeclarations
}

func (p *Program) FunctionDeclarations() []*FunctionDeclaration {
	if p.functionDeclarations == nil {
		p.functionDeclarations = make([]*FunctionDeclaration, 0)
		for _, declaration := range p.Declarations {
			if functionDeclaration, ok := declaration.(*FunctionDeclaration); ok {
				p.functionDeclarations = append(p.functionDeclarations, functionDeclaration)
			}
		}
	}
	return p.functionDeclarations
}
