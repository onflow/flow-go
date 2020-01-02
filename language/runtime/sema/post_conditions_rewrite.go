package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

type PostConditionsRewrite struct {
	BeforeStatements        []ast.Statement
	RewrittenPostConditions ast.Conditions
}
