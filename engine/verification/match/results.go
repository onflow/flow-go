package match

import "github.com/dapperlabs/flow-go/model/flow"

type Result struct {
	ExecutorID      flow.Identifier
	ExecutionResult *flow.ExecutionResult
}

type Results interface {
	Add(result *Result) bool
	Has(resultID flow.Identifier) bool
	Rem(resultID flow.Identifier) bool
	ByID(resultID flow.Identifier) (*Result, bool)
}
