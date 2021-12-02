package rest

import (
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

const blockIDQueryParam = "block_id"

// getExecutionResultByID gets Execution Result payload by ID
func getExecutionResultsByBlockIDs(req *requestDecorator, backend access.API, link LinkGenerator) (interface{}, error) {
	queryID := req.getQueryParam(blockIDQueryParam)
	if len(queryID) == 0 {
		return nil, NewBadRequestError(fmt.Errorf("no blocks IDs specified"))
	}

	blockIDs, err := toIDs(queryID)
	if err != nil {
		return nil, err
	}

	results := make([]*generated.ExecutionResult, len(blockIDs))
	for i, id := range blockIDs {
		res, err := backend.GetExecutionResultForBlockID(req.Context(), id)
		if err != nil {
			return nil, err
		}
		results[i] = executionResultResponse(res, link)
	}

	return results, nil
}

func getExecutionResultByID(req *requestDecorator, backend access.API, link LinkGenerator) (interface{}, error) {
	return nil, nil
}
