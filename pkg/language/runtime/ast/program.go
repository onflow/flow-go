package ast

import "fmt"

type Program struct {
	// all declarations, in the order they are defined
	Declarations          []Declaration
	interfaceDeclarations []*InterfaceDeclaration
	compositeDeclarations []*CompositeDeclaration
	functionDeclarations  []*FunctionDeclaration
	eventDeclarations     []*EventDeclaration
	importedPrograms      map[LocationID]*Program
	importLocations       []Location
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

// ImportedPrograms returns the sub-programs imported by this program, indexed by location ID.
func (p *Program) ImportedPrograms() map[LocationID]*Program {
	if p.importedPrograms == nil {
		p.importedPrograms = make(map[LocationID]*Program)
	}

	return p.importedPrograms
}

// ImportLocations returns the import locations declared by this program.
func (p *Program) ImportLocations() []Location {
	if p.importLocations == nil {
		p.importLocations = make([]Location, 0)

		for _, declaration := range p.Declarations {
			if importDeclaration, ok := declaration.(*ImportDeclaration); ok {
				p.importLocations = append(p.importLocations, importDeclaration.Location)
			}
		}
	}

	return p.importLocations
}

type ImportResolver func(location Location) (*Program, error)

func (p *Program) ResolveImports(resolver ImportResolver) error {
	return p.resolveImports(
		resolver,
		map[LocationID]bool{},
		map[LocationID]*Program{},
	)
}

type CyclicImportsError struct {
	Location Location
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

	for _, location := range locations {

		imported, ok := resolved[location.ID()]
		if !ok {
			var err error
			imported, err = resolver(location)
			if err != nil {
				return err
			}
			if imported != nil {
				resolved[location.ID()] = imported
			}
		}

		if imported == nil {
			continue
		}

		p.setImportedProgram(location.ID(), imported)

		if resolving[location.ID()] {
			return CyclicImportsError{Location: location}
		}

		resolving[location.ID()] = true

		err := imported.resolveImports(resolver, resolving, resolved)
		if err != nil {
			return err
		}

		delete(resolving, location.ID())
	}

	return nil
}

// setImportedProgram adds an imported program to the set of imports, indexed by location ID.
func (p *Program) setImportedProgram(locationID LocationID, program *Program) {
	if p.importedPrograms == nil {
		p.importedPrograms = make(map[LocationID]*Program)
	}

	p.importedPrograms[locationID] = program
}
