package self_field_analyzer

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
)

// UninitializedFieldAccessError

type UninitializedFieldAccessError struct {
	Identifier ast.Identifier
	Pos        ast.Position
}

func (e *UninitializedFieldAccessError) Error() string {
	return fmt.Sprintf("cannot access unassigned field %s", e.Identifier.String())
}

func (*UninitializedFieldAccessError) isSemanticError() {}

func (e *UninitializedFieldAccessError) StartPosition() ast.Position {
	return e.Pos
}

func (e *UninitializedFieldAccessError) EndPosition() ast.Position {
	length := len(e.Identifier.Identifier)
	return e.Pos.Shifted(length - 1)
}
