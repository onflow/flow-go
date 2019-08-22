package ast

import "fmt"

type Program struct {
	// all declarations, in the order they are defined
	Declarations          []Declaration
	interfaceDeclarations []*InterfaceDeclaration
	structureDeclarations []*StructureDeclaration
	functionDeclarations  []*FunctionDeclaration
	imports               map[ImportLocation]*Program
}

func (p *Program) Accept(visitor Visitor) Repr {
	return visitor.VisitProgram(p)
}

func (p *Program) InterfaceDeclarations() []*InterfaceDeclaration {
	if p.interfaceDeclarations == nil {
		p.interfaceDeclarations = make([]*InterfaceDeclaration, 0)
		for _, declaration := range p.Declarations {
			if interfaceDeclaration, ok := declaration.(*InterfaceDeclaration); ok {
				p.interfaceDeclarations = append(p.interfaceDeclarations, interfaceDeclaration)
			}
		}
	}
	return p.interfaceDeclarations
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

func (p *Program) Imports() map[ImportLocation]*Program {
	if p.imports == nil {
		p.imports = make(map[ImportLocation]*Program)
		for _, declaration := range p.Declarations {
			if importDeclaration, ok := declaration.(*ImportDeclaration); ok {
				p.imports[importDeclaration.Location] = nil
			}
		}
	}
	return p.imports
}

type ImportResolver func(location ImportLocation) (*Program, error)

func (p *Program) ResolveImports(resolver ImportResolver) error {
	return p.resolveImports(
		resolver,
		map[ImportLocation]bool{},
		map[ImportLocation]*Program{},
	)
}

type CyclicImportsError struct {
	Location ImportLocation
}

func (e CyclicImportsError) Error() string {
	return fmt.Sprintf("cyclic import of %s", e.Location)
}

func (p *Program) resolveImports(
	resolver ImportResolver,
	resolving map[ImportLocation]bool,
	resolved map[ImportLocation]*Program,
) error {

	imports := p.Imports()
	for location := range imports {
		imported, ok := resolved[location]
		if !ok {
			var err error
			imported, err = resolver(location)
			if err != nil {
				return err
			}
			if imported != nil {
				resolved[location] = imported
			}
		}
		if imported == nil {
			continue
		}
		imports[location] = imported
		if resolving[location] {
			return CyclicImportsError{Location: location}
		}
		resolving[location] = true
		err := imported.resolveImports(resolver, resolving, resolved)
		if err != nil {
			return err
		}
		delete(resolving, location)
	}
	return nil
}
