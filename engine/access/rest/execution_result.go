package rest

import (
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

const blockIDQueryParam = "block_id"

// getExecutionResultByID gets Execution Result payload by block IDs.
func getExecutionResultsByBlockIDs(req *Request, backend access.API, link LinkGenerator) (interface{}, error) {
	queryID := req.GetQueryParam(blockIDQueryParam)
	if len(queryID) == 0 {
		return nil, NewBadRequestError(fmt.Errorf("no blocks IDs specified"))
	}

	blockIDs, err := toIDs(queryID)
	if err != nil {
		return nil, err
	}

	// for each block ID we retrieve execution result
	results := make([]*generated.ExecutionResult, len(blockIDs))
	for i, id := range blockIDs {
		res, err := backend.GetExecutionResultForBlockID(req.Context(), id)
		if err != nil {
			return nil, err
		}
		results[i], err = executionResultResponse(res, link)
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

// getExecutionResultByID gets execution result by the ID.
func getExecutionResultByID(req *Request, backend access.API, link LinkGenerator) (interface{}, error) {
	id, err := req.id()
	if err != nil {
		return nil, err
	}

	res, err := backend.GetExecutionResultByID(req.Context(), id)
	if err != nil {
		return nil, err
	}

	return executionResultResponse(res, link)
}
