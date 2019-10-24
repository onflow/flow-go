package sema

import (
	"github.com/raviqqe/hamt"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/activations"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
)

type TypeActivations struct {
	activations *activations.Activations
}

func NewTypeActivations(baseTypes map[string]Type) *TypeActivations {
	var activation hamt.Map
	for name, baseType := range baseTypes {
		key := common.StringEntry(name)
		activation = activation.Insert(key, baseType)
	}

	typeActivations := &activations.Activations{}
	typeActivations.Push(activation)

	return &TypeActivations{
		activations: typeActivations,
	}
}

func (a *TypeActivations) Set(name string, ty Type) {
	a.activations.Set(name, ty)
}

func (a *TypeActivations) Find(name string) Type {
	value := a.activations.Find(name)
	if value == nil {
		return nil
	}
	ty, ok := value.(Type)
	if !ok {
		return nil
	}
	return ty
}

func (a *TypeActivations) Declare(
	identifier ast.Identifier,
	newType Type,
) (err error) {
	name := identifier.Identifier

	existingType := a.Find(name)
	if existingType != nil {
		err = &RedeclarationError{
			Kind: common.DeclarationKindType,
			Name: name,
			Pos:  identifier.Pos,
			// TODO: previous pos
		}
	}

	// type with this identifier is not declared in current scope, declare it
	a.Set(name, newType)
	return err
}
