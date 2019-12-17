package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

type RewrittenPostConditions struct {
	BeforeStatements        []ast.Statement
	RewrittenPostConditions ast.Conditions
}
