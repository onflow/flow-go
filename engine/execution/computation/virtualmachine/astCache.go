package virtualmachine

import (
	"github.com/onflow/cadence/runtime/ast"
)

type ASTCache interface {
	GetProgram(ast.Location) (*ast.Program, error)
	SetProgram(ast.Location, *ast.Program) error
}

func (vm *virtualMachine) GetProgram(location ast.Location) (*ast.Program, error) {
	program, found := vm.cache.Get(location.ID())
	if found {
		return program.(*ast.Program), nil
	}
	return nil, nil
}

func (vm *virtualMachine) SetProgram(location ast.Location, program *ast.Program) error {
	_ = vm.cache.Add(location.ID(), program)
	return nil
}
