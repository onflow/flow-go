package unittest

import "github.com/onflow/flow-go/model/flow"

var IncorporatedResult incorporatedResultFactory

type incorporatedResultFactory struct{}

func (f *incorporatedResultFactory) Fixture(opts ...func(*flow.IncorporatedResult)) *flow.IncorporatedResult {
	ir, _ := flow.NewIncorporatedResult(flow.UntrustedIncorporatedResult{
		IncorporatedBlockID: IdentifierFixture(),
		Result:              ExecutionResultFixture()})

	for _, apply := range opts {
		apply(ir)
	}
	return ir
}

func (f *incorporatedResultFactory) WithResult(result *flow.ExecutionResult) func(*flow.IncorporatedResult) {
	return func(incResult *flow.IncorporatedResult) {
		incResult.Result = result
	}
}

func (f *incorporatedResultFactory) WithIncorporatedBlockID(id flow.Identifier) func(*flow.IncorporatedResult) {
	return func(incResult *flow.IncorporatedResult) {
		incResult.IncorporatedBlockID = id
	}
}
