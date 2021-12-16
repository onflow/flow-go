package models

import (
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/model/flow"
)

type GetExecutionResult struct {
	BlockIDs []flow.Identifier
}

func (g *GetExecutionResult) Build(r *rest.Request) error {
	err := g.Parse(
		r.GetQueryParams(blockIDQuery),
	)
	if err != nil {
		return rest.NewBadRequestError(err)
	}

	return nil
}

func (g *GetExecutionResult) Parse(rawIDs []string) error {
	var ids IDs
	err := ids.Parse(rawIDs)
	if err != nil {
		return err
	}
	g.BlockIDs = ids.Flow()

	return nil
}
