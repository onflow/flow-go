package request

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
)

const idQuery = "id"
const noBlockIdsErr = "no block IDs provided"

type GetExecutionResultByBlockIDs struct {
	BlockIDs []flow.Identifier
}

func (g *GetExecutionResultByBlockIDs) Build(r *Request) error {
	return g.Parse(
		r.GetQueryParams(blockIDQuery),
	)
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
	GetByIDRequest
}
