package utils

import "github.com/onflow/cadence/runtime/ast"

type NoopCache struct{}

func (n NoopCache) GetProgram(ast.Location) (*ast.Program, error) {
	return nil, nil
}
func (n NoopCache) SetProgram(ast.Location, *ast.Program) error {
	return nil
}
