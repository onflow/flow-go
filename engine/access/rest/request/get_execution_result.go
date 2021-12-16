package request

import (
	"fmt"
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/model/flow"
)

const idQuery = "id"
const noBlockIdsErr = "no block IDs provided"

type GetExecutionResultByBlockIDsRequest struct {
	BlockIDs []flow.Identifier
}

func (g *GetExecutionResultByBlockIDsRequest) Build(r *rest.Request) error {
	err := g.Parse(
		r.GetQueryParams(blockIDQuery),
	)
	if err != nil {
		return rest.NewBadRequestError(err)
	}

	return nil
}

func (g *GetExecutionResultByBlockIDsRequest) Parse(rawIDs []string) error {
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

type GetExecutionResultRequest struct {
	ID flow.Identifier
}

func (g *GetExecutionResultRequest) Build(r *rest.Request) error {
	err := g.Parse(
		r.GetQueryParam(idQuery),
	)
	if err != nil {
		return rest.NewBadRequestError(err)
	}

	return nil
}

func (g *GetExecutionResultRequest) Parse(rawID string) error {
	var id ID
	err := id.Parse(rawID)
	if err != nil {
		return err
	}

	return nil
}
