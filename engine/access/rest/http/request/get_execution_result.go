package request

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/model/flow"
)

const idQuery = "id"

type GetExecutionResultByBlockIDs struct {
	BlockIDs []flow.Identifier
}

// GetExecutionResultByBlockIDsRequest extracts necessary variables from the provided request,
// builds a GetExecutionResultByBlockIDs instance, and validates it.
//
// No errors are expected during normal operation.
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
	var ids parser.IDs
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

// GetExecutionResultRequest extracts necessary variables from the provided request,
// builds a GetExecutionResult instance, and validates it.
//
// No errors are expected during normal operation.
func GetExecutionResultRequest(r *common.Request) (GetExecutionResult, error) {
	var req GetExecutionResult
	err := req.Build(r)
	return req, err
}
