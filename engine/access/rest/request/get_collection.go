package request

import (
	"github.com/onflow/flow-go/engine/access/rest"
)

const expandsTransactions = "transactions"

type GetCollection struct {
	GetByIDRequest
	ExpandsTransactions bool
}

func (g *GetCollection) Build(r *rest.Request) error {
	err := g.GetByIDRequest.Build(r)
	g.ExpandsTransactions = r.Expands(expandsTransactions)

	return err
}
