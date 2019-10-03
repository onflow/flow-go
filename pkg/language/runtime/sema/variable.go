package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/raviqqe/hamt"
	"unsafe"
)

type Variable struct {
	Kind common.DeclarationKind
	// Type is the type of the variable
	Type Type
	// IsConstant indicates if the variable is read-only
	IsConstant bool
	// Depth is the depth of scopes in which the variable was declared
	Depth int
	// ArgumentLabels are the argument labels that must be used in an invocation of the variable
	ArgumentLabels []string
	// Pos is the position where the variable was declared
	Pos *ast.Position
}

// VariableEntry allows using variable pointers as entries in `hamt` structures
//
type VariableEntry struct {
	variable *Variable
}

func (k VariableEntry) Hash() uint32 {
	return uint32(uintptr(unsafe.Pointer(k.variable)))
}

func (k VariableEntry) Equal(e hamt.Entry) bool {
	other, ok := e.(VariableEntry)
	if !ok {
		return false
	}

	return other == k
}
