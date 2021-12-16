package request

import (
	"fmt"
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/model/flow"
)

const idQuery = "id"
const noBlockIdsErr = "no block IDs provided"

type GetExecutionResultByBlockIDs struct {
	BlockIDs []flow.Identifier
}

func (g *GetExecutionResultByBlockIDs) Build(r *rest.Request) error {
	err := g.Parse(
		r.GetQueryParams(blockIDQuery),
	)
	if err != nil {
		return rest.NewBadRequestError(err)
	}

	return nil
}

func (g *GetExecutionResultByBlockIDs) Parse(rawIDs []string) error {
	var ids IDs
	err := ids.Parse(rawIDs)
	if err != nil {
		return err
	}
	g.BlockIDs = ids.Flow()

	if len(g.BlockIDs) == 0 {
		return fmt.Errorf(noBlockIdsErr)
	}

	return nil
}

type GetExecutionResult struct {
	ID flow.Identifier
}

func (g *GetExecutionResult) Build(r *rest.Request) error {
	err := g.Parse(
		r.GetQueryParam(idQuery),
	)
	if err != nil {
		return rest.NewBadRequestError(err)
	}

	return nil
}

func (g *GetExecutionResult) Parse(rawID string) error {
	var id ID
	err := id.Parse(rawID)
	if err != nil {
		return err
	}

	return nil
}
