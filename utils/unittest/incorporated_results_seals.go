package unittest

import "github.com/onflow/flow-go/model/flow"

type IncorporatedResultSealFactory struct{}

func (f *IncorporatedResultSealFactory) Fixture(opts ...func(*flow.IncorporatedResultSeal)) *flow.IncorporatedResultSeal {
	result := ExecutionResultFixture()
	header := BlockHeaderFixture()
	incorporatedBlockID := header.ID()

	ir := IncorporatedResult.Fixture(
		IncorporatedResult.WithResult(result),
		IncorporatedResult.WithIncorporatedBlockID(incorporatedBlockID),
	)
	seal := Seal.Fixture(Seal.WithResult(result))

	irSeal := &flow.IncorporatedResultSeal{
		IncorporatedResult: ir,
		Seal:               seal,
		Header:             &header,
	}

	for _, apply := range opts {
		apply(irSeal)
	}
	return irSeal
}

func (f *IncorporatedResultSealFactory) Fixtures(n int) []*flow.IncorporatedResultSeal {
	seals := make([]*flow.IncorporatedResultSeal, 0, n)
	for i := 0; i < n; i++ {
		seals = append(seals, f.Fixture())
	}
	return seals
}

func (f *IncorporatedResultSealFactory) WithResult(result *flow.ExecutionResult) func(*flow.IncorporatedResultSeal) {
	return func(irSeal *flow.IncorporatedResultSeal) {
		IncorporatedResult.WithResult(result)(irSeal.IncorporatedResult)
		Seal.WithResult(result)(irSeal.Seal)
	}
}

func (f *IncorporatedResultSealFactory) WithIncorporatedBlockID(id flow.Identifier) func(*flow.IncorporatedResultSeal) {
	return func(irSeal *flow.IncorporatedResultSeal) {
		IncorporatedResult.WithIncorporatedBlockID(id)(irSeal.IncorporatedResult)
	}
}
