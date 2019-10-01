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

// VariableKey allows using variable pointers as keys in `hamt` structures
//
type VariableKey struct {
	variable *Variable
}

func (k VariableKey) Hash() uint32 {
	return uint32(uintptr(unsafe.Pointer(k.variable)))
}

func (k VariableKey) Equal(e hamt.Entry) bool {
	other, ok := e.(VariableKey)
	if !ok {
		return false
	}

	return other == k
}
