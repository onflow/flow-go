package request

import (
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/model/flow"
)

const expandsTransactions = "transactions"

type GetCollection struct {
	ID                  flow.Identifier
	ExpandsTransactions bool
}

func (g *GetCollection) Build(r *rest.Request) error {
	err := g.Parse(
		r.GetVar(idQuery),
	)
	if err != nil {
		return rest.NewBadRequestError(err)
	}

	g.ExpandsTransactions = r.Expands(expandsTransactions)

	return nil
}

func (g *GetCollection) Parse(rawID string) error {
	var id ID
	err := id.Parse(rawID)
	if err != nil {
		return err
	}

	g.ID = id.Flow()
	return nil
}
