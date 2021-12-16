package request

import (
	"github.com/onflow/flow-go/engine/access/rest"
)

const resultExpandable = "result"

type GetTransaction struct {
	GetByIDRequest
	ExpandsResult bool
}

func (g *GetTransaction) Build(r *rest.Request) error {
	err := g.GetByIDRequest.Build(r)
	g.ExpandsResult = r.Expands(resultExpandable)

	return err
}

type GetTransactionResult struct {
	GetByIDRequest
}
