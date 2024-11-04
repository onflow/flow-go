package request

import (
	"github.com/onflow/flow-go/engine/access/rest/common"
)

const ExpandsTransactions = "transactions"

type GetCollection struct {
	GetByIDRequest
	ExpandsTransactions bool
}

func (g *GetCollection) Build(r *common.Request) error {
	err := g.GetByIDRequest.Build(r)
	g.ExpandsTransactions = r.Expands(ExpandsTransactions)

	return err
}
