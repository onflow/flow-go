package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/raviqqe/hamt"
	"github.com/segmentio/fasthash/fnv1"
)

type ResourceInvalidation struct {
	Kind ResourceInvalidationKind
	Pos  ast.Position
}

// ResourceInvalidationEntry allows using resource invalidations as entries in `hamt` structures
//
type ResourceInvalidationEntry struct {
	ResourceInvalidation
}

func (e ResourceInvalidationEntry) Hash() (result uint32) {
	result = fnv1.Init32
	result = fnv1.AddUint32(result, uint32(e.ResourceInvalidation.Kind))
	result = fnv1.AddUint32(result, e.ResourceInvalidation.Pos.Hash())
	return
}

func (e ResourceInvalidationEntry) Equal(e2 hamt.Entry) bool {
	other := e2.(ResourceInvalidationEntry)
	return e == other
}
