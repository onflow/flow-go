package ast

import "fmt"

type Program struct {
	// all declarations, in the order they are defined
	Declarations          []Declaration
	interfaceDeclarations []*InterfaceDeclaration
	compositeDeclarations []*CompositeDeclaration
	functionDeclarations  []*FunctionDeclaration
	eventDeclarations     []*EventDeclaration
	importLocations       map[LocationID]ImportLocation
	imports               map[LocationID]*Program
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

func (p *Program) CompositeDeclarations() []*CompositeDeclaration {
	if p.compositeDeclarations == nil {
		p.compositeDeclarations = make([]*CompositeDeclaration, 0)
		for _, declaration := range p.Declarations {
			if compositeDeclaration, ok := declaration.(*CompositeDeclaration); ok {
				p.compositeDeclarations = append(p.compositeDeclarations, compositeDeclaration)
			}
		}
	}
	return p.compositeDeclarations
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

func (p *Program) EventDeclarations() []*EventDeclaration {
	if p.eventDeclarations == nil {
		p.eventDeclarations = make([]*EventDeclaration, 0)
		for _, declaration := range p.Declarations {
			if eventDeclaration, ok := declaration.(*EventDeclaration); ok {
				p.eventDeclarations = append(p.eventDeclarations, eventDeclaration)
			}
		}
	}
	return p.eventDeclarations
}

func (p *Program) Imports() map[LocationID]*Program {
	if p.imports == nil {
		p.imports = make(map[LocationID]*Program)
	}

	return p.imports
}

func (p *Program) ImportLocations() map[LocationID]ImportLocation {
	if p.importLocations == nil {
		p.importLocations = make(map[LocationID]ImportLocation)

		for _, declaration := range p.Declarations {
			if importDeclaration, ok := declaration.(*ImportDeclaration); ok {
				p.importLocations[importDeclaration.Location.ID()] = importDeclaration.Location
			}
		}
	}

	return p.importLocations
}

type ImportResolver func(location ImportLocation) (*Program, error)

func (p *Program) ResolveImports(resolver ImportResolver) error {
	return p.resolveImports(
		resolver,
		map[LocationID]bool{},
		map[LocationID]*Program{},
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
	resolving map[LocationID]bool,
	resolved map[LocationID]*Program,
) error {
	locations := p.ImportLocations()

	for locationID, location := range locations {

		imported, ok := resolved[locationID]
		if !ok {
			var err error
			imported, err = resolver(location)
			if err != nil {
				return err
			}
			if imported != nil {
				resolved[locationID] = imported
			}
		}

		if imported == nil {
			continue
		}

		p.setImportedProgram(locationID, imported)

		if resolving[locationID] {
			return CyclicImportsError{Location: location}
		}

		resolving[locationID] = true

		err := imported.resolveImports(resolver, resolving, resolved)
		if err != nil {
			return err
		}

		delete(resolving, locationID)
	}

	return nil
}

// setImportedProgram adds an imported program to the set of imports, indexed by location ID.
func (p *Program) setImportedProgram(locationID LocationID, program *Program) {
	if p.imports == nil {
		p.imports = make(map[LocationID]*Program)
	}

	p.imports[locationID] = program
}
