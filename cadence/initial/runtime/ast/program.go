/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ast

import "fmt"

type Program struct {
	// all declarations, in the order they are defined
	Declarations            []Declaration
	importDeclarations      []*ImportDeclaration
	interfaceDeclarations   []*InterfaceDeclaration
	compositeDeclarations   []*CompositeDeclaration
	functionDeclarations    []*FunctionDeclaration
	transactionDeclarations []*TransactionDeclaration
	importedPrograms        map[LocationID]*Program
	importLocations         []Location
}

func (p *Program) StartPosition() Position {
	if len(p.Declarations) == 0 {
		return Position{}
	}
	firstDeclaration := p.Declarations[0]
	return firstDeclaration.StartPosition()
}

func (p *Program) EndPosition() Position {
	count := len(p.Declarations)
	if count == 0 {
		return Position{}
	}
	lastDeclaration := p.Declarations[count-1]
	return lastDeclaration.EndPosition()
}

func (p *Program) Accept(visitor Visitor) Repr {
	return visitor.VisitProgram(p)
}

func (p *Program) ImportDeclarations() []*ImportDeclaration {
	if p.importDeclarations == nil {
		p.importDeclarations = make([]*ImportDeclaration, 0)
		for _, declaration := range p.Declarations {
			if importDeclaration, ok := declaration.(*ImportDeclaration); ok {
				p.importDeclarations = append(p.importDeclarations, importDeclaration)
			}
		}
	}
	return p.importDeclarations
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

func (p *Program) TransactionDeclarations() []*TransactionDeclaration {
	if p.transactionDeclarations == nil {
		p.transactionDeclarations = make([]*TransactionDeclaration, 0)
		for _, declaration := range p.Declarations {
			if transactionDeclaration, ok := declaration.(*TransactionDeclaration); ok {
				p.transactionDeclarations = append(p.transactionDeclarations, transactionDeclaration)
			}
		}
	}
	return p.transactionDeclarations
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
	return fmt.Sprintf("cyclic import of `%s`", e.Location)
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
