package interpreter

import "github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"

type Location struct {
	ImportLocation ast.ImportLocation
	Position       ast.Position
}

type LocationRange struct {
	ImportLocation ast.ImportLocation
	StartPos       ast.Position
	EndPos         ast.Position
}
