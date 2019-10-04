package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/activations"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/raviqqe/hamt"
)

type ValueActivations struct {
	activations *activations.Activations
}

func NewValueActivations() *ValueActivations {
	valueActivations := &activations.Activations{}
	valueActivations.Push(hamt.NewMap())
	return &ValueActivations{
		activations: valueActivations,
	}
}

func (a *ValueActivations) Enter() {
	a.activations.PushCurrent()
}

func (a *ValueActivations) Leave() {
	a.activations.Pop()
}

func (a *ValueActivations) Set(name string, variable *Variable) {
	a.activations.Set(name, variable)
}

func (a *ValueActivations) Find(name string) *Variable {
	value := a.activations.Find(name)
	if value == nil {
		return nil
	}
	variable, ok := value.(*Variable)
	if !ok {
		return nil
	}
	return variable
}

func (a *ValueActivations) Depth() int {
	return a.activations.Depth()
}

func (a *ValueActivations) Declare(
	identifier string,
	ty Type,
	kind common.DeclarationKind,
	pos ast.Position,
	isConstant bool,
	argumentLabels []string,
) (variable *Variable, err error) {

	depth := a.activations.Depth()

	// check if variable with this name is already declared in the current scope
	existingVariable := a.Find(identifier)
	if existingVariable != nil && existingVariable.Depth == depth {
		err = &RedeclarationError{
			Kind:        kind,
			Name:        identifier,
			Pos:         pos,
			PreviousPos: existingVariable.Pos,
		}
	}

	// variable with this name is not declared in current scope, declare it
	variable = &Variable{
		Identifier:     identifier,
		Kind:           kind,
		IsConstant:     isConstant,
		Depth:          depth,
		Type:           ty,
		Pos:            &pos,
		ArgumentLabels: argumentLabels,
	}
	a.activations.Set(identifier, variable)
	return variable, err
}

func (a *ValueActivations) DeclareFunction(
	identifier ast.Identifier,
	invokableType InvokableType,
	argumentLabels []string,
) (*Variable, error) {
	return a.Declare(
		identifier.Identifier,
		invokableType,
		common.DeclarationKindFunction,
		identifier.Pos,
		true,
		argumentLabels,
	)
}

func (a *ValueActivations) DeclareImplicitConstant(identifier string, ty Type, kind common.DeclarationKind) (*Variable, error) {
	return a.Declare(
		identifier,
		ty,
		kind,
		ast.Position{},
		true,
		nil,
	)
}

func (a *ValueActivations) VariablesDeclaredInThisScope() map[string]*Variable {
	variables := map[string]*Variable{}

	depth := a.activations.Depth()
	values := a.activations.CurrentOrNew()

	var entry hamt.Entry
	var value interface{}

	for {
		entry, value, values = values.FirstRest()
		if entry == nil {
			break
		}

		variable := value.(*Variable)

		if variable.Depth != depth {
			continue
		}

		name := string(entry.(common.StringEntry))

		variables[name] = variable
	}

	return variables
}
