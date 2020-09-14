package stdmap

import "github.com/dapperlabs/flow-go/model/flow"

type IncorporatedResultSeals struct {
	*Backend
}

func NewIncorporatedResultSeals(limit uint, opts ...OptionFunc) *IncorporatedResultSeals {
	return &IncorporatedResultSeals{
		Backend: NewBackend(append(opts, WithLimit(limit))...),
	}
}

func (ir *IncorporatedResultSeals) Add(seal *flow.IncorporatedResultSeal) bool {
	return ir.Backend.Add(seal)
}

func (ir *IncorporatedResultSeals) All() []*flow.IncorporatedResultSeal {
	entities := ir.Backend.All()
	res := make([]*flow.IncorporatedResultSeal, 0, len(ir.entities))
	for _, entity := range entities {
		res = append(res, entity.(*flow.IncorporatedResultSeal))
	}
	return res
}

func (ir *IncorporatedResultSeals) ByID(id flow.Identifier) (*flow.IncorporatedResultSeal, bool) {
	entity, ok := ir.Backend.ByID(id)
	if !ok {
		return nil, false
	}
	res, ok := entity.(*flow.IncorporatedResultSeal)
	if !ok {
		return nil, false
	}
	return res, true
}

func (ir *IncorporatedResultSeals) Rem(incorporatedResultID flow.Identifier) bool {
	return ir.Backend.Rem(incorporatedResultID)
}
