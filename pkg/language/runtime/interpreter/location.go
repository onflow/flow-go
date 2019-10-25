package interpreter

import "github.com/dapperlabs/flow-go/pkg/language/runtime/ast"

// LocationPosition defines a position in the source of the import tree.
// The Location defines the script within the import tree, the Position
// defines the row/colum within the source of that script.
type LocationPosition struct {
	Location ast.Location
	Position ast.Position
}

// LocationRange defines a range in the source of the import tree.
// The Position defines the script within the import tree, the Range
// defines the start/end position within the source of that script.
type LocationRange struct {
	Location ast.Location
	ast.Range
}
