package interpreter

import "github.com/dapperlabs/flow-go/pkg/language/runtime/ast"

type Location struct {
	ImportLocation ast.ImportLocation
	Position       ast.Position
}

type LocationRange struct {
	ImportLocation ast.ImportLocation
	ast.Range
}
