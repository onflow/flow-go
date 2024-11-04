package request

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/model/flow"
)

const idQuery = "id"

type GetExecutionResultByBlockIDs struct {
	BlockIDs []flow.Identifier
}

func GetExecutionResultByBlockIDsRequest(r *common.Request) (GetExecutionResultByBlockIDs, error) {
	var req GetExecutionResultByBlockIDs
	err := req.Build(r)
	return req, err
}

func (g *GetExecutionResultByBlockIDs) Build(r *common.Request) error {
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
		return fmt.Errorf("no block IDs provided")
	}

	return nil
}

type GetExecutionResult struct {
	GetByIDRequest
}

func GetExecutionResultRequest(r *common.Request) (GetExecutionResult, error) {
	var req GetExecutionResult
	err := req.Build(r)
	return req, err
}
